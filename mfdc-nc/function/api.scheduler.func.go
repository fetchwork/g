package function

import (
	"nc/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Scheduler add godoc
// @Summary      Add scheduler
// @Description  Add new schedule
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param route body model.Scheduler true "Schedule, time format: 00:00:00+03"
// @Router       /schedule/add [post]
// @Security ApiKeyAuth
func AddSchedule(db *sqlx.DB, c *gin.Context) {

	var request model.Scheduler

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Name == nil || request.PeriodicSecond == nil || request.StartTime == nil || request.StopTime == nil || request.TeamID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All columns are required"})
		return
	}

	var scheduleID int
	err := db.QueryRow("INSERT INTO nc.scheduler (name, start_time, stop_time, periodic_sec, team_id) VALUES ($1, $2, $3, $4, $5) RETURNING id", *request.Name, *request.StartTime, *request.StopTime, *request.PeriodicSecond, *request.TeamID).Scan(&scheduleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new schedule", "error": err})
		return
	}

	request.ID = &scheduleID

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Schedule successfully added", "data": request})
}

// List scheduls godoc
// @Summary      List scheduls
// @Description  List scheduls
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Scheduler
// @Router       /schedule/list [get]
// @Security ApiKeyAuth
func SchedulsList(db *sqlx.DB, c *gin.Context) {
	data, err := GetSchedulerSlice(db)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get scheduls", "error": err.Error()})
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": data})
}

// Edit schedule godoc
// @Summary      Edt schedule
// @Description  Edit schedule params
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Schedule ID"
// @Param ip body model.Scheduler true "Data without ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /schedule/edit/{id} [patch]
// @Security ApiKeyAuth
func ScheduleEdit(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var request model.Scheduler

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	// Чтобы методом PATCH иметь возможно частично менять значения, прочитаем текущие
	var data model.Scheduler
	err := db.Get(&data, "SELECT * FROM nc.scheduler WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get schedule", "error": err.Error()})
	}

	// Проверяем и устанавливаем значения
	if request.Name == nil {
		request.Name = data.Name
	}
	if request.StartTime == nil {
		request.StartTime = data.StartTime
	}
	if request.StopTime == nil {
		request.StopTime = data.StopTime
	}
	if request.PeriodicSecond == nil {
		request.PeriodicSecond = data.PeriodicSecond
	}
	if request.TeamID == nil {
		request.TeamID = data.TeamID
	}
	if request.TeamID == nil {
		request.TeamID = data.TeamID
	}

	_, err = db.Exec("UPDATE nc.scheduler SET name = $1, start_time = $2, stop_time = $3, periodic_sec = $4, team_id = $5 WHERE id = $6", request.Name, request.StartTime, request.StopTime, request.PeriodicSecond, request.TeamID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update schedule", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Schedule successfully edited"})
}

// Delete schedule godoc
// @Summary      Delete schedule
// @Description  Drop schedule
// @Tags         Scheduler
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Schedule ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /schedule/delete/{id} [delete]
// @Security ApiKeyAuth
func ScheduleDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var scheduleExists bool
	err := db.Get(&scheduleExists, "SELECT EXISTS (SELECT 1 FROM nc.scheduler WHERE id = $1)", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check schedule on exists", "error": err.Error()})
		return
	}

	if !scheduleExists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Schedule not found for deletion"})
		return
	}

	_, err = db.Exec("DELETE FROM nc.scheduler WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete schedule", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Schedule successfully deleted"})
}
