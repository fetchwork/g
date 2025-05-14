package function

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/jackc/pgx/stdlib"
)

var (
	serviceName = "cdr"
)

// Секретный ключ для подписи сигнатуры
var secretKey = []byte(config.API.Key)

func extractCookieToken(cookie string) string {
	// Находим индекс начала токена
	start := strings.Index(cookie, "jwt=")
	if start == -1 {
		return ""
	}

	// Сдвигаем индекс, чтобы получить только токен
	start += len("jwt=")

	// Находим конец токена (поиск следующего символа ';' или конца строки)
	end := strings.Index(cookie[start:], ";")
	if end == -1 {
		end = len(cookie)
	} else {
		end += start
	}

	return cookie[start:end]
}

var (
	tokenCache = sync.Map{} // Тип для безопасного и быстрого доступа из нескольких горутин
)

// Структура для хранения версии токена и времени кэширования
type cachedVersion struct {
	version   int
	timestamp time.Time
}

// Функция для получения версии токена с кэшированием
func getTokenVersion(username string) (int, error) {
	// Проверяем кэш
	if version, ok := tokenCache.Load(username); ok {
		// Проверяем, не истекло ли время кэширования
		if versionData, ok := version.(*cachedVersion); ok {
			if time.Since(versionData.timestamp) < config.API.TokenVersionCache*time.Minute {
				return versionData.version, nil
			}
		}
	}

	// Если версия не найдена или истекло время, получаем её из БД
	db, err := PGConnect("")
	if err != nil {
		return 0, err
	}
	defer db.Close()

	var version int
	err = db.Get(&version, "SELECT token_version FROM auth.users WHERE email = $1", username)
	if err != nil {
		return 0, err
	}

	// Сохраняем полученную версию в кэш с текущим временем
	tokenCache.Store(username, &cachedVersion{
		version:   version,
		timestamp: time.Now(),
	})

	return version, nil
}

func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Invalid signature method")
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		email, ok := claims["email"].(string) // email должен хранится в токене
		if !ok {
			return nil, fmt.Errorf("Email not found in token claims")
		}

		// Получаем версию токена из БД
		version, err := getTokenVersion(email)
		if err != nil {
			return nil, err
		}

		// Проверяем, равна ли версия из токена версии из БД
		if claims["version"] != float64(version) { // Приводим к float64, так как jwt.MapClaims использует float64 для чисел
			return nil, fmt.Errorf("Token version does not match")
		}

		return claims, nil
	}

	return nil, fmt.Errorf("Invalid token")
}

// Функция для проверки наличия подстроки из среза в строке
func containsSubstring(slice []string, path string) bool {
	for _, v := range slice {
		if strings.Contains(path, v) {
			return true
		}
	}
	return false
}

// Функция проверки на серые сети RFC 1918
func isPrivateIP(ip net.IP) bool {
	return ip.IsPrivate()
}

// Middleware функция проверки токена и прав доступа
func CheckUserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		jwtCookie := c.GetHeader("Cookie")   // Получаем заголовок Cookie
		mfdcKey := c.GetHeader("X-MFDC-Key") // Получаем заголовок X-MFDC-Key

		var token string
		if jwtCookie != "" {
			token = extractCookieToken(jwtCookie)
		} else if mfdcKey != "" {
			token = mfdcKey // Значение X-MFDC-Key как токен
		}

		// Определяем IP-адрес клиента
		clientIP := net.ParseIP(c.ClientIP())

		// Авторизационный ключ для других сервисов
		if mfdcKey != "" && isPrivateIP(clientIP) && mfdcKey == config.API.Key {
			c.Set("email", "robot@mfdc")
			c.Set("role", "admin")
			c.Set("firstname", "Robot")
			c.Set("lastname", "Service")
			// Если есть совпадение, продолжаем выполнение следующего обработчика
			c.Next()
		}

		// Проверяем, установлен ли токен
		if len(token) == 0 {
			c.AbortWithStatus(http.StatusUnauthorized) // Отправляем 401 Unauthorized
			return
		}

		// Проверяем токен
		claim, err := ValidateToken(token)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized) // Отправляем 401 Unauthorized
			return
		}

		// Сохраняем данные в контексте gin
		c.Set("email", claim["email"])
		c.Set("role", claim["role"])
		c.Set("firstname", claim["firstname"])
		c.Set("lastname", claim["lastname"])
		c.Set("exp", claim["exp"])
		c.Set("sections", claim["sections"])

		role, _ := c.Get("role")
		sections, existSections := c.Get("sections")

		if role == "admin" {
			c.Next()
		} else {
			if existSections {
				var sectionsSlice []string

				// Проверяем, является ли sections срезом строк
				if ss, ok := sections.([]string); ok {
					sectionsSlice = ss // Если это срез строк, присваиваем его
				} else if ss, ok := sections.([]interface{}); ok {
					// Если это срез интерфейсов, преобразуем его в срез строк
					for _, item := range ss {
						if str, ok := item.(string); ok {
							sectionsSlice = append(sectionsSlice, str)
						}
					}
				} else if str, ok := sections.(string); ok {
					// Если это просто строка, создаем срез из одной строки
					sectionsSlice = []string{str}
				} else {
					// Обработка случая, если sections не соответствует ожидаемым типам
					sectionsSlice = []string{}
				}

				// Проверяем наличие совпадения
				if containsSubstring(sectionsSlice, serviceName) {
					// Если есть совпадение, продолжаем выполнение следующего обработчика
					c.Next()
				} else {
					// Если нет совпадения, возвращаем ошибку
					c.JSON(http.StatusForbidden, gin.H{"status": "failed", "message": "Access denied"})
					c.Abort() // Прерываем выполнение следующего обработчика
				}
			}
		}
	}
}

// Midleware для авторизации при скачивании файла
func CheckDownloadAuth() gin.HandlerFunc {
	return func(c *gin.Context) {

		id := c.Param("id")
		token := c.Query("token")
		uid := c.Query("uid")

		// Если в URL прислан uid
		if uid != "" {
			// Сохраняем данные в контексте gin
			c.Set("uid", uid)
		}

		authHash := ForAuthSHA1Hash(id)

		// Определяем IP-адрес клиента
		clientIP := net.ParseIP(c.ClientIP())

		// Авторизационный ключ для других сервисов
		if token != "" && isPrivateIP(clientIP) && token == authHash {
			// Если есть совпадение, продолжаем выполнение следующего обработчика
			c.Next()
		} else {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}
}
