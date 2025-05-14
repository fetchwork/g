package function

import (
	"bytes"
	"cdr-api/model"
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Глобальная переменная для хранения конфигурации
var (
	config             model.Config
	configMutex        sync.RWMutex      // Мьютекс для безопасного доступа к конфигурации
	APIPath            = "/api/cdr/"     // "/api/cdr/"
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

// Validate проверяет соответствие конфигурации необходимым условиям
func Validate(config model.Config) error {
	if config.S3.Bucket == "" {
		return errors.New("bucket cannot be empty")
	}
	if config.S3.Region == "" {
		return errors.New("region cannot be empty")
	}
	// Добавить дополнительные проверки по мере необходимости
	return nil
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

	if db_object == "dwh" {
		db_host = config.PostgreSQLDWH.Host
		db_port = config.PostgreSQLDWH.Port
		db_user = config.PostgreSQLDWH.User
		db_password = config.PostgreSQLDWH.Password
		db_name = config.PostgreSQLDWH.DBName
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

func CheckDB(c *gin.Context) (any, error) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database not found"})
		return nil, fmt.Errorf("database not found")
	}
	return db, nil
}

func StartAPI2DBTask(ctx context.Context) {
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		return
	}
	defer db.Close()

	ticker := time.NewTicker(config.API_Webitel.PeriodicCheckSecond * time.Second)
	defer ticker.Stop()

	for {
		select { // Ожидание событий от нескольких каналов
		case <-ticker.C: // Ожидаем данные из канала ticker с полем C (срабатывание таймера, сигнал. ticker тип time.Ticker)
			API2DB(db) // Вызываем функцию получения данных от API
		case <-ctx.Done(): // Если контекст горутины завершает родительский процесс
			OutLog.Println("Stopping data fetching from the API...")
			return // Завершаем выполнение функции
		}
	}
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

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.API_Webitel.Header, config.API_Webitel.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
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

// CheckJsonVars проверяет переменные JSON и устанавливает значение в dbVar
func CheckJsonTimeVars(jsonVar *string) (*time.Time, error) {
	if jsonVar != nil {
		convertedTime, err := ConvertUnixMillisToTime(*jsonVar)
		if err != nil {
			return nil, err // Возвращаем ошибку
		}
		return convertedTime, nil // Возвращаем преобразованное время
	}
	return nil, nil // Если jsonVar равно nil, возвращаем nil
}

func CheckJsonStringVars(jsonVar *string) *string {
	if jsonVar != nil {
		dbVar := jsonVar
		return dbVar
	}
	return nil // Если jsonVar равно nil, возвращаем nil
}

func CheckJsonIntVars(jsonVar *int) *int {
	if jsonVar != nil {
		dbVar := jsonVar
		return dbVar
	}
	return nil // Если jsonVar равно nil, возвращаем nil
}

func GenRecordPath(date *time.Time, file *string) *string {
	if file != nil {
		if date != nil {
			var record_path string
			record_date := date.UTC()
			// Извлекаем год, месяц и число
			year, month, day := record_date.Year(), record_date.Month(), record_date.Day()
			record_path = "1/" + strconv.Itoa(year) + "/" + strconv.Itoa(int(month)) + "/" + strconv.Itoa(day) + "/" + *file
			return &record_path
		}
	}
	return nil
}

func GenRecordPathWithHour(date *time.Time, file *string) *string {
	if file != nil && date != nil {
		// Получаем дату в формате UTC
		record_date := date.UTC()

		// Извлекаем год, месяц, день и час
		year, month, day, hour := record_date.Year(), record_date.Month(), record_date.Day(), record_date.Hour()

		// Формируем путь к записи
		record_path := "1/" + strconv.Itoa(year) + "/" + strconv.Itoa(int(month)) + "/" + strconv.Itoa(day) + "/" + strconv.Itoa(hour) + "/" + *file

		return &record_path
	}
	return nil
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

func S3FileExists(file string) (bool, error) {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		ErrLog.Printf("Failed to create S3 client: %v", err)
		return false, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Проверяем метаданные объекта
	_, err = minioClient.StatObject(context.Background(), config.S3.Bucket, file, minio.StatObjectOptions{})
	if err != nil {
		ErrLog.Printf("Error checking file existence: %v", err)
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil // Файл не существует
		}
		return false, fmt.Errorf("failed to get object info from S3 file not found: %w", err)
	}

	return true, nil // Файл существует
}

// S3GetSize получает размер файла в S3
func S3GetSize(file string) (int64, error) {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		ErrLog.Printf("Failed to create S3 client: %v", err)
		return 0, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Получаем метаданные объекта
	objectInfo, err := minioClient.StatObject(context.Background(), config.S3.Bucket, file, minio.StatObjectOptions{})
	if err != nil {
		ErrLog.Printf("Failed to get object info from S3: %v", err)
		return 0, fmt.Errorf("failed to get object info from S3: %w", err)
	}

	return objectInfo.Size, nil // Возвращаем размер объекта
}

func S3Get(file string, start, end *int64) (*minio.Object, error) {
	// Создаем новый клиент S3
	minioClient, err := minio.New(config.S3.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.S3.Key, config.S3.Secret, ""),
		Secure: true,
	})
	if err != nil {
		ErrLog.Printf("Failed to create S3 client: %v", err)
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	// Опции для получения объекта
	getObjectOptions := minio.GetObjectOptions{}

	// Устанавливаем диапазон, если он задан
	if start != nil && end != nil {
		getObjectOptions.SetRange(*start, *end)
	}

	// Получаем объект из S3
	object, err := minioClient.GetObject(context.Background(), config.S3.Bucket, file, getObjectOptions)
	if err != nil {
		ErrLog.Printf("Failed to get object from S3: %v", err)
		return nil, fmt.Errorf("failed to get object from S3: %w", err)
	}

	return object, nil // Возвращаем объект и nil, если нет ошибок
}

func CreateSectionCalls(db *sqlx.DB, create string) error {
	now := time.Now()
	var partitionTableName string
	var startDate string
	var endDate string

	// Если в функции аргумент future то формируем имя таблицы на следующий месяц
	if create == "future" {
		// Получаем следующий месяц и год
		nextMonth := now.Month() + 1
		nextYear := now.Year()

		// Обработка перехода на следующий год
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}

		// Формируем имя секционной таблицы
		partitionTableName = fmt.Sprintf("calls_%02d_%d", nextMonth, nextYear)

		// Форматируем даты
		startDate = fmt.Sprintf("%d-%02d-01 00:00:00.001", nextYear, nextMonth)

		// Переход к следующему месяцу для endDate
		endMonth := nextMonth + 1
		if endMonth > 12 {
			endMonth = 1
			nextYear++
		}

		endDate = fmt.Sprintf("%d-%02d-01 00:00:00.000", nextYear, endMonth)
	} else {
		// Получаем текущий месяц и год
		month := now.Month()
		year := now.Year()

		// Формируем имя секционной таблицы
		partitionTableName = fmt.Sprintf("calls_%02d_%d", month, year)

		// Форматируем даты
		startDate = fmt.Sprintf("%d-%02d-01 00:00:00.001", year, month)

		// Обработка перехода на следующий месяц
		nextMonth := int(month) + 1
		nextYear := year
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}

		endDate = fmt.Sprintf("%d-%02d-01 00:00:00.000", nextYear, nextMonth)
	}

	// Создание SQL-запроса
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS cdr.%s PARTITION OF cdr.calls FOR VALUES FROM ('%s') TO ('%s')",
		partitionTableName, startDate, endDate)

	// Выполнение запроса
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// SQL-запросы для создания индексов
	indexSQLs := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_call_id_idx ON cdr.%s USING btree (call_id);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_created_at_idx ON cdr.%s USING btree (created_at);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_from_number_idx ON cdr.%s USING btree (from_number);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_has_children_idx ON cdr.%s USING btree (has_children);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_parent_id_idx ON cdr.%s USING btree (parent_id);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_sip_code_idx ON cdr.%s USING btree (sip_code);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_to_number_idx ON cdr.%s USING btree (to_number);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_user_name_idx ON cdr.%s USING btree (user_name);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_tag_id_idx ON cdr.%s USING btree (tag_id);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_destination_idx ON cdr.%s USING btree (destination);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_talk_sec_idx ON cdr.%s USING btree (talk_sec);", partitionTableName, partitionTableName),
	}

	// Выполнение запросов на создание индексов
	for _, indexSQL := range indexSQLs {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}

	return nil
}

func CheckAndCreatePartition(ctx context.Context, db *sqlx.DB) {
	for {
		select {
		case <-ctx.Done():
			OutLog.Println("Stopping checking create section table...")
			return // Завершаем выполнение функции
		default:
			now := time.Now()

			// Проверка, является ли сегодня 1 число месяца
			if now.Day() == 1 {
				// Создаем секционную таблицу для calls
				err := CreateSectionCalls(db, "future")
				if err != nil {
					OutLog.Printf("Error creating tables: %v", err)
				} else {
					OutLog.Println("Create a section of the partition has been started")
				}

				// Ждем до следующего первого числа месяца
				nextMonth := now.Month() + 1
				nextYear := now.Year()
				if nextMonth > 12 {
					nextMonth = 1
					nextYear++
				}
				nextFirst := time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, time.UTC)

				// Вычисляем время ожидания до следующего первого числа месяца
				sleepDuration := time.Until(nextFirst)
				if sleepDuration > 0 {
					// Если время ожидания положительное, ждем
					select {
					case <-time.After(sleepDuration):
						// Возвращаемся к началу цикла
					case <-ctx.Done():
						OutLog.Println("Stopping checking create section table...")
						return
					}
				}
			} else {
				// Ждем сутки перед следующей проверкой
				select {
				case <-time.After(24 * time.Hour):
					// Возвращаемся к началу цикла
				case <-ctx.Done():
					OutLog.Println("Stopping checking create section table...")
					return
				}
			}
		}
	}
}

// token=hash
// где hash это sha1 хэш из file_id+unixtime(2025-03-17 15:04:05)
// где 2025-03-17 это текущее число, а время 15:04:05 константа
func ForAuthSHA1Hash(fileID string) string {
	// Получаем текущее время в зоне
	loc, _ := time.LoadLocation(config.API.TimeZone)

	// Получаем текущее время
	currentTime := time.Now().In(loc)

	// Форматируем дату
	dateNow := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, currentTime.Location())
	unixDate := dateNow.Unix()

	// Константное время
	constantTime := "15:04:05"

	// Создаем строку для хэширования
	inputString := fmt.Sprintf("%s%d%s", fileID, unixDate, constantTime)
	//OutLog.Println(dateNow)
	OutLog.Println(dateNow)
	OutLog.Println(inputString)

	// Вычисляем SHA-1 хэш
	hasher := sha1.New()
	hasher.Write([]byte(inputString))
	hashBytes := hasher.Sum(nil)

	// Преобразуем хэш в строку в шестнадцатеричном формате
	hashString := fmt.Sprintf("%x", hashBytes)

	return hashString
}

func AuthAPIFetch(method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Устанавливаем reqBody только если метод не GET и не DELETE
	if jsonData != nil && method != http.MethodGet && method != http.MethodDelete {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("Failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.AUTH_API.Header, config.AUTH_API.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
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

func GetFIO(db *sqlx.DB, uid string) (userFields model.AuthServiceResponseData, err error) {
	url := fmt.Sprintf("%s/users/%s/info", config.AUTH_API.URL, uid)

	body, statusCode, err := AuthAPIFetch("GET", url, "")
	if err != nil {
		return model.AuthServiceResponseData{}, fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	if statusCode != http.StatusOK {
		return model.AuthServiceResponseData{}, fmt.Errorf("bad response from VC API, status code: %d", statusCode)
	}

	var response model.AuthServiceResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return model.AuthServiceResponseData{}, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if response.Data != nil && response.Data.FirstName != nil && response.Data.LastName != nil {
		userFields = *response.Data // Заполнение userFields данными из response.Data
		return userFields, nil
	}

	return model.AuthServiceResponseData{}, nil
}
