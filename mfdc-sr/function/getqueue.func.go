package function

import (
	"fmt"
	"net/http"
	"sr-api/model"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Groups list godoc
// @Summary      List queues
// @Description  Get a list of all routes with pagination
// @Tags         Queues
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponseSwagger
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of routes per page" default(100)
// @Router       /list [get]
// @Security ApiKeyAuth
func GetList(c *gin.Context) {
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

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

	var slice []model.Queues
	var queue *string

	query := `SELECT * FROM sr.queues WHERE 1=1`
	var args []interface{}
	paramIndex := 1 // Индекс для параметров

	if queue != nil {
		query += fmt.Sprintf(" AND queue_name = $%d", paramIndex)
		args = append(args, *queue)
	}

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" ORDER By queue_name LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)

	args = append(args, limit, offset)

	// Выполняем запрос
	err = db.Select(&slice, query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch data", "error": err.Error(), "query": query})
		return
	}

	// Проверяем, есть ли данные на текущей странице
	if len(slice) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}
