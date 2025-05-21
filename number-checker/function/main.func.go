package function

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"number-checker/model"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

// Глобальная переменная для хранения конфигурации
var (
	config             model.Config
	configMutex        sync.RWMutex      // Мьютекс для безопасного доступа к конфигурации
	configReloadSignal = "config.reload" // Имя файла для сигнала перезагрузки
)

// Самая первая автоматически-загружаемая функция
func init() {
	// Загружаем конфигурацию при запуске
	if err := LoadConfig(); err != nil {
		ErrLog.Println("Error loading config:", err)
		return
	}
}

// Функция для проверки и загрузки конфигурации
func LoadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	if _, err := os.Stat("number-checker.json"); os.IsNotExist(err) {
		return errors.New("configuration file number-checker.json not found")
	}

	data, err := os.ReadFile("number-checker.json")
	if err != nil {
		return err
	}

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

// PGConnect подключается к PostgreSQL и возвращает указатель на базу данных и ошибку
func PGConnect() (*sqlx.DB, error) {
	cfg := GetConfig()

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PostgreSQL.User,
		cfg.PostgreSQL.Password,
		cfg.PostgreSQL.Host,
		cfg.PostgreSQL.Port,
		cfg.PostgreSQL.DBName)

	db, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %v", err)
		return nil, err
	}

	if err := db.Ping(); err != nil {
		ErrLog.Printf("Failed to ping PostgreSQL: %v", err)
		return nil, err
	}

	return db, nil
}

// DBMiddleware передаёт готовое подключение к БД в контекст
func DBMiddleware(db *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	}
}

// UpdateConfig обновляет конфигурацию через API
func UpdateConfig(c *gin.Context) {
	if err := LoadConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "failed",
			"message": "Failed to reload config",
			"error":   err.Error(),
		})
		return
	}

	newCfg := GetConfig()
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Configuration reloaded",
		"data": gin.H{
			"api_bind": newCfg.API.Bind,
			"pg_host":  newCfg.PostgreSQL.Host,
		},
	})
}

// MonitorConfigReload следит за наличием файла сигнала перезагрузки
func MonitorConfigReload(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(configReloadSignal); err == nil {
				OutLog.Println("Config file changed, reloading...")
				if err := LoadConfig(); err != nil {
					ErrLog.Printf("Failed to reload config: %v", err)
				} else {
					OutLog.Println("Configuration successfully reloaded")
				}
				os.Remove(configReloadSignal)
			}
		case <-ctx.Done():
			OutLog.Println("Stopping config reload monitoring...")
			return
		}
	}
}
