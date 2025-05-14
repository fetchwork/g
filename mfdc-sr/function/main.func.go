package function

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sr-api/model"
	"sync"
	"time"

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

	// Подключение к PostgreSQL
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
	}
	defer db.Close()

	// Создаём структуру БД если нет
	err = CreateTables(db)
	if err != nil {
		ErrLog.Printf("Error creating structure tables: %v", err)
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

	db_host = config.PostgreSQL.Host
	db_port = config.PostgreSQL.Port
	db_user = config.PostgreSQL.User
	db_password = config.PostgreSQL.Password
	db_name = config.PostgreSQL.DBName

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

func APIFetch(method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Если переданы данные, кодируем их в JSON
	if jsonData != nil {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("Failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)

	}
	//fmt.Printf("Request Body: %s\n", reqBody)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}
	//fmt.Println(req)

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.API_Webitel.Header, config.API_Webitel.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to read response body: %w", err)
	}

	// Возвращаем body ответа
	return body, resp.StatusCode, nil
}

const (
	createQueueTableSQL = `CREATE TABLE IF NOT EXISTS sr.queues (
		id serial4 NOT NULL,
		queue_id int4 NULL,
		queue_name varchar NULL,
		max_calls_100 varchar NULL,
		max_agent_line_100 varchar NULL,
		load_factor_100 varchar NULL,
		change_at timestamptz NULL,
		change_login varchar NULL,
		coef_mal float8 NULL,
		coef_lf float8 NULL,
		current_percent int4 NULL
	);
		CREATE TABLE IF NOT EXISTS sr.logs (
			id bigserial NOT NULL,
			queue_id int4 NULL,
			change_at timestamptz NULL,
			change_login varchar NULL,
			last_percent int4 NULL,
			new_percent int4 NULL,
			CONSTRAINT logs_pk PRIMARY KEY (id),
			CONSTRAINT logs_queues_fk FOREIGN KEY (queue_id) REFERENCES sr.queues(queue_id)
		);
		CREATE INDEX IF NOT EXISTS logs_change_at_idx ON sr.logs USING btree (change_at);
		CREATE INDEX IF NOT EXISTS logs_queue_id_idx ON sr.logs USING btree (queue_id);`
)

func CreateTables(db *sqlx.DB) error {
	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createQueueTableSQL)
	if err != nil {
		return err
	}

	return nil
}
