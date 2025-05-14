package function

import (
	"fmt"
	"math"
	"net/http"
	"sr-api/model"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Change queue godoc
// @Summary      Change queue
// @Description  Change queue sets
// @Tags         Queues
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Queue ID"
// @Param data body model.SetQueue true "Provider"
// @Success      200  {array}   model.JsonResponseStandardSwagger
// @Router       /edit/{id} [patch]
// @Security ApiKeyAuth
func ChangeQueue(c *gin.Context) {
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	// Получение ID из URL
	queueID := c.Param("id")
	if queueID != "" {
		// Пытаемся преобразовать строку в целое число чтобы проверить что queue_id это число
		if _, err := strconv.Atoi(queueID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Queue ID must be a number"})
			return
		}
		if len(queueID) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Resources must not be empty"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Queue ID is empty"})
		return
	}

	var Queue model.Queues
	err = db.Get(&Queue, "SELECT * FROM sr.queues WHERE queue_id = $1", queueID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch data", "error": err.Error()})
		return
	}
	var queueParam model.SetQueue
	if queueParam.Percent != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Percent cannot be null"})
		return
	}
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&queueParam); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var MaxCalls *int
	var MaxAgentLine *int
	var LoadFactor *int

	if Queue.MaxCalls100 != nil && queueParam.Percent != nil {
		maxCalls := (*Queue.MaxCalls100 / 100) * (*queueParam.Percent) // Вычисляем значение
		MaxCalls = &maxCalls                                           // Присваиваем адрес переменной
	}

	if Queue.MaxAgentLine100 != nil && queueParam.Percent != nil && Queue.CoefMaxAgentLine != nil {
		// Вычисляем MaxAgentLine как float64
		MaxAgentLineFloat := float64(*Queue.MaxAgentLine100) / 100 * float64(*queueParam.Percent) * *Queue.CoefMaxAgentLine
		maxAgentLine := int(MaxAgentLineFloat) // Приводим к int, отбрасывая дробную часть
		MaxAgentLine = &maxAgentLine           // Присваиваем адрес переменной
		// Устанавливаем *MaxAgentLine в диапазон от 1 до queue.MaxAgentLine100
		if *Queue.MaxAgentLine100 > 0 {
			// Устанавливаем *MaxAgentLine в диапазон от 1 до queue.MaxAgentLine100
			*MaxAgentLine = int(math.Max(1, math.Min(float64(*MaxAgentLine), float64(*Queue.MaxAgentLine100))))
		} else {
			// Обработка случая, когда MaxAgentLine100 <= 0
			*MaxAgentLine = 1
		}
	}

	if Queue.LoadFactor100 != nil && queueParam.Percent != nil && Queue.CoefLoadFactor != nil {
		LoadFactorFloat := float64(*Queue.LoadFactor100) / 100 * float64(*queueParam.Percent) * *Queue.CoefLoadFactor
		loadFactor := int(LoadFactorFloat) // Приводим к int
		LoadFactor = &loadFactor           // Присваиваем адрес переменной
		if *Queue.LoadFactor100 > 0 {
			*LoadFactor = int(math.Max(5, math.Min(float64(*LoadFactor), float64(*Queue.LoadFactor100))))
		} else {
			*LoadFactor = 5
		}
	}

	//OutLog.Printf("Percent: %d, MaxCalls: %d, MaxAgentLine: %d, LoadFactor: %d\n", *queueParam.Percent, *MaxCalls, *MaxAgentLine, *LoadFactor)
	if MaxCalls != nil && MaxAgentLine != nil && LoadFactor != nil {

		// Отправляем PATCH запрос в API Webitel по group_id
		requestBody := map[string]interface{}{
			"payload": map[string]int{
				"max_calls":      *MaxCalls,
				"max_agent_line": *MaxAgentLine,
				"load_factor":    *LoadFactor,
			},
		}

		updateURL := fmt.Sprintf("%s/call_center/queues/%s", config.API_Webitel.URL, string(queueID))
		_, statusCode, err := APIFetch("PATCH", updateURL, requestBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": fmt.Sprintf("Failed to update queue %s", queueID), "error": err.Error()})
			return
		}

		if statusCode <= 200 || statusCode > 300 {
			// Получение текущего времени
			currentTime := time.Now()
			// Получение текущего логина из JWT
			currentLogin, _ := c.Get("email")

			_, err = db.Exec("UPDATE sr.queues SET change_at = $1, change_login = $2, current_percent = $3 WHERE queue_id = $4", currentTime, currentLogin, queueParam.Percent, queueID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update changer info", "error": err.Error()})
				return
			}

			// Логируем
			var changeLogin string
			if currentLogin != nil {
				changeLogin = currentLogin.(string)
			} else {
				changeLogin = "testuser"
			}

			queue_id, _ := strconv.Atoi(queueID)
			var lastPercent int
			if Queue.CurrentPercent != nil {
				lastPercent = *Queue.CurrentPercent
			} else {
				lastPercent = 0
			}
			err = ChangeLog(db, queue_id, currentTime, changeLogin, lastPercent, *queueParam.Percent)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to add change log", "error": err.Error()})
				return
			}
			// Отправляем JSON-ответ
			c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Queue successfully edited"})
		}
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": fmt.Sprintf("One or more parameters provided are null for queue %s", queueID)})
		return
	}
}
