package function

import (
	"auth-api/model"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/swaggo/swag/example/celler/httputil"
)

var (
	// Глобальная переменная для хранения конфигурации
	config      model.Config
	configMutex sync.RWMutex // Мьютекс для безопасного доступа к конфигурации

	maxAttempts   int
	resetDuration time.Duration
	tokenExpires  time.Duration
	loginAttempts = make(map[string]*LoginAttempt)
)

type LoginAttempt struct {
	Count       int
	LastAttempt time.Time
}

// Самая первая автоматически-загружаемая функция
func init() {
	// Загружаем конфигурацию при запуске
	if err := LoadConfig(); err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	// Переменные для функции AuthUser()
	maxAttempts = config.API.MaxCountFailedLogin
	resetDuration = config.API.ResetAttemptMinut * time.Minute
	tokenExpires = config.API.TokenExpires * time.Hour
}

// Функция для проверки и загрузки конфигурации
func LoadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Проверка наличия файла billing.json
	if _, err := os.Stat("auth.json"); os.IsNotExist(err) {
		return errors.New("configuration file billing.json not found")
	}

	// Считывание конфигурации из файла
	data, err := os.ReadFile("auth.json")
	if err != nil {
		return err
	}

	// Парсинг JSON конфига
	if err := json.Unmarshal([]byte(data), &config); err != nil {
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
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update config", "error": err.Error()})
		return
	}
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Config successfully reloaded"})
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

// Функция для кодирования строки в SHA-256
func TextToSH256(input string) string {
	hash := sha256.New()                 // Создаем новый хеш
	hash.Write([]byte(input))            // Записываем строку в хеш
	hashBytes := hash.Sum(nil)           // Получаем хеш в виде байтового среза
	return hex.EncodeToString(hashBytes) // Кодируем байты в шестнадцатеричный формат и возвращаем
}

// CompareStringSlices сравнивает два среза []string и возвращает true, если они идентичны (по количеству элементов, значениям и порядку)
func CompareStringSlices(slice1, slice2 []string) bool {
	// Сравниваем длины срезов
	if len(slice1) != len(slice2) {
		return false
	}

	// Сравниваем элементы по порядку
	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}

	return true
}

// Функция для формирования среза []string из pgtype.TextArray
func getSectionsFromCurrent(current *pgtype.TextArray) []string {
	if current == nil {
		return []string{} // Возвращаем пустой срез, если current nil
	}

	if current.Status == pgtype.Present {
		sections := make([]string, len(current.Elements))
		for i, elem := range current.Elements {
			sections[i] = string(elem.String)
		}
		return sections
	}

	return []string{} // Возвращаем пустой срез, если статус не Present
}

func CreateToken(uid int, email string, firstname string, lastname string, role string, team_id int, sections pgtype.TextArray) (string, int64, error) {
	// Получаем текущую версию токена пользователя из БД
	version, err := getTokenVersion(email)
	if err != nil {
		return "", 0, err
	}

	var expires int64
	expires = time.Now().Add(tokenExpires).Unix()

	// Создаем новый токен
	claims := jwt.MapClaims{
		"uid":       uid,
		"email":     email,
		"firstname": firstname,
		"lastname":  lastname,
		"role":      role,
		"team_id":   team_id,
		"version":   version,
		"exp":       expires, // config.API.TokenExpires Устанавливаем срок действия токена в часах
	}

	// Добавляем секции только если они есть
	if len(sections.Elements) > 0 {
		claims["sections"] = sections.Elements
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Подписываем токен
	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		return "", 0, err
	}

	return tokenString, expires, nil
}

// Auth user godoc
// @Summary      Auth user
// @Description  User authorization
// @Tags         Login
// @Accept       json
// @Produce      json
// @Success      200  {array}  string
// @Param route body model.UserAuthRequest true "Data"
// @Router       /login [post]
// @Security ApiKeyAuth
func AuthUser(db *sqlx.DB, c *gin.Context) {
	var data model.UserAuthRequest

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid JSON data"})
		return
	}

	emailToCheck := data.Email // Используем email

	// Проверка попыток входа
	attempt, exists := loginAttempts[emailToCheck]
	if !exists {
		attempt = &LoginAttempt{Count: 0}
		loginAttempts[emailToCheck] = attempt
	}

	// Сбрасываем счетчик, если время последней попытки превышает лимит
	if time.Since(attempt.LastAttempt) > resetDuration {
		attempt.Count = 0
	}

	// Проверяем, достигнут ли лимит попыток
	if attempt.Count >= maxAttempts {
		c.JSON(http.StatusTooManyRequests, gin.H{"status": "failed", "message": "Too many login attempts. Please try again later."})
		return
	}

	var user model.UserAuth

	// Выполняем запрос для проверки наличия пользователя
	errSelect := db.Get(&user, "SELECT uid, email, firstname, lastname, password, role, team_id, sections FROM auth.users WHERE email=$1 AND enabled=true", emailToCheck)
	if errSelect != nil {
		if errSelect == sql.ErrNoRows {
			attempt.Count++
			attempt.LastAttempt = time.Now()
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "User not found"})
		} else {
			ErrLog.Printf("Error fetching user: %v\n", errSelect) // Логируем ошибку
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch user", "error": errSelect.Error()})
		}
		return
	}

	// Шифруем plain-text пароль в SHA-256
	pwdHash := TextToSH256(data.Password)
	if pwdHash == user.Password {
		// Сбрасываем счетчик при успешной аутентификации
		delete(loginAttempts, emailToCheck)

		// Создаем токен
		tokenString, expires, err := CreateToken(user.UID, user.Email, user.Firstname, user.Lastname, user.Role, user.TeamID, user.Sections)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to create token", "error": err.Error()})
			return
		}
		// Отправляем JWT-токен в качестве cookie на 12 часов
		c.SetCookie("jwt", tokenString, 43200, "/", "", false, true)
		c.IndentedJSON(http.StatusOK, gin.H{"jwt": tokenString, "expires": expires})
	} else {
		// Увеличиваем количество попыток при неудачной аутентификации
		attempt.Count++
		attempt.LastAttempt = time.Now()

		// Отправляем 401 Unauthorized
		httputil.NewError(c, http.StatusUnauthorized, errors.New("authentication failed"))
	}
}
