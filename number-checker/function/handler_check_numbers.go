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

// CheckNumbersRequest — входящий JSON-запрос
type CheckNumbersRequest struct {
	Numbers []string `json:"numbers"`
}

// HistoryNumber — данные из БД
type HistoryNumber struct {
	Number    string         `json:"number" db:"number"`
	Isp       sql.NullString `json:"isp" db:"isp"`
	Pool      sql.NullString `json:"pool" db:"pool"`
	CreatedAt sql.NullTime   `json:"created_at" db:"created_at"`
	DeletedAt sql.NullTime   `json:"deleted_at" db:"delete_at"`
}

// nullToEmpty — конвертирует sql.NullString в строку или "" если NULL
func nullToEmpty(ns sql.NullString) interface{} {
	if ns.Valid && ns.String != "" {
		return ns.String
	}
	return ""
}

// MarshalJSON — кастомная сериализация полей created_at и deleted_at
func (h HistoryNumber) MarshalJSON() ([]byte, error) {
	type Alias HistoryNumber

	var createdAt interface{}
	if h.CreatedAt.Valid {
		createdAt = h.CreatedAt.Time.Format(time.RFC3339)
	} else {
		createdAt = ""
	}

	var deletedAt interface{}
	if h.DeletedAt.Valid {
		deletedAt = h.DeletedAt.Time.Format(time.RFC3339)
	} else {
		deletedAt = ""
	}

	return json.Marshal(&struct {
		Number    string      `json:"number"`
		Isp       interface{} `json:"isp"`
		Pool      interface{} `json:"pool"`
		CreatedAt interface{} `json:"created_at"`
		DeletedAt interface{} `json:"deleted_at"`
	}{
		Number:    h.Number,
		Isp:       nullToEmpty(h.Isp),
		Pool:      nullToEmpty(h.Pool),
		CreatedAt: createdAt,
		DeletedAt: deletedAt,
	})
}

// CheckNumberResponseItem — элемент результата для одного номера
type CheckNumberResponseItem struct {
	Number string         `json:"number"`
	Found  bool           `json:"found"`
	Info   *HistoryNumber `json:"info,omitempty"`
}

// CheckNumberResponse — итоговый ответ API
type CheckNumberResponse struct {
	Status string                    `json:"status"` // "success", "error"
	Data   []CheckNumberResponseItem `json:"data,omitempty"`
	Error  string                    `json:"message,omitempty"`
}

// normalizePhoneNumber нормализует телефонный номер к формату 79991218679
func normalizePhoneNumber(number string) (string, error) {
	// Оставляем только цифры
	var digits strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	digitsStr := digits.String()

	// Проверяем длину
	switch len(digitsStr) {
	case 11:
		// Если начинается на 8 — меняем на 7
		if digitsStr[0] == '8' {
			digitsStr = "7" + digitsStr[1:]
		}
		return digitsStr, nil
	case 10:
		// Предполагаем, что это московский номер без кода страны — добавляем 7
		return "7" + digitsStr, nil
	default:
		return "", fmt.Errorf("неверный формат номера: %s", number)
	}
}

// Numbers list godoc
// @Summary Проверяет наличие номеров в базе данных
// @Description Возвращает информацию о каждом номере из списка: найден или нет, и дополнительные поля
// @Tags Numbers
// @Accept json
// @Produce json
// @Param numbers body CheckNumbersRequest true "Список номеров для проверки"
// @Success 200 {object} model.JsonResponse "Найденные номера"
// @Failure 400 {object} model.JsonResponseError "Ошибка в формате запроса"
// @Failure 500 {object} model.JsonResponseError "Ошибка сервера"
// @Router /check-numbers [post]
// @Security ApiKeyAuth
func CheckNumbers(db *sqlx.DB, c *gin.Context) {
	var req CheckNumbersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "invalid request body",
			"error":   err.Error(),
		})
		return
	}

	if len(req.Numbers) == 0 {
		c.JSON(http.StatusOK, CheckNumberResponse{
			Status: "success",
			Data:   []CheckNumberResponseItem{},
		})
		return
	}

	// Нормализуем номера
	normalizedNumbers := make([]string, 0, len(req.Numbers))
	for _, num := range req.Numbers {
		normalized, err := normalizePhoneNumber(num)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "invalid phone number format",
				"error":   err.Error(),
			})
			return
		}
		normalizedNumbers = append(normalizedNumbers, normalized)
	}

	// Подготавливаем SQL-запрос
	placeholders := strings.Repeat(",?", len(normalizedNumbers)-1)
	query := fmt.Sprintf("SELECT number, isp, pool, created_at, delete_at FROM history_numbers WHERE number IN (?%s)", placeholders)

	args := make([]interface{}, len(normalizedNumbers))
	for i, num := range normalizedNumbers {
		args[i] = num
	}

	var results []HistoryNumber
	err := db.Select(&results, query, args...)
	if err != nil {
		ErrLog.Printf("Database error: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, CheckNumberResponse{
			Status: "error",
			Error:  "database error: " + err.Error(),
		})
		return
	}

	// Создаём мап для быстрого поиска результата по номеру
	resultMap := make(map[string]HistoryNumber)
	for _, item := range results {
		resultMap[item.Number] = item
	}

	// Формируем ответ по каждому номеру
	responseData := make([]CheckNumberResponseItem, 0, len(normalizedNumbers))
	for _, number := range normalizedNumbers {
		if data, found := resultMap[number]; found {
			responseData = append(responseData, CheckNumberResponseItem{
				Number: number,
				Found:  true,
				Info:   &data,
			})
		} else {
			responseData = append(responseData, CheckNumberResponseItem{
				Number: number,
				Found:  false,
			})
		}
	}

	// Возвращаем успешный ответ
	c.JSON(http.StatusOK, CheckNumberResponse{
		Status: "success",
		Data:   responseData,
	})
}
