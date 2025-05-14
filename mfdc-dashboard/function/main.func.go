package function

import (
	"context"
	"dashboard/model"
	"encoding/json"
	"errors"
	"fmt"
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
	configMutex        sync.RWMutex        // Мьютекс для безопасного доступа к конфигурации
	configReloadSignal = "config.reload"   // Имя файла для сигнала перезагрузки
	SwaggerAPIpath     = "/api/dashboard/" // /api/dashboard/
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

	if db_object == "webitel" {
		db_host = config.PostgreSQLWebitel.Host
		db_port = config.PostgreSQLWebitel.Port
		db_user = config.PostgreSQLWebitel.User
		db_password = config.PostgreSQLWebitel.Password
		db_name = config.PostgreSQLWebitel.DBName
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
// Middleware для подключения к базе данных
func DBMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Подключение к основной базе данных
		db, err := PGConnect("")
		if err != nil {
			ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "failed",
				"message": "Database connection error",
				"error":   err.Error(),
			})
			c.Abort() // Прерываем выполнение следующего обработчика
			return
		}
		defer func() {
			if err := db.Close(); err != nil {
				ErrLog.Printf("Failed to close database connection: %s", err)
			}
		}()

		// Подключение к базе данных webitel
		dbWebitel, err := PGConnect("webitel")
		if err != nil {
			ErrLog.Printf("Failed to connect to PostgreSQL (webitel): %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "failed",
				"message": "Database connection error",
				"error":   err.Error(),
			})
			c.Abort() // Прерываем выполнение следующего обработчика
			return
		}
		defer func() {
			if err := dbWebitel.Close(); err != nil {
				ErrLog.Printf("Failed to close database connection (webitel): %s", err)
			}
		}()

		// Сохраняем db в контексте Gin
		c.Set("db", db)
		c.Set("db_webitel", dbWebitel)

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

func CheckDBWebitel(c *gin.Context) (any, error) {
	db, exists := c.Get("db_webitel")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}

func CheckIDAsInt(request_id string, c *gin.Context) (result int) {
	result, err := strconv.Atoi(request_id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "ID must be a number", "error": err.Error()})
		return
	}
	return result
}

// ConvertUnixMillisToTime преобразует время в миллисекундах в time.Time
func ConvertUnixMillisToTime(unixMillis string) (*time.Time, error) {
	if unixMillis == "" {
		return nil, errors.New("input string is empty")
	}

	millis, err := strconv.ParseInt(unixMillis, 10, 64)
	if err != nil {
		return nil, err
	}

	if millis == 0 {
		return nil, nil // Возвращаем nil для времени, если миллисекунды равны 0
	}

	t := time.UnixMilli(millis)
	return &t, nil
}

// ConvertTimeToUnixMillis принимает указатель на time.Time и возвращает Unix-время в миллисекундах.
func ConvertTimeToUnixMillis(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond) // Конвертируем наносекунды в миллисекунды
}

func LoadTimeLocation() (*time.Location, error) {
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("Failed to load location zone:", err)

	}
	return location, nil
}

func GetTeamIDFromJWT(c *gin.Context) (int, error) {
	// Получаем значение team_id из контекста
	teamIDInterface, ok := c.Get("team_id")
	if !ok {
		return 0, fmt.Errorf("failed to get TeamID from JWT")
	}

	// Пробуем преобразовать значение в int
	switch v := teamIDInterface.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil // Преобразуем float64 в int
	case string:
		id, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("failed to convert TeamID from string: %v", err)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("unexpected type for TeamID: %T", v)
	}
}

func getWebitelQueues(db *sqlx.DB, queues *json.RawMessage) ([]model.WebitelQueues, error) {
	var resourcesList []model.WebitelQueues

	// Проверяем, есть ли данные в queues
	if queues != nil {
		// Парсим JSON
		err := json.Unmarshal(*queues, &resourcesList)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %s, error: %w", string(*queues), err)
		}
	}

	return resourcesList, nil
}

func GetTeam(db *sqlx.DB, teamID int) (model.Team, error) {
	var teamDB model.TeamDB
	var team model.Team

	// Получаем данные о команде из базы данных
	err := db.Get(&teamDB, "SELECT * FROM dashboard.teams WHERE team_id = $1", teamID)
	if err != nil {
		return model.Team{}, fmt.Errorf("failed to get team with ID %d: %w", teamID, err)
	}

	// Копируем простые поля
	team.ID = teamDB.ID
	team.Name = teamDB.Name
	team.TeamID = teamDB.TeamID

	// Инициализируем указатели для массивов
	if teamDB.WebitelTeamIDS != nil {
		webitelTeamIDs := PgIntArr2IntArr(*teamDB.WebitelTeamIDS)
		team.WebitelTeamIDS = &webitelTeamIDs
	}

	if teamDB.WebitelQueues != nil {
		webitelQueues, err := getWebitelQueues(db, teamDB.WebitelQueues)
		if err != nil {
			return model.Team{}, fmt.Errorf("failed to get webitel queues for team ID %d: %w", teamID, err)
		}
		team.WebitelQueues = &webitelQueues
	}

	// Проверяем наличие обязательных полей
	if team.WebitelTeamIDS == nil || len(*team.WebitelTeamIDS) == 0 {
		return model.Team{}, fmt.Errorf("array webitel_team_ids is empty for team ID %d", teamID)
	}

	if team.WebitelQueues == nil || len(*team.WebitelQueues) == 0 {
		return model.Team{}, fmt.Errorf("array webitel_queues is empty for team ID %d", teamID)
	}

	return team, nil
}

// Принимает значение времени в миллисекундах (в формате строки) и возвращает строку в формате HH24:MI:SS с учетом часового пояса.
func FormatTimeInStatus(lastStatusChangeStr string) (string, error) {
	// Преобразуем строку в int64
	lastStatusChange, err := strconv.ParseInt(lastStatusChangeStr, 10, 64)
	if err != nil {
		return "", err
	}

	// Преобразуем миллисекунды в секунды
	seconds := lastStatusChange / 1000

	// Создаем объект времени из Unix времени
	t := time.Unix(seconds, 0)

	// Получаем указатель на нужный часовой пояс
	loc, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return "", err
	}

	// Применяем часовой пояс к времени
	t = t.In(loc)

	// Получаем текущее время в нужном часовом поясе
	now := time.Now().In(loc)

	// Вычисляем разницу во времени
	duration := now.Sub(t)

	// Получаем часы, минуты и секунды из разницы
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secondsDiff := int(duration.Seconds()) % 60

	// Форматируем строку в нужном формате HH24:MI:SS
	timeInStatus := fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secondsDiff)

	return timeInStatus, nil
}
