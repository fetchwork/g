package function

import (
	"number-checker/model"

	"github.com/gin-gonic/gin"
)

// AbortWithError отправляет клиенту унифицированный ответ с ошибкой
func AbortWithError(c *gin.Context, status int, message, err string) {
	c.AbortWithStatusJSON(status, model.NewErrorResponse(message, err))
}
