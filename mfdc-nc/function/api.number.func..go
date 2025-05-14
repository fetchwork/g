package function

import (
	"database/sql"
	"nc/model"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Get number info godoc
// @Summary      Get number info
// @Description  Get number info
// @Tags         Numbers
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.NumberInfo
// @Param        number   path      int  true  "Number format 79XXXXXXXXX"
// @Router       /numbers/info/{number} [get]
// @Security ApiKeyAuth
func NumberInfo(db *sqlx.DB, c *gin.Context) {

	number := c.Param("number")
	CheckIDAsInt(number, c)

	query := `SELECT n.id, n.value, n.activated_at, n.used, n.active, n.spin, v.name AS vendor, t.name AS team FROM nc.numbers AS n
			LEFT JOIN nc.vendors AS v ON n.vendor_id=v.id
			LEFT JOIN nc.teams AS t ON n.team_id=t.id
			WHERE n.value = $1 LIMIT 1`

	var numberInfo model.NumberInfo
	err := db.Get(&numberInfo, query, number)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "No data for request"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get number info", "error": err.Error()})
		return
	}

	var numberLogs []model.NumberLogs
	err = db.Select(&numberLogs, "SELECT start_at, end_at FROM nc.logs WHERE number_id = $1 ORDER By start_at DESC", numberInfo.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get number logs", "error": err.Error()})
		return
	}

	numberInfo.Logs = numberLogs

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": numberInfo})
}

// Get number team info godoc
// @Summary      Get number team info
// @Description  Get number team info
// @Tags         Numbers
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.NumberTeamInfo
// @Param        number   path      int  true  "Number format 79XXXXXXXXX"
// @Router       /numbers/routing/{number} [get]
// @Security ApiKeyAuth
func NumberTeamInfo(db *sqlx.DB, c *gin.Context) {

	number := c.Param("number")
	CheckIDAsInt(number, c)

	query := `SELECT n.value, t.name AS team FROM nc.numbers AS n
			LEFT JOIN nc.teams AS t ON n.team_id=t.id
			WHERE n.value = $1 LIMIT 1`

	var numberInfo model.NumberTeamInfo
	err := db.Get(&numberInfo, query, number)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "No data for request"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get number info", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "number": numberInfo.Number, "routing": strings.ToLower(numberInfo.Team)})
}

// Get numbers in pool godoc
// @Summary      Get numbers in pool
// @Description  Get numbers in pool
// @Tags         Numbers
// @Accept       json
// @Produce      json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number per page" default(100)
// @Param filter query string false "Number filter"
// @Success      200  {array}   model.NumbersJsonResponse
// @Param        pool_id   path      int  true  "Pool ID"
// @Router       /numbers/list/{pool_id} [get]
// @Security ApiKeyAuth
func NumbersInPool(db *sqlx.DB, c *gin.Context) {

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

	poolID := c.Param("pool_id")
	filter := c.Query("filter")

	CheckIDAsInt(poolID, c)

	query := `SELECT n.id, n.value, n.enabled, n.subpool_id, v.name AS vendor, t.name AS team FROM nc.numbers AS n
			LEFT JOIN nc.vendors AS v ON n.vendor_id=v.id
			LEFT JOIN nc.teams AS t ON n.team_id=t.id
			WHERE n.pool_id = $1 AND n.value LIKE $2
			ORDER BY n.subpool_id LIMIT $3 OFFSET $4`

	query_count := `SELECT COUNT(id) FROM nc.numbers WHERE pool_id = $1 AND value LIKE $2`

	var numbers []model.NumbersInPool

	filterNumber := filter + "%"

	err := db.Select(&numbers, query, poolID, filterNumber, limitStr, offset)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Pool is empty"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get numbers from pool", "error": err.Error()})
		return
	}

	var rows_count int
	err = db.Get(&rows_count, query_count, poolID, filterNumber)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to count numbers", "message": err.Error()})
		return
	}

	response := model.NumbersJsonResponse{
		Status: "success",
		Count:  rows_count,
		Data:   numbers,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Removing a number from rotation godoc
// @Summary      Removing a number from rotation
// @Description  Removing a number from rotation
// @Tags         Numbers
// @Accept       json
// @Produce      json
// @Param ip body model.NumbersExclusion true "Numbers array"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /numbers/exclusion [patch]
// @Security ApiKeyAuth
func NumberExclusion(db *sqlx.DB, c *gin.Context) {

	var request model.NumbersExclusion

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if len(request.Numbers) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Numbers array is empty"})
		return
	}

	numCount := len(request.Numbers)
	numCountStr := strconv.Itoa(numCount)

	for _, num := range request.Numbers {
		_, err := db.Exec("UPDATE nc.numbers SET enabled = $1 WHERE id = $2", num.Enabled, num.NumberID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update number", "error": err.Error()})
			return
		}

	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": numCountStr + " numbers successfully edited"})
}
