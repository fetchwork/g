package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
	"vc-api/model"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

// Глобальная переменная для хранения конфигурации
var (
	config      model.Config
	configMutex sync.RWMutex // Мьютекс для безопасного доступа к конфигурации
)

// Самая первая автоматически-загружаемая функция
func init() {
	// Загружаем конфигурацию при запуске
	if err := LoadConfig(); err != nil {
		fmt.Println("Error loading config:", err)
		return
	}
}

// Функция для проверки и загрузки конфигурации
func LoadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Проверка наличия файла billing.json
	if _, err := os.Stat("config.json"); os.IsNotExist(err) {
		return errors.New("configuration file billing.json not found")
	}

	// Считывание конфигурации из файла
	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}

	// Парсинг JSON конфига
	if err := json.Unmarshal(data, &config); err != nil {
		return err
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

	if db_object == "webitel" {
		db_host = config.PostgreSQL_webitel.Host
		db_port = config.PostgreSQL_webitel.Port
		db_user = config.PostgreSQL_webitel.User
		db_password = config.PostgreSQL_webitel.Password
		db_name = config.PostgreSQL_webitel.DBName
	} else {
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

func StartCheckActualSub(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select { // Ожидание событий от нескольких каналов
		case <-ticker.C: // Ожидаем данные из канала ticker с полем C (срабатывание таймера, сигнал. ticker тип time.Ticker)
			OutLog.Println("Start Webitel sync")
			// Запускаем функцию активации сабпула
			err := CheckActualSub(db)
			if err != nil {
				ErrLog.Printf("Error: %s", err)
			}
		case <-ctx.Done(): // Если контекст горутины завершает родительский процесс
			OutLog.Println("Stopping Webitel sync...")
			return // Завершаем выполнение функции
		}
	}
}
