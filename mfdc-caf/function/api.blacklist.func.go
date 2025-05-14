package function

import (
	"caf/model"
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// List blocked godoc
// @Summary      Blacklist
// @Description  List blocked with filter by date
// @Tags         Blacklist
// @Accept       json
// @Produce      json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of logs per page" default(100)
// @Success      200  {array}   model.BLJsonResponse
// @Param data body model.LogRequest true "Data"
// @Router       /blacklist/view [post]
// @Security ApiKeyAuth
func GetBlacklist(db *sqlx.DB, c *gin.Context) {

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

	var blockedRequest model.BlockedRequest
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&blockedRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var blackList []model.BlackList
	query := `SELECT * FROM caf.blacklist WHERE 1=1`

	query_count := `SELECT COUNT(id) FROM caf.blacklist WHERE 1=1`

	var args []interface{}
	var args_count []interface{}
	paramIndex := 1 // Индекс для параметров

	if (blockedRequest.From_date != nil && isValidDateFormat(*blockedRequest.From_date)) &&
		(blockedRequest.To_date != nil && isValidDateFormat(*blockedRequest.To_date)) {

		query += fmt.Sprintf(" AND created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		query_count += fmt.Sprintf(" AND created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *blockedRequest.From_date, *blockedRequest.To_date)
		args_count = args
		paramIndex += 2
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Please provide a date range"})
		return
	}

	if blockedRequest.Number != nil && *blockedRequest.Number != "" {
		addCondition(&query, "number", paramIndex, &args, *blockedRequest.Number)
		addCondition(&query_count, "number", paramIndex, &args_count, *blockedRequest.Number)
		paramIndex++
	}

	if blockedRequest.TeamID != nil && *blockedRequest.TeamID != 0 {
		addCondition(&query, "team_id", paramIndex, &args, *blockedRequest.TeamID)
		addCondition(&query_count, "team_id", paramIndex, &args_count, *blockedRequest.TeamID)
		paramIndex++
	}

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	args = append(args, limit, offset)

	err := db.Select(&blackList, query, args...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get blacklist", "error": err.Error()})
		return
	}

	var rows_count int
	err = db.Get(&rows_count, query_count, args_count...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to count blocked numbers", "message": err.Error()})
		return
	}

	for idx, blockedNumber := range blackList {
		var Logs []model.BLLogs
		err := db.Select(&Logs, "SELECT created_at, description, filtered FROM caf.logs WHERE number = $1 ORDER BY created_at DESC", blockedNumber.Number)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to get logs for number", "message": err.Error()})
			return
		}
		blackList[idx].Logs = &Logs
	}

	response := model.LogJsonResponse{
		Status: "success",
		Count:  rows_count,
		Data:   blackList,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Delete blacklist godoc
// @Summary      Delete blacklist
// @Description  Drop blcacklist
// @Tags         Blacklist
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Block ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /blacklist/delete/{id} [delete]
// @Security ApiKeyAuth
func BLDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var rowNumber string
	err := db.QueryRow("SELECT number FROM caf.blacklist WHERE id = $1", id).Scan(&rowNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Row not found for deletion from blacklist"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check row on exists", "error": err.Error()})
		return
	}
	// Разблокируем номер
	err = BlockNumberActions(db, false, nil, nil, rowNumber, nil, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to unblock number actions", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Row from blacklist successfully deleted"})
}

// Blacklist add godoc
// @Summary      Add to blacklist
// @Description  Add to blacklist
// @Tags         Blacklist
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param blacklist body model.BlackListAdd true "Blacklist params"
// @Router       /blacklist/add [post]
// @Security ApiKeyAuth
// @Description JSON object containing resource IDs
func AddBL(db *sqlx.DB, c *gin.Context) {

	var request model.BlackListAdd

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Number == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Number must be not empty"})
		return
	}

	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to load timezone", "error": err.Error()})
		return
	}
	currentDate := time.Now().In(location)

	// Получение текущего логина из JWT
	currentLogin, _ := c.Get("email")
	var changeLogin string
	if currentLogin != nil {
		changeLogin = currentLogin.(string)
	} else {
		changeLogin = "User"
	}

	var description string
	if request.Description != nil {
		description = "(" + changeLogin + ")" + " " + *request.Description
	} else {
		description = "(" + changeLogin + ")"
	}

	var blockedID *int64
	addQuery := `INSERT INTO caf.blacklist (created_at, number, team_id, description) 
				VALUES ($1, $2, $3, $4) RETURNING id`

	err = db.QueryRow(addQuery, currentDate, request.Number, request.TeamID, description).Scan(&blockedID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new row to blacklist", "error": err.Error()})
		return
	}

	err = SendNumberToWebitel(*request.Number, description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed add to Webitel blacklist", "error": err.Error()})
		return
	}

	request.ID = blockedID

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Number successfully added to blacklist", "data": request})
}
