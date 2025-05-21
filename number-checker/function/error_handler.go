package function

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Используем AbortWithError из текущего пакета (т.к. мы внутри package function)
				AbortWithError(c, http.StatusInternalServerError, "Internal server error", fmt.Sprintf("%v", err))
			}
		}()

		c.Next()

		if c.Writer.Written() {
			return
		}

		status := c.Writer.Status()
		if status >= 400 {
			var errMsg string
			var message string

			if len(c.Errors) > 0 {
				err := c.Errors.Last()
				errMsg = err.Error()
				message = http.StatusText(status)
			} else {
				errMsg = "empty response body"
				message = http.StatusText(status)
			}

			// Вызываем AbortWithError из текущего пакета
			AbortWithError(c, status, message, errMsg)
		}
	}
}
