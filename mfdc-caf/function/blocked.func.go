package function

import (
	"caf/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jmoiron/sqlx"
)

// Блокировка номера внутри сервиса
func BlockNumberActions(db *sqlx.DB, blocked bool, blockedAt *time.Time, blockToDate *time.Time, number string, clientID *string, teamID *int, description *string) error {

	if blocked {
		// Отправляем номер в ЧС
		_, err := db.Exec("INSERT INTO caf.blacklist (number, created_at, team_id, description) VALUES ($1, $2, $3, $4)", number, blockedAt, teamID, description)
		if err != nil {
			return fmt.Errorf("failed to insert number: %w", err)
		}

		_, err = db.Exec("UPDATE caf.numbers SET blocked = $1, blocked_at = $2, stop_expirid = $3, stat_waiting = $4 WHERE number = $5", blocked, blockedAt, blockToDate, true, number)
		if err != nil {
			return fmt.Errorf("failed to update stop date for number: %w", err)
		}

		var descriptionLog string
		if description != nil {
			descriptionLog = *description
		} else {
			descriptionLog = "Добавлен в ЧС"
		}
		var TeamID int
		if teamID != nil {
			TeamID = *teamID
		} else {
			TeamID = 0
		}
		err = addLog(db, TeamID, number, descriptionLog, true)
		if err != nil {
			ErrLog.Printf("Failed save to log: %s", err)
		}
	} else {
		_, err := db.Exec("DELETE FROM caf.blacklist WHERE number = $1", number)
		if err != nil {
			return fmt.Errorf("failed to unblock number from blacklist: %w", err)
		}
		_, err = db.Exec("UPDATE caf.numbers SET blocked = $1 WHERE number = $2", blocked, number)
		if err != nil {
			return fmt.Errorf("failed to update stop date for number: %w", err)
		}

		// Удаляем номер из ЧС БП
		if clientID != nil {
			err = SendNumberToBP(true, number, clientID, false, false, false)
			if err != nil {
				return fmt.Errorf("failed to send number to BP: %s", err)
			}
		} else {
			err = SendNumberToBP(false, number, nil, false, false, false)
			if err != nil {
				return fmt.Errorf("failed to send number to BP: %s", err)
			}
		}

		var descriptionLog string
		if description != nil {
			descriptionLog = *description
		} else {
			descriptionLog = "Удалён из ЧС"
		}
		var TeamID int
		if teamID != nil {
			TeamID = *teamID
		} else {
			TeamID = 0
		}
		err = addLog(db, TeamID, number, descriptionLog, false)
		if err != nil {
			ErrLog.Printf("Failed save to log: %s", err)
		}
	}
	return nil
}

// Отправка номера в ЧС Webitel
func SendNumberToWebitel(number string, description string) error {
	if number == "" {
		return fmt.Errorf("number cannot be empty")
	}

	// Создаем новый запрос
	url := fmt.Sprintf("%s/call_center/list/%d/communication", config.API_Webitel.URL, config.API_Webitel.BlacklistID)

	// Формируем запрос
	jsonRequest := map[string]interface{}{
		"description": "CAF " + description,
		"number":      number,
	}

	// Читаем тело ответа
	_, statusCode, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "POST", url, jsonRequest)
	if err != nil {
		fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	if statusCode > 299 || statusCode < 200 {
		fmt.Errorf("failed add number to Webitel API: %s", err.Error())
	}

	return nil
}

// Отправка номера в ЧС БП
func SendNumberToBP(byClientID bool, number string, clientID *string, marketing bool, incoming bool, outgoing bool) error {
	if number == "" {
		return fmt.Errorf("number cannot be empty")
	}

	var method, byKey string

	if byClientID && clientID != nil {
		method = "black-list:communications:data-from-dialer-service:2"
		byKey = *clientID
	} else {
		method = "black-list:communications:data-from-dialer-service:1"
		byKey = number
	}

	JRPCParams := []map[string]interface{}{ // params это массив
		{
			"options": map[string]interface{}{
				"marketing": true,
				"incoming":  true,
				"outgoing":  true,
			},
			"phone": byKey,
		},
	}

	ID := strconv.FormatInt(time.Now().Unix(), 10) // В качестве ID текущее время в формате UnixTime преобразуем из int64 в string

	responseBody, err := JRPCFetch(config.BP_API.ServiceSecurityKey, method, config.BP_API.URL, ID, JRPCParams)
	if err != nil {
		return fmt.Errorf("failed to fetch JRPS BP: %w", err)
	}

	var JRPCResponse model.JRPSResponse

	// Парсим JSON-ответ
	err = json.Unmarshal([]byte(responseBody), &JRPCResponse)
	if err != nil {
		return fmt.Errorf("failed to parse JSON response from API: %s", err.Error())
	}

	// Проверяем ID ответа
	if JRPCResponse.ID != ID {
		return fmt.Errorf("invalid response ID from JRPC: expected %s, got %s", ID, JRPCResponse.ID)
	}

	if JRPCResponse.Error != nil {
		return fmt.Errorf("code: %d; message: %s; data: %v", *JRPCResponse.Error.Code, *JRPCResponse.Error.Message, *JRPCResponse.Error.Data.Description)
	}

	return nil
}

// Ежесуточная функция проверки номеров для блокирования по стратегии unsuccessful
func CheckNumberForBlockByUnsuccessful(db *sqlx.DB) error {
	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %s", err)
	}

	now := time.Now().In(location)

	// Получаем 30 дней назад
	minus30Days := now.AddDate(0, 0, -30)

	// Создаем переменную start, устанавливая время на 00:00:00.000 в указанной временной зоне
	startTime := time.Date(minus30Days.Year(), minus30Days.Month(), minus30Days.Day(), 0, 0, 0, 0, location)

	// Создаем переменную stop, устанавливая время на 23:59:59.999 в указанной временной зоне
	stopTime := time.Date(minus30Days.Year(), minus30Days.Month(), minus30Days.Day(), 23, 59, 59, 999999999, location)

	var Numbers []model.Numbers
	query := `SELECT n.id, n.number, n.client_id, n.success, n.stat_waiting, n.team_id FROM caf.numbers AS n
				LEFT JOIN caf.teams AS t ON n.team_id=t.id 
				WHERE t.strategy = $1
				AND t.filtration = $2
				AND n.first_load_at BETWEEN $3 AND $4
				AND n.blocked = $5`
	err = db.Select(&Numbers, query, "unsuccessful", true, startTime, stopTime, false)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("numbers not found")
		}
		return fmt.Errorf("failed to get numbers: %w", err)
	}

	// Перебираем выбранные номера
	for _, number := range Numbers {
		currentDate := time.Now().In(location)
		if number.Success { // Если у номера за 30 дней был успешный вызов, то удаляем его из БД
			_, err := db.Exec("DELETE FROM caf.numbers WHERE id = $1", number.ID)
			if err != nil {
				return fmt.Errorf("failed to delete number: %w", err)
			}
		} else if !number.Success && !number.StatWaiting { // Если у номера за 30 дней не было успешных вызовов, но были неуспешные то отмечаем как заблокированный
			// Помечаем номер как заблокированный
			description := "За 30 дней не было успешных"
			err := BlockNumberActions(db, true, &currentDate, nil, number.Number, number.ClientID, number.TeamID, &description)
			if err != nil {
				return fmt.Errorf("failed to block number actions: %w", err)
			}

			// Отправляем номер в ЧС БП
			if number.ClientID != nil {
				err = SendNumberToBP(true, number.Number, number.ClientID, true, true, true)
				if err != nil {
					return fmt.Errorf("failed to send number to BP: %s", err)
				}
			} else {
				err = SendNumberToBP(false, number.Number, nil, true, true, true)
				if err != nil {
					return fmt.Errorf("failed to send number to BP: %s", err)
				}
			}

			// Отправляем номер в ЧС Webitel
			err = SendNumberToWebitel(number.Number, description)
			if err != nil {
				return fmt.Errorf("failed to send number to Webitel: %s", err)
			}
		}
	}

	return nil
}

// Ежесуточная функция проверки заблокированных номеров если при изменений данных владельца номера была запрошена дополнительная проверка на сутки
func RecheckNumberForBlockByUnsuccessful(db *sqlx.DB) error {
	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %s", err)
	}

	now := time.Now().In(location)

	// TODO
	// Получаем минус период
	minusPeriod := now.AddDate(0, 0, -1)

	// Создаем переменную start, устанавливая время на 00:00:00.000 в указанной временной зоне
	startTime := time.Date(minusPeriod.Year(), minusPeriod.Month(), minusPeriod.Day(), 0, 0, 0, 0, location)

	// Создаем переменную stop, устанавливая время на 23:59:59.999 в указанной временной зоне
	stopTime := time.Date(minusPeriod.Year(), minusPeriod.Month(), minusPeriod.Day(), 23, 59, 59, 999999999, location)

	var Numbers []model.Numbers
	query := `SELECT n.id, n.number, n.success, n.stat_waiting, n.team_id FROM caf.numbers AS n
				LEFT JOIN caf.teams AS t ON n.team_id=t.id 
				WHERE n.repeated_check = $1
				AND n.first_load_at BETWEEN $2 AND $3
				AND n.blocked = $4
				AND n.stat_waiting = $5`
	err = db.Select(&Numbers, query, true, startTime, stopTime, true, false)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("numbers not found")
		}
		return fmt.Errorf("failed to get numbers: %w", err)
	}

	// Перебираем выбранные номера
	for _, number := range Numbers {
		if number.Success { // Если у номера был успешный вызов, то удаляем его из БД
			_, err := db.Exec("DELETE FROM caf.numbers WHERE id = $1", number.ID)
			if err != nil {
				return fmt.Errorf("failed to delete number: %w", err)
			}
		} else if !number.Success && !number.StatWaiting { // Если у номера не было успешных вызовов, но были неуспешные то отмечаем что повторная проверка состоялась и снимаем отметку что требуется повторная проверка ранее заблокированного номера
			_, err := db.Exec("UPDATE caf.numbers SET block_rechecked = $1, repeated_check = $2 WHERE id = $3", true, false, number.ID)
			if err != nil {
				return fmt.Errorf("failed to delete number: %w", err)
			}
		}
	}

	return nil
}

// Ежесуточная функция проверки номеров для блокирования по стратегии cause
func CheckNumberForBlockByCause(db *sqlx.DB) error {
	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %s", err)
	}

	now := time.Now().In(location)

	// TODO
	// Получаем минус период
	minusPeriod := now.AddDate(0, 0, -1)

	// Создаем переменную start, устанавливая время на 00:00:00.000 в указанной временной зоне
	startTime := time.Date(minusPeriod.Year(), minusPeriod.Month(), minusPeriod.Day(), 0, 0, 0, 0, location)

	// Создаем переменную stop, устанавливая время на 23:59:59.999 в указанной временной зоне
	stopTime := time.Date(minusPeriod.Year(), minusPeriod.Month(), minusPeriod.Day(), 23, 59, 59, 999999999, location)

	var Numbers []model.NumbersBlocked
	query := `SELECT n.id, n.number, n.client_id, n.success, n.stat_waiting, n.team_id, n.stop_expirid, t.stop_days FROM caf.numbers AS n
				LEFT JOIN caf.teams AS t ON n.team_id=t.id 
				WHERE t.strategy = $1
				AND t.filtration = $2
				AND n.first_load_at BETWEEN $3 AND $4
				AND n.blocked = $5`
	err = db.Select(&Numbers, query, "cause", true, startTime, stopTime, false)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("numbers not found")
		}
		return fmt.Errorf("failed to get numbers: %w", err)
	}

	// Перебираем выбранные номера
	for _, number := range Numbers {
		if number.Success { // Если у номера за предыдущие сутки был успешный вызов, то удаляем его из БД
			_, err := db.Exec("DELETE FROM caf.numbers WHERE id = $1", number.ID)
			if err != nil {
				return fmt.Errorf("failed to delete number: %w", err)
			}
		}

		currentDate := time.Now().In(location)
		// Разблокируем номер
		if number.StopExpirid != nil && currentDate.After(*number.StopExpirid) {
			err := BlockNumberActions(db, false, nil, nil, number.Number, number.ClientID, number.TeamID, nil)
			if err != nil {
				return fmt.Errorf("failed to unblock number actions: %w", err)
			}
		}

		if !number.StatWaiting { // Если статистика по номеру есть
			// Получаем список SIP - кодов для блокировки по команде
			var badSIPCausesDB pgtype.Int4Array
			err = db.Get(&badSIPCausesDB, "SELECT bad_sip_codes FROM caf.teams WHERE id = $1", number.TeamID)
			if err != nil {
				//continue
				return fmt.Errorf("failed to get sip codes for team: %w", err)
			}

			// Конвертируем срез pgtype.Int4Array в []int
			badSIPCauses := PgIntArr2IntArr(badSIPCausesDB)

			var reasonsStat []model.ReasonsStat
			err := db.Select(&reasonsStat, "SELECT * FROM caf.num_reasons WHERE num_id = $1", number.ID)
			if err != nil {
				//continue
				return fmt.Errorf("failed to get stat for number: %w", err)
			}

			existsSuccess := false
			signBlocked := false

			// Перебираем коды ответа по номеру из статистики
			for _, statCause := range reasonsStat {
				// Если в статистике есть sip_code 200, то помечаем переменную и выходим из цикла
				if statCause.SipCode == "200" {
					existsSuccess = true
					break // Выходим из первого цикла
				}
			}

			// Если 200 не найден, продолжаем проверку на блокировки
			if !existsSuccess {
				// Повторно перебираем коды ответа по номеру из статистики
				for _, statCause := range reasonsStat {
					for _, badCause := range badSIPCauses {
						// Если находим совпадение, помечаем переменную и выходим из цикла
						if strconv.Itoa(badCause) == statCause.SipCode {
							signBlocked = true
							break // Выход из внутреннего цикла
						}
					}
					if signBlocked {
						break // Выход из внешнего цикла, если блокировка найдена
					}
				}
			}

			if existsSuccess { // Если у номера за период был успешный вызов
				_, err := db.Exec("DELETE FROM caf.numbers WHERE id = $1", number.ID)
				if err != nil {
					return fmt.Errorf("failed to delete number: %w", err)
				}
			}

			if signBlocked {

				// Делаем то что нужно при блокировке по cause стратегии
				// Блокируем номер
				if number.StopExpirid == nil && number.StopDays != nil {
					blockToDate := currentDate.AddDate(0, 0, *number.StopDays)
					description := "Не было успешного отбоя"
					err := BlockNumberActions(db, true, &currentDate, &blockToDate, number.Number, number.ClientID, number.TeamID, &description)
					if err != nil {
						return fmt.Errorf("failed to block number actions: %w", err)
					}
				}

			}

			// Удаляем статистику по обработанному номеру
			_, err = db.Exec("DELETE FROM caf.num_reasons WHERE num_id = $1", number.ID)
			if err != nil {
				return fmt.Errorf("failed to delete number stat: %w", err)
			}

		}
	}

	return nil
}
