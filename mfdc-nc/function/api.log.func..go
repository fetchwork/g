package function

import (
	"fmt"
	"nc/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// List logs godoc
// @Summary      List logs
// @Description  List logs with filter by date
// @Tags         Logs
// @Accept       json
// @Produce      json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Logs per page" default(100)
// @Success      200  {array}   model.LogJsonResponse
// @Param data body model.LogRequest true "Data"
// @Router       /logs [post]
// @Security ApiKeyAuth
func GetLogs(db *sqlx.DB, c *gin.Context) {

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

	var logRequest model.LogRequest
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&logRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var logs []model.LogList
	query := `SELECT 
			l.start_at, 
			l.end_at, 
			l.comment,
			n.value AS number,
			t.name AS team,
			v.name AS vendor,
			p.name AS pool_name
			FROM nc.logs AS l
			LEFT JOIN nc.numbers AS n ON l.number_id=n.id
			LEFT JOIN nc.teams AS t ON l.team_id=t.id
			LEFT JOIN nc.vendors AS v ON l.vendor_id=v.id
			LEFT JOIN nc.pools AS p ON l.pool_id=p.id
			WHERE 1=1`

	query_count := `SELECT COUNT(l.id)
			FROM nc.logs AS l
			LEFT JOIN nc.numbers AS n ON l.number_id=n.id
			LEFT JOIN nc.teams AS t ON l.team_id=t.id
			LEFT JOIN nc.vendors AS v ON l.vendor_id=v.id
			LEFT JOIN nc.pools AS p ON l.pool_id=p.id
			WHERE 1=1`

	var args []interface{}
	var args_count []interface{}
	paramIndex := 1 // Индекс для параметров

	if (logRequest.From_date != nil && isValidDateFormat(*logRequest.From_date)) &&
		(logRequest.To_date != nil && isValidDateFormat(*logRequest.To_date)) {

		query += fmt.Sprintf(" AND l.start_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		query_count += fmt.Sprintf(" AND l.start_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *logRequest.From_date, *logRequest.To_date)
		args_count = args
		paramIndex += 2
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Please provide a date range"})
		return
	}

	if logRequest.Number != nil && *logRequest.Number != "" {
		addCondition(&query, "n.value", paramIndex, &args, *logRequest.Number)
		addCondition(&query_count, "n.value", paramIndex, &args_count, *logRequest.Number)
		paramIndex++
	}

	if logRequest.PoolID != nil && *logRequest.PoolID != 0 {
		addCondition(&query, "l.pool_id", paramIndex, &args, *logRequest.PoolID)
		addCondition(&query_count, "l.pool_id", paramIndex, &args_count, *logRequest.PoolID)
		paramIndex++
	}

	if logRequest.SubPoolID != nil && *logRequest.SubPoolID != 0 {
		addCondition(&query, "l.subpool_id", paramIndex, &args, *logRequest.SubPoolID)
		addCondition(&query_count, "l.subpool_id", paramIndex, &args_count, *logRequest.SubPoolID)
		paramIndex++
	}

	if logRequest.TeamID != nil && *logRequest.TeamID != 0 {
		addCondition(&query, "l.team_id", paramIndex, &args, *logRequest.TeamID)
		addCondition(&query_count, "l.team_id", paramIndex, &args_count, *logRequest.TeamID)
		paramIndex++
	}

	if logRequest.VendorID != nil && *logRequest.VendorID != 0 {
		addCondition(&query, "l.vendor_id", paramIndex, &args, *logRequest.VendorID)
		addCondition(&query_count, "l.vendor_id", paramIndex, &args_count, *logRequest.VendorID)
		paramIndex++
	}

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" ORDER BY l.start_at DESC LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	args = append(args, limit, offset)

	err := db.Select(&logs, query, args...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get logs", "error": err.Error()})
		return
	}

	var rows_count int
	err = db.Get(&rows_count, query_count, args_count...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to count logs", "message": err.Error()})
		return
	}

	response := model.LogJsonResponse{
		Status: "success",
		Count:  rows_count,
		Data:   logs,
	}

	c.IndentedJSON(http.StatusOK, response)
}
