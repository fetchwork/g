package function

import (
	"caf/model"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

func checkReasonExistByNumberID(db *sqlx.DB, num_id int64, sip_code string) (exists bool, err error) {
	// Выполняем запрос и заполняем attemptsCounter
	err = db.Get(&exists, "SELECT EXISTS (SELECT 1 FROM caf.num_reasons WHERE num_id = $1 AND sip_code = $2)", num_id, sip_code)
	if err != nil {
		return false, fmt.Errorf("failed to check number reasons: %w", err)
	}

	return exists, nil
}

func checkStatByNumber(number string, from_date int64, to_date int64) (statusCode int, response model.StatResponseAPI, err error) {
	// Создаем новый запрос
	url := fmt.Sprintf("%s/csc", config.BILLING_API.URL)

	// Формируем тело запроса обновления в Webitel с новым resource ID и правильным приоритетом
	requestBody := map[string]interface{}{
		"number":    number,
		"from_date": strconv.FormatInt(from_date, 10), // Преобразование int64 в строку
		"to_date":   strconv.FormatInt(to_date, 10),   // Преобразование int64 в строку
	}

	// Читаем тело ответа
	body, statusCode, err := APIFetch(config.BILLING_API.Header, config.BILLING_API.Key, "POST", url, requestBody)
	if err != nil {
		return statusCode, model.StatResponseAPI{}, fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	//OutLog.Println(string(body))

	// Парсим JSON-ответ
	err = json.Unmarshal([]byte(body), &response)
	if err != nil {
		return statusCode, model.StatResponseAPI{}, fmt.Errorf("failed to parse JSON response from API: %s", err.Error())

	}

	return statusCode, response, nil
}

func CompareNumberToStat(db *sqlx.DB, ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		start := time.Now()

		var Numbers []model.Numbers
		// Номер не заблокирован или номер заблокирован и имеет флаг повторной проверки
		err := db.Select(&Numbers, "SELECT id, number FROM caf.numbers WHERE blocked = $1 OR (blocked = $2 AND repeated_check = $3)", false, true, true)
		if err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("not blocked Numbers not found")
			}
			return fmt.Errorf("failed to get Numbers: %w", err)
		}

		// Загружаем временную зону из конфигурации
		location, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %s", err)
		}

		now := time.Now().In(location)

		// Получаем вчерашнее число
		yesterday := now.AddDate(0, 0, -1)

		// Создаем переменную start, устанавливая время на 00:00:00.000 в указанной временной зоне
		startTime := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
		startUnixTime := ConvertTimeToUnixMillis(&startTime)

		// Создаем переменную stop, устанавливая время на 23:59:59.999 в указанной временной зоне
		stopTime := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 999999999, location)
		stopUnixTime := ConvertTimeToUnixMillis(&stopTime)

		numberCount := 0
		// Перебираем номера
		for _, number := range Numbers {
			numberCount++
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				statusCode, response, err := checkStatByNumber(number.Number, startUnixTime, stopUnixTime)
				if err != nil {
					return fmt.Errorf("failed to fetch CDR API: %s", err)
				}
				OutLog.Printf("Check %d %s", numberCount, number.Number)
				if statusCode >= 200 && statusCode < 300 {
					if response.Data != nil {
						// Перебираем статистику по текущему номеру
						for _, stat := range *response.Data {
							// Если данные есть
							if stat.OK != nil && stat.Exists != nil {
								// Проверяем есть ли уже данные по причине отбоя по текущему номеру
								sip_code_exists, err := checkReasonExistByNumberID(db, number.ID, *stat.SipCode)
								if err != nil {
									return fmt.Errorf("failed to check reason by number: %s", err)
								}
								// Если код отбоя по текущему номеру уже есть в БД
								if sip_code_exists {
									// Обновляем счётчик кода отбоя увеличивая на кол-во по каждому отбою за период
									_, err := db.Exec("UPDATE caf.num_reasons SET count = count + $1 WHERE num_id = $2 AND sip_code = $3", stat.Count, number.ID, stat.SipCode)
									if err != nil {
										return fmt.Errorf("failed to update reason: %s", err)
									}
								} else {
									// Если по текущему номеру ещё нет данных в БД по отбою, то добавляем новой строкой
									_, err := db.Exec("INSERT INTO caf.num_reasons (num_id, count, sip_code, sip_reason) VALUES ($1, $2, $3, $4)", number.ID, stat.Count, stat.SipCode, stat.SipReason)
									if err != nil {
										return fmt.Errorf("failed to insert reason: %s", err)
									}
								}
								// Если в массиве кодов отбоя по текущему номеру есть успешный то обновляем информацию по номеру
								if *stat.OK {
									_, err := db.Exec("UPDATE caf.numbers SET stat_waiting = $1, success = $2 WHERE id = $3", false, true, number.ID)
									if err != nil {
										return fmt.Errorf("failed to update number: %s", err)
									}
									// Если в массиве кодов отбоя по текущему номеру есть не успешный, но статистика по номеру есть то обновляем информацию
								} else if !*stat.OK && *stat.Exists {
									_, err := db.Exec("UPDATE caf.numbers SET stat_waiting = $1 WHERE id = $2", false, number.ID)
									if err != nil {
										return fmt.Errorf("failed to update number: %s", err)
									}
								}
								// Если оба поля stat.OK && stat.Exists == false, ничего не обновляем по номеру
							}
						}
					}
				}
			}
		}

		duration := time.Since(start)
		OutLog.Printf("All stat checked in %s", duration)
		return nil
	}
}
