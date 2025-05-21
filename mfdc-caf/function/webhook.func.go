package function

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Webhook call godoc
// @Summary      Webhook call
// @Description  Webhook call
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        class   path      string  true  "Type (success, try)"
// @Param        number   path      string  true  "Number"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /webhooks/{type}/{number} [get]
// @Security ApiKeyAuth
func CallHook(db *sqlx.DB, c *gin.Context) {
	rtype := c.Param("type")
	number := c.Param("number")

	if rtype == "" || number == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Params can't be empty"})
		return
	}

	// Проверяем что присланы цифры
	CheckIDAsInt(number, c)

	var err error
	var result sql.Result

	existNumber := false
	err = db.Get(&existNumber, "SELECT EXISTS (SELECT 1 FROM caf.numbers WHERE number = $1)", number)
	if err != nil {
		// Обработка ошибки при выполнении запроса
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check number for exists", "error": err.Error()})
		return
	}

	if !existNumber {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Number not found"})
		return
	}

	switch rtype {
	case "success":
		talk_sec := c.Query("talk_sec")
		talkSec := CheckIDAsInt(talk_sec, c)
		// Считаем успешным звонок только более 6 сек
		if talkSec > 6 {
			currentTime := time.Now()
			/*
				Если first_success_call_at равно NULL или меньше даты понедельника текущей недели, то ему присваивается значение $2.
				Если first_success_call_at больше или равно дате понедельника текущей недели, то second_success_call_at присваивается значение $2
			*/
			query := `UPDATE caf.numbers
						SET 
							today_success_call = $1,
							first_success_call_at = CASE 
								WHEN first_success_call_at IS NULL OR first_success_call_at < date_trunc('week', CURRENT_DATE) 
								THEN $2 
								ELSE first_success_call_at 
							END,
							second_success_call_at = CASE 
								WHEN first_success_call_at >= date_trunc('week', CURRENT_DATE) 
								THEN $2 
								ELSE second_success_call_at 
							END,
							success = $3,
							stat_waiting = $4
						WHERE number = $5;`
			result, err = db.Exec(query, true, currentTime, true, false, number)
		} else {
			c.JSON(http.StatusNotAcceptable, gin.H{"status": "failed", "message": "Talk time less than 6 sec, it is unsuccessful"})
			return
		}
	case "try":
		result, err = db.Exec("UPDATE caf.numbers SET attempts_counter = COALESCE(attempts_counter, 0) + 1 WHERE number = $1", number)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Param type is invalid"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update call for number", "error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to retrieve affected rows count", "error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Number not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Info updated for number: " + number})
}

// Webhook recheck number godoc
// @Summary      Webhook recheck number
// @Description  Webhook recheck number
// @Tags         Webhooks
// @Accept       json
// @Produce      json
// @Param        number   path      string  true  "Number"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /webhooks/recheck/{number} [get]
// @Security ApiKeyAuth
func RecheckNumberHook(db *sqlx.DB, c *gin.Context) {
	number := c.Param("number")

	if number == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Number can't be empty"})
		return
	}

	CheckIDAsInt(number, c)

	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to load timezone", "error": err.Error()})
		return
	}

	now := time.Now().In(location)

	result, err := db.Exec("UPDATE caf.numbers SET repeated_check = $1, first_load_at = $2 WHERE number = $3", true, now, number)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update number", "error": err.Error()})
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to retrieve affected rows count", "error": err.Error()})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Number not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Info updated for number: " + number})
}
