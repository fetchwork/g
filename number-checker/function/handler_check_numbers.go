package function

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// CheckNumbersRequest — структура входящего JSON-запроса
type CheckNumbersRequest struct {
	Numbers []string `json:"numbers"`
}

// HistoryNumber — структура данных из БД
type HistoryNumber struct {
	Number    string         `json:"number" db:"number"`
	Isp       sql.NullString `json:"isp,omitempty" db:"isp"`
	Pool      sql.NullString `json:"pool,omitempty" db:"pool"`
	UpdatedAt time.Time      `json:"updated_at" db:"updated_at"`
	CreatedAt sql.NullTime   `json:"created_at" db:"created_at"`
	DeletedAt sql.NullTime   `json:"deleted_at,omitempty" db:"delete_at"`
}

// MarshalJSON переопределяем, чтобы показывать deleted_at как false, если NULL
func (h HistoryNumber) MarshalJSON() ([]byte, error) {
	type Alias HistoryNumber

	var deletedAt interface{}
	if h.DeletedAt.Valid {
		deletedAt = h.DeletedAt.Time
	} else {
		deletedAt = nil
	}

	var CreatedAt interface{}
	if h.CreatedAt.Valid {
		CreatedAt = h.CreatedAt.Time
	} else {
		CreatedAt = nil
	}

	return json.Marshal(&struct {
		CreatedAt interface{} `json:"created_at"`
		DeletedAt interface{} `json:"deleted_at"`
		Isp       interface{} `json:"isp,omitempty"`
		Pool      interface{} `json:"pool,omitempty"`
	}{
		CreatedAt: CreatedAt,
		DeletedAt: deletedAt,
		Isp:       nullToEmpty(h.Isp),
		Pool:      nullToEmpty(h.Pool),
	})
}

// nullToEmpty преобразует sql.NullString в nil или строку
func nullToEmpty(ns sql.NullString) interface{} {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	return ns.String
}

// CheckNumberResponse — структура ответа для одного номера
type CheckNumberResponse struct {
	Number string         `json:"number"`
	Found  bool           `json:"found"`
	Data   *HistoryNumber `json:"data,omitempty"`
}

// Numbers list godoc
// @Summary      Numbers list
// @Description  Numbers list
// @Tags         Numbers
// @Accept       json
// @Produce      json
// @Param numbers body CheckNumbersRequest true "Numbers list"
// @Router       /check-numbers [post]
// @Security ApiKeyAuth
// CheckNumbers — обработчик POST /check-numbers
func CheckNumbers(db *sqlx.DB, c *gin.Context) {
	var req CheckNumbersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.Numbers) == 0 {
		c.JSON(http.StatusOK, gin.H{"result": []CheckNumberResponse{}})
		return
	}

	// Подготавливаем SQL-запрос
	placeholders := strings.Repeat(",?", len(req.Numbers)-1)
	query := fmt.Sprintf("SELECT number, isp, pool, updated_at, created_at, delete_at FROM history_numbers WHERE number IN (?%s)", placeholders)

	args := make([]interface{}, len(req.Numbers))
	for i, num := range req.Numbers {
		args[i] = num
	}

	var results []HistoryNumber
	err := db.Select(&results, query, args...)
	if err != nil {
		ErrLog.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Создаём мап для быстрого поиска результата по номеру
	resultMap := make(map[string]*HistoryNumber)
	for _, item := range results {
		resultMap[item.Number] = &item
	}

	// Формируем ответ по каждому номеру
	response := make([]CheckNumberResponse, 0, len(req.Numbers))
	for _, number := range req.Numbers {
		if data, found := resultMap[number]; found {
			response = append(response, CheckNumberResponse{
				Number: number,
				Found:  true,
				Data:   data,
			})
		} else {
			response = append(response, CheckNumberResponse{
				Number: number,
				Found:  false,
				Data:   nil,
			})
		}
	}

	// Возвращаем JSON
	c.JSON(http.StatusOK, gin.H{"result": response})
}
