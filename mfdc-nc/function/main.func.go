package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"nc/model"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

// Глобальная переменная для хранения конфигурации
var (
	config             model.Config
	configMutex        sync.RWMutex      // Мьютекс для безопасного доступа к конфигурации
	configReloadSignal = "config.reload" // Имя файла для сигнала перезагрузки
	subPoolDeactivate  = 0
	spdmu              sync.Mutex
	SwaggerAPIpath     = "/api/nc/" // /api/nc/
)

// Самая первая автоматически-загружаемая функция
func init() {
	// Загружаем конфигурацию при запуске
	if err := LoadConfig(); err != nil {
		ErrLog.Println("Error loading config:", err)
		return
	}
}

func LoadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Проверка наличия файла config.json
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		ErrLog.Printf("Configuration file config.json not found")
		return errors.New("configuration file config.json not found")
	}

	file, err := os.Open("config.json")
	if err != nil {
		ErrLog.Printf("Failed to open config file: %v", err)
		return fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		ErrLog.Printf("Failed to decode config: %v", err)
		return fmt.Errorf("failed to decode config: %v", err)
	}

	return nil
}

// Функция для получения текущей конфигурации
func GetConfig() model.Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}

// Reload config godoc
// @Summary      Execute reload config
// @Description  Reloading API configuration
// @Tags         Reload
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Reload
// @Router       /config/reload [get]
// @Security ApiKeyAuth
func UpdateConfig(c *gin.Context) {
	err := LoadConfig() // вызываем LoadConfig() для обновления
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "reload": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Reload successffuly"})
}

func MonitorConfigReload(ctx context.Context) {
	for {
		select {
		case <-time.After(5 * time.Second): // Пауза между проверками
			// Проверяем наличие файла сигнала
			if _, err := os.Stat(configReloadSignal); err == nil {
				err := LoadConfig() // вызываем LoadConfig() для обновления
				if err != nil {
					return
				}
				OutLog.Println("Configuration reloaded")
				// Удаляем файл после обработки сигнала
				os.Remove(configReloadSignal)
			}
		case <-ctx.Done(): // Если контекст завершен
			OutLog.Println("Stopping config reload monitoring...")
			return // Завершаем выполнение функции
		}
	}
}

// PGConnect подключается к PostgreSQL и возвращает указатель на базу данных и ошибку
func PGConnect(db_object string) (*sqlx.DB, error) {
	config := GetConfig()

	var (
		db_host     string
		db_port     int
		db_user     string
		db_password string
		db_name     string
	)

	if db_object == "" {
		db_host = config.PostgreSQL.Host
		db_port = config.PostgreSQL.Port
		db_user = config.PostgreSQL.User
		db_password = config.PostgreSQL.Password
		db_name = config.PostgreSQL.DBName
	}

	// Подключение к PostgreSQL
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		db_host,
		db_port,
		db_user,
		db_password,
		db_name)

	db, err := sqlx.Connect("pgx", psqlInfo)
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		return nil, err // Возвращаем nil и ошибку
	}

	// Проверка соединения с базой данных
	if err := db.Ping(); err != nil {
		ErrLog.Printf("Failed to ping PostgreSQL: %s", err)
		return nil, err // Возвращаем nil и ошибку
	}

	return db, nil // Возвращаем указатель на базу данных и nil в качестве ошибки
}

// Middleware для подключения к базе данных
func DBMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := PGConnect("")
		if err != nil {
			ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
			c.Abort() // Прерываем выполнение следующего обработчика
			return
		}
		defer db.Close()

		// Сохраняем db в контексте Gin
		c.Set("db", db)

		c.Next() // Продолжаем выполнение следующего обработчика
	}
}

// Функция конвертации из []int в pgtype.Int4Array
func IntArr2PgIntArr(int_arr []int) (pg_arr pgtype.Int4Array) {
	if len(int_arr) > 0 {
		pg_arr.Set(int_arr)
	} else {
		pg_arr.Set([]int{})
	}

	return pg_arr
}

// Функция конвертации из pgtype.Int4Array в []int
func PgIntArr2IntArr(pg_arr pgtype.Int4Array) []int {
	if pg_arr.Status == pgtype.Present {
		ids := make([]int, 0, len(pg_arr.Elements))
		for _, elem := range pg_arr.Elements {
			if elem.Status == pgtype.Present {
				ids = append(ids, int(elem.Int))
			}
		}
		return ids
	}
	return []int{}
}

// Функция для добавления условий в запрос
func addCondition(query *string, condition string, paramIndex int, args *[]interface{}, value interface{}) {
	*query += fmt.Sprintf(" AND %s = $%d", condition, paramIndex)
	*args = append(*args, value)
}

func isValidDateFormat(dateStr string) bool {
	// Регулярное выражение для проверки формата даты и время
	const dateFormat = `^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2}(\.\d{1,3})?)? ?([+-]\d{2}(:?\d{2})?|Z)?$`
	re := regexp.MustCompile(dateFormat)
	return re.MatchString(dateStr)
}

func CheckDB(c *gin.Context) (any, error) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}

func CheckIDAsInt(request_id string, c *gin.Context) {
	_, err := strconv.Atoi(request_id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "ID must be a number", "error": err.Error()})
		return
	}
}
