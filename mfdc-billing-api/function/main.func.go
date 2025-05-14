package function

import (
	"billing-api/model"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Глобальная переменная для хранения конфигурации
var (
	config             model.Config
	configMutex        sync.RWMutex      // Мьютекс для безопасного доступа к конфигурации
	mu                 sync.Mutex        // Мьютекс для экспорта CSV, чтобы только одна горутина выполняла экспорт в один пик времени
	configReloadSignal = "config.reload" // Имя файла для сигнала перезагрузки
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
	if _, err := os.Stat("billing.json"); os.IsNotExist(err) {
		return errors.New("configuration file billing.json not found")
	}

	// Считывание конфигурации из файла
	data, err := os.ReadFile("billing.json")
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
func PGConnect() (*sqlx.DB, error) {
	config := GetConfig()

	// Подключение к PostgreSQL
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.PostgreSQL.Host,
		config.PostgreSQL.Port,
		config.PostgreSQL.User,
		config.PostgreSQL.Password,
		config.PostgreSQL.DBName)

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

// Функция для ручного вызова функции AggregateSum()
func StartAggregation(startDate time.Time, ctx context.Context, loc time.Location) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	currentDate := startDate

	for {
		select {
		case <-ticker.C:
			// Здесь мы передаем currentDate в AgregateSum
			OutLog.Println("Start aggregation task for date", currentDate)
			AggregateSum(currentDate, loc)
			currentDate = currentDate.AddDate(0, 0, 1)

		case <-ctx.Done():
			// Логируем завершение фоновой задачи
			OutLog.Println("Stopping aggregation task")
			return // Завершаем выполнение функции
		}
	}
}

func TaskAgregateSum(ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 05:00
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 5, 0, 0, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			OutLog.Println("Start aggregation task for date", time.Now())
			AggregateSum(time.Now(), *loc) // Вызываем функцию
		case <-ctx.Done():
			fmt.Println("Stopping aggregate data every day...")
			return // Завершаем выполнение функции
		}
	}
}

func AggregateSum(date time.Time, loc time.Location) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
	}
	defer db.Close()

	if date.IsZero() {
		date = time.Now()
	}

	var providers []model.ProvidersOnly
	err = db.Select(&providers, "SELECT pid, name, description FROM billing.providers")
	if err != nil {
		ErrLog.Printf("Failed to fetch providers: %s", err)
		return
	}

	yesterday := date.AddDate(0, 0, -1)
	startOfYesterday := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	endOfYesterday := startOfYesterday.Add(24 * time.Hour).Add(-time.Nanosecond)

	for _, provider := range providers {
		var data []model.Sum

		// Перебираем PID провайдеров из конфига у которых вычисление должно быть по дате завершения вызова
		var endCalc bool
		for _, configPid := range config.DateTimeCalcByEndPids {
			if provider.PID == configPid {
				endCalc = true
			}
		}

		var query string

		if endCalc { // Исключаем звонки менее 3 сек, отнимаем у всех звонков 1 сек и считаем время звонка по полю end_at
			query =
				`SELECT 
				p.name AS provider_name, 
				COALESCE(c.team, 'undefined') AS team,
				ROUND(SUM((GREATEST(c.duration - 1, 0) / 60.0) * c.rate), 2) AS bill_summ,
				ROUND(SUM(GREATEST(c.duration - 1, 0)) / 60.0, 2) AS talk_minutes,
				COUNT(c.cid) AS count 
			FROM billing.calls AS c 
			LEFT JOIN billing.providers AS p ON c.pid = p.pid 
			WHERE c.end_at BETWEEN $1 AND $2 
				AND c.pid = $3 
				AND c.sip_code = $4
				AND (c.duration - 1) <= 10800
				AND (c.duration - 1) >= 3
				AND (c.team IN (SELECT unnest($5::varchar[])) OR c.team IS NULL OR c.team = $6)
			GROUP BY p.name, c.team;`
		} else {
			query =
				`SELECT 
            p.name AS provider_name, 
            COALESCE(c.team, 'undefined') AS team,
            ROUND(SUM(c.bill), 2) AS bill_summ, 
            ROUND(SUM(c.duration) / 60.0, 2) AS talk_minutes,
            COUNT(c.cid) as count 
			FROM billing.calls AS c 
			LEFT JOIN billing.providers AS p ON c.pid = p.pid 
			WHERE c.created BETWEEN $1 AND $2 AND c.pid = $3 AND c.sip_code = $4
			AND c.duration <= 10800
			AND (c.team IN (SELECT unnest($5::varchar[])) OR c.team IS NULL OR c.team = $6)
			GROUP BY p.name, c.team`
		}

		// Преобразуем teamList в массив для передачи в SQL
		err = db.Select(&data, query, startOfYesterday, endOfYesterday, provider.PID, "200", pq.Array(config.TeamList), "")

		if err != nil {
			if err == sql.ErrNoRows {
				ErrLog.Printf("No data for provider %d: %s", provider.PID, err)
			} else {
				ErrLog.Printf("Failed to fetch data for provider %d: %s", provider.PID, err)
			}
			continue
		}
		// Прибавляем 12 часов
		logTime := startOfYesterday.Add(12 * time.Hour)

		for _, store := range data {
			_, err = db.Exec("INSERT INTO billing.sum (pid, provider_name, created, talk_minutes, bill_summ, team, count) VALUES ($1, $2, $3, $4, $5, $6, $7)",
				provider.PID, provider.Name, logTime, store.TalkMinutes, store.BillSumm, store.Team, store.Count)
			if err != nil {
				ErrLog.Printf("Failed to insert data for provider %d: %s", provider.PID, err)
				continue
			}
		}
	}
}

// Middleware для подключения к базе данных
func DBMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := PGConnect()
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

func CheckDB(c *gin.Context) (any, error) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}

// Функция для проверки выполнения задач по экспорту данных в CSV
func StartCSVCheker(db *sqlx.DB, ctx context.Context) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			OutLog.Println("Start export tasks checker")

			mu.Lock() // Блокируем мьютекс перед выполнением задачи
			err := WriteCSV(db)
			mu.Unlock() // Освобождаем мьютекс после завершения задачи

			if err != nil {
				OutLog.Println("Error writing CSV:", err)
			}

		case <-ctx.Done():
			// Логируем завершение фоновой задачи
			OutLog.Println("Stopping export tasks checker")
			return // Завершаем выполнение функции
		}
	}
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

// ConvertToUnixMillis принимает указатель на time.Time и возвращает Unix-время в миллисекундах.
func ConvertToUnixMillis(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond) // Конвертируем наносекунды в миллисекунды
}
