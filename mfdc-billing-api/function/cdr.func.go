package function

import (
	"billing-api/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func isValidDateFormat(dateStr string) bool {
	// Регулярное выражение для проверки формата даты и время
	const dateFormat = `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2}(\.\d{1,3})?)? ?([+-]\d{2}(:?\d{2})?|Z)?$`
	re := regexp.MustCompile(dateFormat)
	return re.MatchString(dateStr)
}

// Функция для добавления условий в запрос
func addCondition(query *string, condition string, paramIndex int, args *[]interface{}, value interface{}) {
	*query += fmt.Sprintf(" AND %s = $%d", condition, paramIndex)
	*args = append(*args, value)
}

// CDR list godoc
// @Summary      Get CDR
// @Description  If there are no arguments, you get all the data page by page. The arguments are used as filters, one or more.
// @Tags         CDR
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponse
// @Param page query int false "Page number" default(1)
// @Param cdr body model.CDRRequest true "CDR Arguments, date format: 0000-00-00 00:00:00.000 +03"
// @Param limit query int false "Number of rows per page" default(100)
// @Router       /cdr/list [post]
// @Security ApiKeyAuth
func GetCDR(db *sqlx.DB, c *gin.Context) {

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

	var cdr_slice []model.CDR
	var cdr_request model.CDRRequest

	var providers []model.ProvidersOnly

	err := db.Select(&providers, "SELECT * FROM billing.providers")
	if err != nil {
		ErrLog.Printf("Failed to fetch providers: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to fetch providers list", "message": err.Error()})
		return
	}

	// Создаем мапу для быстрого поиска имени провайдера по Pid
	providerMap := make(map[int]string)
	for _, provider := range providers {
		providerMap[provider.PID] = provider.Name
	}

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&cdr_request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Начинаем строить запрос
	query := "SELECT c.cid, c.pid, c.callid, c.created, c.callerid, c.callee, c.duration, c.rate, c.bill, c.rid, r.description AS route, c.sip_code, c.sip_reason, c.team FROM billing.calls AS c LEFT JOIN billing.routes AS r ON c.rid=r.rid WHERE 1=1"
	query_count := "SELECT COUNT(c.cid) FROM billing.calls AS c LEFT JOIN billing.routes AS r ON c.rid=r.rid WHERE 1=1"
	var args []interface{}
	var args_count []interface{}
	paramIndex := 1 // Индекс для параметров

	// Создаем пустую карту для хранения JSON-объекта
	exportJsonMap := make(map[string]interface{})
	var exportFileName string

	if (cdr_request.From_date != nil && isValidDateFormat(*cdr_request.From_date)) &&
		(cdr_request.To_date != nil && isValidDateFormat(*cdr_request.To_date)) {

		query += fmt.Sprintf(" AND c.created BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		query_count += fmt.Sprintf(" AND c.created BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *cdr_request.From_date, *cdr_request.To_date)
		args_count = args
		paramIndex += 2

		exportJsonMap["from_date"] = *cdr_request.From_date
		exportJsonMap["to_date"] = *cdr_request.To_date

		// Парсим строки времени
		layout := "2006-01-02 15:04:05.000 -0700" // Формат, который соответствует исходной строке
		start, err := time.Parse(layout, *cdr_request.From_date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Error parsing start time:", "message": err.Error()})
			return
		}

		stop, err := time.Parse(layout, *cdr_request.To_date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Error parsing stop time:", "message": err.Error()})
			return
		}

		// Получаем текущую дату
		currentTime := time.Now()

		// Вычисляем дату, которая на 45 дней раньше текущей
		daysAgo := currentTime.AddDate(0, 0, -45)

		// Если значение start меньше чем 45 дней от текущей даты, то возвращаем ошибку
		if start.Before(daysAgo) {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "error": "Start date must be not older than 45 days"})
			return
		}

		// Форматируем время в нужный формат
		formattedStartTime := start.Format("02.01.2006_15-04")
		formattedStopTime := stop.Format("02.01.2006_15-04")

		exportFileName = formattedStartTime + "-" + formattedStopTime
	}

	if cdr_request.Pids != nil && len(*cdr_request.Pids) != 0 {
		var PidList []int
		// Итерируемся по элементам cdr_request.Pids
		for _, pid := range *cdr_request.Pids {
			PidList = append(PidList, pid) // Добавляем PID в список
		}

		// Создаем строку для SQL-запроса
		var pidStrings []string
		for _, pid := range PidList {
			pidStrings = append(pidStrings, fmt.Sprintf("$%d", paramIndex)) // Используем $n для параметров
			args = append(args, pid)
			args_count = append(args_count, pid) // Добавляем сам PID в args
			paramIndex++
		}

		// Объединяем элементы в строку через запятую
		pidString := strings.Join(pidStrings, ",")

		// Обновляем запрос с использованием параметров
		query += fmt.Sprintf(" AND c.pid IN(%s)", pidString)
		query_count += fmt.Sprintf(" AND c.pid IN(%s)", pidString)
		exportJsonMap["pids"] = *cdr_request.Pids
	}

	if cdr_request.CallID != nil && *cdr_request.CallID != "" {
		addCondition(&query, "c.callid", paramIndex, &args, *cdr_request.CallID)
		addCondition(&query_count, "c.callid", paramIndex, &args_count, *cdr_request.CallID)
		paramIndex++
		exportJsonMap["callid"] = *cdr_request.CallID
	}

	if cdr_request.CallerID != nil && *cdr_request.CallerID != "" {
		addCondition(&query, "c.callerid", paramIndex, &args, *cdr_request.CallerID)
		addCondition(&query_count, "c.callerid", paramIndex, &args_count, *cdr_request.CallerID)
		paramIndex++
		exportJsonMap["callerid"] = *cdr_request.CallerID
	}

	if cdr_request.Callee != nil && *cdr_request.Callee != "" {
		addCondition(&query, "c.callee", paramIndex, &args, *cdr_request.Callee)
		addCondition(&query_count, "c.callee", paramIndex, &args_count, *cdr_request.Callee)
		paramIndex++
		exportJsonMap["callee"] = *cdr_request.Callee
	}

	if cdr_request.Sip_code != nil && *cdr_request.Sip_code != "" {
		addCondition(&query, "c.sip_code", paramIndex, &args, *cdr_request.Sip_code)
		addCondition(&query_count, "c.sip_code", paramIndex, &args_count, *cdr_request.Sip_code)
		paramIndex++
		exportJsonMap["sip_code"] = *cdr_request.Sip_code
	}

	if cdr_request.Sip_reason != nil && *cdr_request.Sip_reason != "" {
		addCondition(&query, "c.sip_reason", paramIndex, &args, *cdr_request.Sip_reason)
		addCondition(&query_count, "c.sip_reason", paramIndex, &args_count, *cdr_request.Sip_reason)
		paramIndex++
		exportJsonMap["sip_reason"] = *cdr_request.Sip_reason
	}

	if cdr_request.Teams != nil {
		var TeamsList []string
		// Итерируемся по элементам cdr_request.Teams
		for _, team_name := range *cdr_request.Teams {
			TeamsList = append(TeamsList, team_name) // Добавляем team в список
		}

		// Создаем строку для SQL-запроса
		var teamStrings []string
		for _, team := range TeamsList {
			teamStrings = append(teamStrings, fmt.Sprintf("$%d", paramIndex)) // Используем $n для параметров
			args = append(args, team)
			args_count = append(args_count, team) // Добавляем сам Teams в args
			paramIndex++
		}

		// Объединяем элементы в строку через запятую
		teamsString := strings.Join(teamStrings, ",")

		// Обновляем запрос с использованием параметров
		query += fmt.Sprintf(" AND c.team IN(%s)", teamsString)
		query_count += fmt.Sprintf(" AND c.team IN(%s)", teamsString)
		exportJsonMap["teams"] = *cdr_request.Teams
	}

	// Сортируем по убыванию даты
	query += fmt.Sprintf(" ORDER BY c.created DESC")

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	args = append(args, limit, offset)

	if cdr_request.Export != nil && *cdr_request.Export {
		// Сериализуем карту в JSON
		ExportJsonData, err := json.Marshal(exportJsonMap)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Error marshaling JSON", "message": err.Error()})
			return
		}
		err = AddExportTask(db, exportFileName, ExportJsonData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to add new export task:", "message": err.Error()})
			return
		}
		c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Export task successfully added"})
	} else {

		// Выполняем запрос
		err = db.Select(&cdr_slice, query, args...)

		// Проверяем наличие данных
		if err != nil {
			if err == sql.ErrNoRows {
				// Данных нет
				c.JSON(http.StatusOK, gin.H{"status": "failed", "error": "No data found", "message": err.Error()})
			} else {
				// Обработка других ошибок
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to fetch CDR", "message": err.Error(), "args": args, "query": query})
			}
			return
		}

		var rows_count int
		err = db.QueryRow(query_count, args_count...).Scan(&rows_count)
		if err != nil {
			ErrLog.Printf("Failed to count CDR: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to count CDR", "message": err.Error(), "args": args_count, "query": query_count})
			return
		}

		// Проходим по каждому элементу cdr_slice и добавляем имя провайдера
		for i := range cdr_slice {
			if name, ok := providerMap[*cdr_slice[i].Pid]; ok {
				cdr_slice[i].Provider = name
			}
		}

		// Проверяем, есть ли данные на текущей странице
		if len(cdr_slice) == 0 {
			response := model.CDRJsonResponseNull{
				Status: "success",
				Data:   []string{},
			}
			c.JSON(http.StatusOK, response)
			return
		}

		response := model.CDRJsonResponse{
			Status: "success",
			Count:  rows_count,
			Data:   cdr_slice,
		}
		c.IndentedJSON(http.StatusOK, response)
	}
}

// CDR list godoc
// @Summary      Get Summ
// @Description  If there are no arguments, you get all the data page by page. The arguments are used as filters, one or more.
// @Tags         CDR
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponseSwagger
// @Param        id   path      int  true  "Provider ID"
// @Param cdr body model.CDRReportRequest true "CDR Arguments, date format: 0000-00-00 00:00:00.000 +03"
// @Router       /cdr/{id}/report [post]
// @Security ApiKeyAuth
func GetSumm(db *sqlx.DB, c *gin.Context) {
	id := c.Param("id")

	var cdr_slice model.CDRReport
	var cdr_request model.CDRReportRequest

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&cdr_request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Начинаем строить запрос
	query := "SELECT provider_name AS provider, ROUND(SUM(talk_minutes)) AS talk_minutes, ROUND(SUM(bill_summ)) AS bill_summ, SUM(count) AS count_calls FROM billing.sum WHERE 1=1"
	var args []interface{}
	paramIndex := 1 // Индекс для параметров

	if id != "" {
		if _, err := strconv.Atoi(id); err != nil { // Пытаемся преобразовать строку в целое число чтобы проверить что id это число
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Provider ID must be a number"})
			return
		} else {
			addCondition(&query, "pid", paramIndex, &args, &id)
			paramIndex++
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Provider ID is required"})
		return
	}

	teamCheckedDate := false
	if (cdr_request.From_date != nil && isValidDateFormat(*cdr_request.From_date)) &&
		(cdr_request.To_date != nil && isValidDateFormat(*cdr_request.To_date)) {

		query += fmt.Sprintf(" AND created BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *cdr_request.From_date, *cdr_request.To_date)
		paramIndex += 2
		teamCheckedDate = true
	}

	// Добавляем группировку
	query += " GROUP BY provider_name"

	// Добавляем даты в JSON ответ
	if cdr_request.From_date != nil {
		cdr_slice.From_date = *cdr_request.From_date
	}
	if cdr_request.To_date != nil {
		cdr_slice.To_date = *cdr_request.To_date
	}

	// Выполняем запрос
	err := db.Get(&cdr_slice, query, args...)

	// Проверяем наличие данных
	if err != nil {
		if err == sql.ErrNoRows {
			// Данных нет
			c.JSON(http.StatusNotFound, gin.H{"status": "failed", "error": "No data found", "message": err.Error()})
		} else {
			// Обработка других ошибок
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to fetch CDR", "message": err.Error(), "args": args, "query": query})
		}
		return
	}

	if cdr_request.Team != nil {
		var teamSlice model.CDRReportTeam

		for _, team := range *cdr_request.Team {
			var teamArgs []interface{}
			teamIndex := 1
			team_query := `SELECT team, ROUND(SUM(talk_minutes)) AS talk_minutes, ROUND(SUM(bill_summ)) AS bill_summ FROM billing.sum WHERE 1=1`

			if teamCheckedDate {
				team_query += fmt.Sprintf(" AND created BETWEEN $%d AND $%d", teamIndex, teamIndex+1)
				teamArgs = append(teamArgs, *cdr_request.From_date, *cdr_request.To_date)
				teamIndex += 2
			}

			addCondition(&team_query, "pid", teamIndex, &teamArgs, id)
			teamIndex++
			addCondition(&team_query, "team", teamIndex, &teamArgs, team)
			teamIndex++

			team_query += ` GROUP By team`
			err = db.Get(&teamSlice, team_query, teamArgs...)
			if err != nil {
				ErrLog.Println("Failed to fetch team:", err.Error())
				continue
			}
			// Добавляем запрошенные team в основной вывод
			cdr_slice.Team = append(cdr_slice.Team, teamSlice)
			//OutLog.Println("Query:", team_query)
		}
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   cdr_slice,
	}
	c.IndentedJSON(http.StatusOK, response)
}

// Delete route godoc
// @Summary      Delete export task
// @Description  Drop export task
// @Tags         Export
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Task ID"
// @Success      200  {array}   model.DeleteReply
// @Router       /export/{id} [delete]
// @Security ApiKeyAuth
func DeleteExportTaskByID(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	taskID, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Param ID no integer"})
		return
	}

	var taskName string

	// Выполняем запрос для проверки существования и получения имени
	err = db.Get(&taskName, "SELECT name FROM billing.export_tasks WHERE id = $1", taskID)

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Active task by id not found"})
			return
		} else {
			// Ошибка при выполнении запроса
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get task for delete", "error": err.Error()})
			return
		}
	} else {
		// Запись найдена, устанавливаем флаг существования
		err := DeleteTaskByID(db, taskID, taskName)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete export task", "error": err.Error()})
			return
		}
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Export task successfully deleted"})
}
