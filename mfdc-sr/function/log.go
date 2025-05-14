package function

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sr-api/model"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

var (
	OutLog *log.Logger
	ErrLog *log.Logger
)

func init() {
	// Настройка логгера для stdout
	OutLog = log.New(os.Stdout, "", log.LstdFlags)

	// Настройка логгера для stderr
	ErrLog = log.New(os.Stderr, "", log.LstdFlags)
}

func ChangeLog(db *sqlx.DB, queue int, change_at time.Time, change_login string, last_percent int, new_percent int) error {
	_, err := db.Exec("INSERT INTO sr.logs (queue_id, change_at, change_login, last_percent, new_percent) VALUES ($1, $2, $3, $4, $5)", queue, change_at, change_login, last_percent, new_percent)
	if err != nil {
		return err
	}
	return nil
}

// List logs godoc
// @Summary      List logs
// @Description  List logs with filter by date
// @Tags         Logs
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponseStandardSwagger
// @Param data body model.LogRequest true "Data"
// @Router       /logs [post]
// @Security ApiKeyAuth
func GetLogs(c *gin.Context) {
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	var logRequest model.LogRequest
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&logRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var logs []model.Logs
	query := `SELECT l.*, q.queue_name FROM sr.logs AS l
		LEFT JOIN sr.queues AS q ON l.queue_id=q.queue_id 
		WHERE 1=1`

	var args []interface{}
	paramIndex := 1 // Индекс для параметров

	if logRequest.From_date != nil && logRequest.To_date != nil {
		query += fmt.Sprintf(" AND l.change_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *logRequest.From_date, *logRequest.To_date)
		paramIndex += 2
	}

	if logRequest.QueueID != nil {
		query += fmt.Sprintf(" AND l.queue_id = $%d", paramIndex)
		args = append(args, *logRequest.QueueID)
		paramIndex++
	}

	err = db.Select(&logs, query, args...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get logs", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "data": logs})
}
