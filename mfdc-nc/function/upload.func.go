package function

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"nc/model"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func randomName(name string) string {
	// Генерация случайного 4-значного числа
	randomNumber := rand.Intn(9000) + 1000 // Генерируем число от 1000 до 9999

	name = fmt.Sprintf("%s.%d", name, randomNumber)
	return name
}

func checkPoolExists(db *sqlx.DB, pool_name string) (exists bool, err error) {
	err = db.Get(&exists, "SELECT EXISTS (SELECT 1 FROM nc.pools WHERE name = $1)", &pool_name)
	if err != nil {
		return false, fmt.Errorf("failed to check pool: %w", err)
	}
	return exists, nil
}

// Функция для создания нового пула
func createPool(db *sqlx.DB, name string, subpool_block int, vendor_id int, team_id int, num_count int, subpool_count int) (string, int, error) {
	var poolID int
	name = randomName(name)
	count, err := checkPoolExists(db, name)
	if err != nil {
		return "", 0, fmt.Errorf("failed to check pool: %w", err)
	}

	if count {
		name = randomName(name)
	}

	query := "INSERT INTO nc.pools (name, active, created_at, subpool_block, vendor_id, team_id, num_count, subpool_count) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id"

	err = db.QueryRow(query, name, false, time.Now(), subpool_block, vendor_id, team_id, num_count, subpool_count).Scan(&poolID)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create pool: %w", err)
	}

	return name, poolID, nil
}

// Функция для создания сабпула
func createSubPool(db *sqlx.DB, poolID int, index int) (int, error) {
	var subPoolID int
	query := "INSERT INTO nc.subpools (pool_id, status, index) VALUES ($1, $2, $3) RETURNING id"
	err := db.QueryRow(query, poolID, "inactive", index).Scan(&subPoolID)
	if err != nil {
		return 0, fmt.Errorf("failed to create subpool: %w", err)
	}
	return subPoolID, nil
}

// Функция для добавления номера
func createNumber(db *sqlx.DB, value string, label bool, pool_id int, subpool_id int, vendor_id int, team_id int) (int, error) {
	var numberID int
	query := "INSERT INTO nc.numbers (value, label, used, pool_id, subpool_id, vendor_id, team_id) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id"
	err := db.QueryRow(query, value, label, label, pool_id, subpool_id, vendor_id, team_id).Scan(&numberID)
	if err != nil {
		return 0, fmt.Errorf("failed to create number: %w", err)
	}
	return numberID, nil
}

// UploadNumbers обработчик загрузки номеров
// @Summary      Upload numbers
// @Description  Upload a file and JSON data
// @Tags         Numbers
// @Accept       multipart/form-data
// @Produce      json
// @Success      200  {array}  model.SwaggerDefaultResponse
// @Param file formData file true "CSV file"
// @Param data formData model.PoolNewRequest true "Pool params"
// @Router       /numbers/upload [post]
// @Security ApiKeyAuth
func UploadNumbers(db *sqlx.DB, c *gin.Context) {
	var newPool model.PoolNewRequest

	// Чтение данных из формы (JSON)
	if err := c.ShouldBind(&newPool); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	poolName := c.PostForm("name")
	teamIDS := c.PostForm("team_id")
	vendorIDS := c.PostForm("vendor_id")
	subPoolBlockS := c.PostForm("subpool_block")

	if poolName == "" || vendorIDS == "" || teamIDS == "" {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All fields are required"})
		return
	}

	newPool.Name = poolName // Сохранение в структуру

	vendorID, err := strconv.Atoi(vendorIDS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Vendor ID must be a number", "error": err.Error()})
		return
	}

	teamID, err := strconv.Atoi(teamIDS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Team ID must be a number", "error": err.Error()})
		return
	}

	subPoolBlock, err := strconv.Atoi(subPoolBlockS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Sub pool block must be a number", "error": err.Error()})
		return
	}

	newPool.VendorID = vendorID         // Сохранение в структуру
	newPool.TeamID = teamID             // Сохранение в структуру
	newPool.SubPoolBlock = subPoolBlock // Сохранение в структуру

	//OutLog.Printf("1: %s, 2: %s, 3: %s", PoolName, VendorID, Team)

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "File is required", "error": err.Error()})
		return
	}

	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Unable to open file", "error": err.Error()})
		return
	}
	defer src.Close()

	reader := csv.NewReader(src)
	records, err := reader.ReadAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Unable to read CSV", "error": err.Error()})
		return
	}

	var numbers []model.Numbers
	for _, record := range records {
		if len(record) > 0 {
			numbers = append(numbers, model.Numbers{Value: record[0], Label: false, Used: false}) // Устанавливаем статус по умолчанию
		}
	}

	var subpools [][]model.Numbers
	for i := 0; i < len(numbers); i += newPool.SubPoolBlock {
		end := i + newPool.SubPoolBlock
		if end > len(numbers) {
			end = len(numbers)
		}
		subpools = append(subpools, numbers[i:end])
	}

	poolName, poolID, err := createPool(db, newPool.Name, newPool.SubPoolBlock, newPool.VendorID, newPool.TeamID, len(records), len(subpools)) // Создаем новый пул с именем
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
		return
	}

	// Перебираем сабпулы
	for i, subpool := range subpools {
		subPoolID, err := createSubPool(db, poolID, i)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
			return
		}

		for _, number := range subpool {
			_, err := createNumber(db, number.Value, number.Label, poolID, subPoolID, newPool.VendorID, newPool.TeamID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": err.Error()})
				return
			}
			//OutLog.Printf("Inserted Number ID: %s into SubPool ID: %s\n", numberID, subPoolID) // Логируем вставленные номера
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Numbers processed and saved in database",
		"data": gin.H{
			"pool_id":       poolID,
			"pool_name":     poolName,
			"numbers_count": len(records),
		}})
}
