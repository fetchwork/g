package function

import (
	"cdr-api/model"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Tag add godoc
// @Summary      Add tag
// @Description  Add new tag
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartResponse
// @Param tag body model.Tag true "Tag name"
// @Router       /tags/add [post]
// @Security ApiKeyAuth
func AddTag(db *sqlx.DB, c *gin.Context) {
	var request model.Tag
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	if request.Name != nil {
		_, err := db.NamedExec("INSERT INTO cdr.tags (name) VALUES (:name)", &request)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new tag", "error": err.Error()})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Name must be not empty"})
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Tag successfully added"})
}

// Tag delete godoc
// @Summary      Delete tag
// @Description  Delete tag
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartResponse
// @Param        id   path      int  true  "Tag ID"
// @Router       /tags/{id} [delete]
// @Security ApiKeyAuth
func DeleteTag(db *sqlx.DB, c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := strconv.Atoi(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "TagID must be integer"})
		return
	}

	_, err := db.Exec("DELETE FROM cdr.tags WHERE id = $1", &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete route", "error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Tag has been deleted and all associated records have been unlinked"})
}

// Tags list godoc
// @Summary      Tags list
// @Description  Tags list
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDataResponse
// @Router       /tags/list [get]
// @Security ApiKeyAuth
func ListTags(db *sqlx.DB, c *gin.Context) {
	var slice []model.Tags

	err := db.Select(&slice, "SELECT * FROM cdr.tags")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get tags", "error": err.Error()})
		return
	}

	if len(slice) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "No data in slice"})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "data": slice})
}

// Tag push godoc
// @Summary      Push tag to call
// @Description  Push tag to call
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartResponse
// @Param slice body []model.TagInsert true "CallRowID + TagID"
// @Router       /tags/push [put]
// @Security ApiKeyAuth
func PushTag(db *sqlx.DB, c *gin.Context) {
	var request []model.TagInsert
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	if len(request) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "No data in slice"})
		return
	}

	// Перебираем срез
	for _, call := range request {
		_, err := db.Exec("UPDATE cdr.calls SET tag_id = $1 WHERE id = $2", call.TagID, call.CallRowID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update call row", "error": err.Error()})
			return
		}
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Tags successfully pushed"})
}
