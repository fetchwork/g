package function

import (
	"billing-api/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// CDR get success call for period godoc
// @Summary      CDR get success call for period
// @Description  CDR get success call for period
// @Tags         CDR
// @Accept       json
// @Produce      json
// @Success      200  {array}  model.JsonResponse
// @Param data body model.CallCheck true "Call"
// @Router       /csc [post]
// @Security ApiKeyAuth
func CheckSuccessCallByNumber(db *sqlx.DB, c *gin.Context) {
	var call model.CallCheck

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&call); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Проверка на nil
	if call.FromDate == nil || call.ToDate == nil || call.Number == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Params must not be empty"})
		return
	}

	// Конвертация Unix времени в Time
	fromDate, err := ConvertUnixMillisToTime(*call.FromDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to convert unixtime for from time"})
		return
	}

	toDate, err := ConvertUnixMillisToTime(*call.ToDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to convert unixtime for to time"})
		return
	}

	var result []model.CheckResult
	query := `WITH call_data AS (
				SELECT sip_code, sip_reason
				FROM billing.calls 
				WHERE created BETWEEN $1 AND $2 
				AND callee = $3 
			)

			SELECT 
				sip_code,
				sip_reason,
				COUNT(*) AS count,
				COUNT(CASE WHEN sip_code = '200' THEN 1 END) > 0 AS ok,
				EXISTS (SELECT 1 FROM billing.calls WHERE created BETWEEN $1 AND $2 AND callee = $3) AS exists
			FROM call_data
			GROUP BY sip_code, sip_reason
			ORDER BY sip_code;
`
	err = db.Select(&result, query, fromDate, toDate, call.Number)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check exist call for requested number", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": result})
}
