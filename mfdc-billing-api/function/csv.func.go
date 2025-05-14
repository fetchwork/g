package function

import (
	"archive/zip"
	"billing-api/model"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Export list godoc
// @Summary      Export list
// @Description  Get a list of all exports with pagination
// @Tags         Export
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponse
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number per page" default(100)
// @Router       /export/list [get]
// @Security ApiKeyAuth
func GetExports(db *sqlx.DB, c *gin.Context) {

	// Получаем параметры пагинации из запроса
	pageStr := c.Query("page")   // Номер страницы
	limitStr := c.Query("limit") // Размер страницы

	// Устанавливаем значения по умолчанию
	page := 1
	limit := 100

	// Парсим параметры, если они указаны
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Рассчитываем OFFSET для SQL-запроса
	offset := (page - 1) * limit

	var slice []model.Export
	// Выбираем поля кроме content
	err := db.Select(&slice, "SELECT id, name, created_at, done FROM billing.export_tasks ORDER By created_at LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch exports", "error": err.Error()})
		return
	}

	// Проверяем, есть ли данные на текущей странице
	if len(slice) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	for i := range slice {
		if slice[i].Done {
			slice[i].DownloadURL = "/export/download/" + strconv.Itoa(slice[i].ID)
		}
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}

func AddExportTask(db *sqlx.DB, fileName string, jsonData interface{}) error {
	var taskExist bool
	err := db.Get(&taskExist, "SELECT EXISTS(SELECT 1 FROM billing.export_tasks WHERE name = $1)", fileName)
	if err != nil {
		return fmt.Errorf("Failed to get exists task: %s", err)
	}

	if taskExist {
		return fmt.Errorf("Exported data archive already exists for requested period")
	}

	_, err = db.Exec("INSERT INTO billing.export_tasks (name, created_at, new, args) VALUES ($1, $2, $3, $4)", fileName, time.Now(), true, jsonData)
	if err != nil {
		return err
	}
	return nil
}

func getExportParams(ARGs *json.RawMessage) (param model.CDRRequest, startTime *time.Time, stopTime *time.Time, err error) {
	// Преобразуем ARGs из JSON в структуру CDRRequest
	if ARGs != nil {
		err := json.Unmarshal(*ARGs, &param)
		if err != nil {
			return model.CDRRequest{}, nil, nil, fmt.Errorf("failed to unmarshal params: %w", err)
		}
	}

	// Парсим строки времени для преобразования из string в time.Time
	layout := "2006-01-02 15:04:05.000 -0700" // Формат, который соответствует исходной строке

	// Парсим startTime
	if param.From_date != nil {
		parsedStartTime, err := time.Parse(layout, *param.From_date)
		if err != nil {
			return model.CDRRequest{}, nil, nil, fmt.Errorf("error parsing start time: %w", err)
		}
		startTime = &parsedStartTime // Инициализируем указатель на время
	}

	// Парсим stopTime
	if param.To_date != nil {
		parsedStopTime, err := time.Parse(layout, *param.To_date)
		if err != nil {
			return model.CDRRequest{}, nil, nil, fmt.Errorf("error parsing stop time: %w", err)
		}
		stopTime = &parsedStopTime // Инициализируем указатель на время
	}

	return param, startTime, stopTime, nil
}

func WriteCSV(db *sqlx.DB) error {

	err := ClearOldExportTasks(db)
	if err != nil {
		return fmt.Errorf("failed to clear old tasks: %s", err)
	}

	// Запоминаем время начала выполнения
	startWork := time.Now()

	var providers []model.ProvidersOnly

	err = db.Select(&providers, "SELECT * FROM billing.providers")
	if err != nil {
		return fmt.Errorf("failed to fetch providers: %s", err)
	}

	// Создаем мапу для быстрого поиска имени провайдера по Pid
	providerMap := make(map[int]string)
	for _, provider := range providers {
		providerMap[provider.PID] = provider.Name
	}

	var data []model.Export
	err = db.Select(&data, "SELECT * FROM billing.export_tasks WHERE new = $1 AND saved = $2", true, false)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	// Перебираем новые задачи на экспорт
	for _, export := range data {
		param, startTime, stopTime, err := getExportParams(export.ARGs)
		if err != nil {
			return fmt.Errorf("failed to get params: %s", err)
		}

		// Копируем текущие значения диапазона в новые переменные для сохранения в файлы
		var startDate time.Time
		var stopDate time.Time
		if startTime != nil && stopTime != nil {
			startDate = *startTime
			stopDate = *stopTime
		}

		// Количество строк выбираемых в каждой порции запроса данных
		pageSize := 1000

		// Загружаем временную зону из конфигурации
		location, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %s", err)
		}

		// Признак что данные были извлечены
		dataExists := false

		// Перебираем дни в пределах диапазона от и до из задачи
		for currentDate := startTime; !currentDate.After(*stopTime); *currentDate = currentDate.AddDate(0, 0, 1) {
			query := "SELECT c.cid, c.pid, c.callid, c.created, c.callerid, c.callee, c.duration, c.rate, c.bill, c.rid, r.description AS route, c.sip_code, c.sip_reason, c.team FROM billing.calls AS c LEFT JOIN billing.routes AS r ON c.rid=r.rid WHERE 1=1"

			// Используем параметры для подготовки запроса
			args := []interface{}{}
			paramIndex := 1 // Индекс для параметров

			// Создаем переменную start, устанавливая время на 00:00:00.000 в указанной временной зоне
			start := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 0, 0, 0, 0, location)

			// Создаем переменную stop, устанавливая время на 23:59:59.999 в указанной временной зоне
			stop := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), 23, 59, 59, 999999999, location)

			// Проверяем, если stopTime < stop, то stop = stopTime
			if stopTime.Before(stop) {
				stop = *stopTime
			}

			query += fmt.Sprintf(" AND c.created BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
			args = append(args, start, stop)
			paramIndex += 2

			if param.CallID != nil {
				query += fmt.Sprintf(" AND c.callid = $%d", paramIndex)
				args = append(args, *param.CallID)
				paramIndex++
			}

			if param.Callee != nil {
				query += fmt.Sprintf(" AND c.callee = $%d", paramIndex)
				args = append(args, *param.Callee)
				paramIndex++
			}

			if param.CallerID != nil {
				query += fmt.Sprintf(" AND c.callerid = $%d", paramIndex)
				args = append(args, *param.CallerID)
				paramIndex++
			}

			if param.Sip_code != nil {
				query += fmt.Sprintf(" AND c.sip_code = $%d", paramIndex)
				args = append(args, *param.Sip_code)
				paramIndex++
			}

			if param.Sip_reason != nil {
				query += fmt.Sprintf(" AND c.sip_reason = $%d", paramIndex)
				args = append(args, *param.Sip_reason)
				paramIndex++
			}

			if param.Pids != nil && len(*param.Pids) != 0 {
				var PidList []int
				for _, pid := range *param.Pids {
					PidList = append(PidList, pid) // Добавляем PID в список
				}

				// Создаем строку для SQL-запроса
				var pidStrings []string
				for _, pid := range PidList {
					pidStrings = append(pidStrings, fmt.Sprintf("$%d", paramIndex)) // Используем $n для параметров
					args = append(args, pid)
					paramIndex++
				}

				// Объединяем элементы в строку через запятую
				pidString := strings.Join(pidStrings, ",")

				// Обновляем запрос с использованием параметров
				query += fmt.Sprintf(" AND c.pid IN(%s)", pidString)
			}

			if param.Teams != nil {
				var TeamsList []string
				// Итерируемся по элементам cdr_request.Teams
				for _, team_name := range *param.Teams {
					TeamsList = append(TeamsList, team_name) // Добавляем team в список
				}

				// Создаем строку для SQL-запроса
				var teamStrings []string
				for _, team := range TeamsList {
					teamStrings = append(teamStrings, fmt.Sprintf("$%d", paramIndex)) // Используем $n для параметров
					args = append(args, team)
				}

				// Объединяем элементы в строку через запятую
				teamsString := strings.Join(teamStrings, ",")

				// Обновляем запрос с использованием параметров
				query += fmt.Sprintf(" AND c.team IN(%s)", teamsString)
			}

			// Сортируем по убыванию даты
			query += " ORDER BY c.created DESC"
			// Задаём начальный индекс итерации по порциям
			page_idx := 0

			// Делаем пагинацию для запросов, чтобы выбирать данные порционно не более pageSize строк за раз
			for offset := 0; ; offset += pageSize {
				var cdrSlice []model.CDR

				// Устанавливаем лимиты для пагинации
				paginatedQuery := query + fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)

				// Добавляем параметры лимитов как целые числа
				args = append(args, int64(pageSize), int64(offset)) // Приводим к int64

				//OutLog.Println(paginatedQuery)
				//OutLog.Println(args...)

				// Выполняем запрос
				err := db.Select(&cdrSlice, paginatedQuery, args...)
				if err != nil {
					return fmt.Errorf("failed to execute paginated query: %w", err)
				}

				// Удаляем последние два элемента (LIMIT и OFFSET) из args для следующей итерации для пагинации
				args = args[:len(args)-2] // Убираем LIMIT и OFFSET

				if len(cdrSlice) == 0 {
					break // Выход из цикла, если больше нет данных
				} else {
					dataExists = true
				}

				// Проходим по каждому элементу cdr_slice и добавляем имя провайдера
				for i := range cdrSlice {
					if name, ok := providerMap[*cdrSlice[i].Pid]; ok {
						cdrSlice[i].Provider = name
					}
				}

				// Создание CSV заголовков
				var csvBuilder strings.Builder
				writer := csv.NewWriter(&csvBuilder)
				// Установка разделителя на точку с запятой
				writer.Comma = ';'

				// Заголовки CSV
				if page_idx == 0 { // Если это первая порция за день, то только в неё добавляем заголовки
					headers := []string{"Created", "CallerID", "Callee", "Duration", "Rate", "Bill", "Route", "SipCode", "SipReason", "Team", "Provider"}
					if err := writer.Write(headers); err != nil {
						return fmt.Errorf("failed to write headers to CSV: %s", err)
					}
				}

				// Заполнение CSV данными
				for _, cdr := range cdrSlice {
					var CallerID string
					var Callee string
					var Duration string
					var Rate string
					var Bill string
					var Route string
					var Sip_code string
					var Sip_reason string
					var Team string
					if cdr.CallerID != nil {
						CallerID = *cdr.CallerID
					}
					if cdr.Callee != nil {
						Callee = *cdr.Callee
					}

					if cdr.Duration != nil {
						Duration = fmt.Sprintf("%d", *cdr.Duration)
					}

					if cdr.Rate != nil {
						Rate = fmt.Sprintf("%v", *cdr.Rate)
					}

					if cdr.Bill != nil {
						Bill = fmt.Sprintf("%.2f", *cdr.Bill)
					}

					if cdr.Route != nil {
						Route = *cdr.Route
					}

					if cdr.Sip_code != nil {
						Sip_code = *cdr.Sip_code
					}

					if cdr.Sip_reason != nil {
						Sip_reason = *cdr.Sip_reason
					}
					if cdr.Team != nil {
						Team = *cdr.Team
					}
					record := []string{
						cdr.Created.Format("02.01.2006 15:04:05"),
						CallerID,
						Callee,
						Duration,
						Rate,
						Bill,
						Route,
						Sip_code,
						Sip_reason,
						Team,
						cdr.Provider,
					}
					if err := writer.Write(record); err != nil {
						return fmt.Errorf("failed to write record to CSV: %s", err)
					}
				}

				// Завершение записи и получение результата в строку
				writer.Flush()
				if err := writer.Error(); err != nil {
					return fmt.Errorf("error flushing writer: %s", err)
				}

				csvContent := csvBuilder.String()

				if page_idx == 0 { // Если это первая порция за день, то вставляем новую строку
					_, err = db.Exec("INSERT INTO billing.export_data (task_id, data_date, content) VALUES ($1, $2, $3)", export.ID, currentDate.Format("2006-01-02"), csvContent)
					if err != nil {
						return fmt.Errorf("failed to insert CSV content into DB: %s", err)
					}
				} else {
					// Запись CSV в таблицу с конкатенацией
					_, err = db.Exec("UPDATE billing.export_data SET content = COALESCE(content, '') || $1 WHERE data_date = $2", csvContent, currentDate.Format("2006-01-02"))
					if err != nil {
						return fmt.Errorf("failed to update CSV content into DB: %s", err)
					}
				}

				page_idx++

			}

		}

		if dataExists {
			// Обновляем статус о том, что данные сохранены в БД
			_, err = db.Exec("UPDATE billing.export_tasks SET saved = $1 WHERE id = $2", true, export.ID)
			if err != nil {
				return fmt.Errorf("failed to update saved status: %s", err)
			}

			err = generateCSVFiles(db, export.ID, &startDate, &stopDate, export.Name)
			if err != nil {
				return fmt.Errorf("failed to create csv files: %v", err)
			}
		}

		// Обновляем статус о том, что экспорт успешно завершён
		_, err = db.Exec("UPDATE billing.export_tasks SET done = $1, new = $2 WHERE id = $3", true, false, export.ID)
		if err != nil {
			return fmt.Errorf("failed to update succefully status: %s", err)
		}

	}

	// Запоминаем время окончания выполнения
	elapsedWork := time.Since(startWork)
	// Выводим время выполнения
	OutLog.Printf("Time since for export data: %s\n", elapsedWork)

	return nil
}

func generateCSVFiles(db *sqlx.DB, task_id int, startTime *time.Time, stopTime *time.Time, fileName string) error {
	OutLog.Println("Start generate CSV files")

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	defer zipWriter.Close()

	for currentDate := startTime; !currentDate.After(*stopTime); *currentDate = currentDate.AddDate(0, 0, 1) {
		query := `SELECT d.data_date, d.content FROM billing.export_data AS d
					LEFT JOIN billing.export_tasks AS t ON d.task_id=t.id
					WHERE d.task_id = $1 
					AND d.data_date = $2
					AND t.saved = $3`

		var data model.ExportDownload
		err := db.Get(&data, query, task_id, currentDate.Format("2006-01-02"), true)
		if err != nil {
			return fmt.Errorf("failed to get exported data: %v", err)
		}

		if data.Content == nil || *data.Content == "" {
			return fmt.Errorf("no data found for the given ID")
		}

		fileName := data.DataDate.Format("2006-01-02") + ".csv"

		buf := new(strings.Builder)
		writer := transform.NewWriter(buf, charmap.Windows1251.NewEncoder())
		csvWriter := csv.NewWriter(writer)
		csvWriter.Comma = ';'

		records := strings.Split(*data.Content, "\n")

		for _, record := range records {
			if record == "" {
				continue
			}

			fields := strings.Split(record, ";")
			if len(fields) == 11 {
				if err := csvWriter.Write(fields); err != nil {
					return fmt.Errorf("error writing row to CSV: %v", err)
				}
			} else {
				return fmt.Errorf("columns count mismatch, must be 11 but: %s", record)
			}
		}
		csvWriter.Flush()
		if err := writer.Close(); err != nil {
			return fmt.Errorf("error closing writer: %v", err)
		}

		// Запись в ZIP файл
		zipEntry, err := zipWriter.Create(fileName)
		if err != nil {
			return fmt.Errorf("failed to create zip entry: %v", err)
		}

		// Запись содержимого CSV в ZIP файл
		if _, err := io.Copy(zipEntry, strings.NewReader(buf.String())); err != nil {
			return fmt.Errorf("failed to write to zip entry: %v", err)
		}
	}

	// Закрываем ZIP архив после добавления всех файлов
	if err := zipWriter.Close(); err != nil {
		return fmt.Errorf("failed to close zip writer: %v", err)
	}

	// Загружаем ZIP в S3
	if err := UploadToS3(fileName+".zip", &buf); err != nil {
		return fmt.Errorf("failed to upload zip to S3: %v", err)
	}

	// Когда все файлы запакованы и архив отправлен в S3, удаляем данные из БД
	_, err := db.Exec("DELETE FROM billing.export_data WHERE task_id = $1", task_id)
	if err != nil {
		return fmt.Errorf("failed to delete used data: %v", err)
	}

	return nil
}

func DownloadCSVHandler(db *sqlx.DB, c *gin.Context) {
	id := c.Param("id")

	// Получение данных из одной ячейки
	query := "SELECT name FROM billing.export_tasks WHERE id = $1 AND done = $2"

	var fileName string
	err := db.Get(&fileName, query, id, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get exported data", "error": err.Error()})
		return
	}

	// Получаем файл из S3
	fileStream, err := GetFromS3(fileName + ".zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get file from S3", "error": err.Error()})
		return
	}
	defer fileStream.Close()

	// Заголовки для ответа
	c.Header("Content-Disposition", "attachment; filename="+fileName+".zip")
	c.Header("Content-Type", "application/zip")

	// Отправляем файл в ответе
	if _, err := io.Copy(c.Writer, fileStream); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to send file", "error": err.Error()})
		return
	}
}

// Очистка старых заданий
func ClearOldExportTasks(db *sqlx.DB) error {
	var slice []model.ExportMinimal
	// Выбираем поля кроме content
	err := db.Select(&slice, "SELECT id, created_at, name FROM billing.export_tasks WHERE done = $1", true)
	if err != nil {
		return fmt.Errorf("failed to fetch export tasks: %s", err)
	}

	// Проверяем, есть ли данные на текущей странице
	if len(slice) == 0 {
		return nil
	}

	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %s", err)
	}

	// Получаем текущую дату
	now := time.Now().In(location)

	// Вычисляем дату, которая была n месяцев назад
	monthsAgo := now.AddDate(0, -config.API.ExpiredExportTasksMonth, 0)

	// Перебираем исполненные задания
	for _, task := range slice {
		if task.CreatedAt.Before(monthsAgo) {
			err := DeleteTaskByID(db, task.ID, task.Name)
			if err != nil {
				return fmt.Errorf("failed to delete export task: %s", err)
			}
		}
	}

	return nil
}

// Удаление задания по ID
func DeleteTaskByID(db *sqlx.DB, taskID int, fileName string) error {
	// Выполняем запрос на удаление
	result, err := db.Exec("DELETE FROM billing.export_tasks WHERE id = $1 AND done = $2", taskID, true)
	if err != nil {
		return fmt.Errorf("failed to delete export task: %s", err)
	}

	// Проверяем количество затронутых строк
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %s", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("active task by id %d not found", taskID)
	}

	// Удаляем файл из S3
	err = DeleteFromS3(fileName + ".zip")
	if err != nil {
		return fmt.Errorf("failed to delete archive from S3: %s", err)
	}

	return nil
}
