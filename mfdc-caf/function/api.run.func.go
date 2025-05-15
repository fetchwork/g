package function

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Get manual stat godoc
// @Summary      Get manual stat
// @Description  Get manual stat
// @Tags         Run Method
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /runmethod/stat [get]
// @Security ApiKeyAuth
func RunCompareNumberToStat(db *sqlx.DB, c *gin.Context, ctx context.Context) {
	// Запуск функции проверки статистики номеров
	go StartCompareNumberToStat(db, ctx, "now")

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": ""})
}
