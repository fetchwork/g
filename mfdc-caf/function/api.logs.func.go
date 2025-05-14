package function

import (
	"caf/model"
	"fmt"
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
// @Param limit query int false "Number of logs per page" default(100)
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

	var logs []model.Logs
	query := `SELECT * FROM caf.logs WHERE 1=1`

	query_count := `SELECT COUNT(id) FROM caf.logs WHERE 1=1`

	var args []interface{}
	var args_count []interface{}
	paramIndex := 1 // Индекс для параметров

	if (logRequest.From_date != nil && isValidDateFormat(*logRequest.From_date)) &&
		(logRequest.To_date != nil && isValidDateFormat(*logRequest.To_date)) {

		query += fmt.Sprintf(" AND created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		query_count += fmt.Sprintf(" AND created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *logRequest.From_date, *logRequest.To_date)
		args_count = args
		paramIndex += 2
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Please provide a date range"})
		return
	}

	if logRequest.Number != nil && *logRequest.Number != "" {
		addCondition(&query, "number", paramIndex, &args, *logRequest.Number)
		addCondition(&query_count, "number", paramIndex, &args_count, *logRequest.Number)
		paramIndex++
	}

	if logRequest.TeamID != nil && *logRequest.TeamID != 0 {
		addCondition(&query, "team_id", paramIndex, &args, *logRequest.TeamID)
		addCondition(&query_count, "team_id", paramIndex, &args_count, *logRequest.TeamID)
		paramIndex++
	}

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
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
