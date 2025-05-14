package function

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/swaggo/swag/example/celler/httputil"

	"github.com/gin-gonic/gin"
)

func SwaggerAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		config := GetConfig()
		authHeader := c.GetHeader("X-MFDC-Key") // Получаем заголовок
		if len(authHeader) == 0 {               // Проверяем, установлен ли
			httputil.NewError(c, http.StatusUnauthorized, errors.New("Authorization is required Header"))
			c.Abort()
		} else {
			if authHeader != config.API.Key { // Проверяем, подходит ли API-ключ
				httputil.NewError(c, http.StatusUnauthorized, fmt.Errorf("Invalid key X-MFDC-Key=%s", authHeader))
				c.Abort()
			}
		}
		c.Next()
	}
}
