package function

import (
	"database/sql"
	"nc/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// List pools godoc
// @Summary      List pools
// @Description  List pools
// @Tags         Pools
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerPoolsList
// @Router       /pools/list [get]
// @Security ApiKeyAuth
func PoolsList(db *sqlx.DB, c *gin.Context) {
	var pools []model.Pool
	err := db.Select(&pools, "SELECT * FROM nc.pools ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get pool", "error": err.Error()})
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": pools})
}

// Activate pool godoc
// @Summary      Activate pool
// @Description  Activate pool
// @Tags         Pools
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Pool ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /pools/{id}/activate [get]
// @Security ApiKeyAuth
func ActivatePoolManual(db *sqlx.DB, c *gin.Context) {
	// Получение ID Pool из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var pool model.Pool
	err := db.Get(&pool, "SELECT id,active FROM nc.pools WHERE id=$1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Pool ID not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get pool", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	// Проверка на nil и значение активности
	if pool.Active == nil || *pool.Active {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Pool is already active"})
		return
	}
	// Активация пула
	newActiveStatus := true
	pool.Active = &newActiveStatus // Установка нового значения

	_, err = db.NamedExec("UPDATE nc.pools SET active=:active WHERE id=:id", &pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update pool in DB", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	response := "Pool ID " + id + " successfully activated"
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": response})
}

// Deactivate pool godoc
// @Summary      Deactivate pool
// @Description  Deactivate pool
// @Tags         Pools
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Pool ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /pools/{id}/deactivate [get]
// @Security ApiKeyAuth
func DeactivatePoolManual(db *sqlx.DB, c *gin.Context) {
	// Получение ID Pool из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var pool model.Pool
	err := db.Get(&pool, "SELECT id,active FROM nc.pools WHERE id=$1", id)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Pool ID not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get pool", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	// Проверка на nil и значение активности
	if pool.Active == nil || !*pool.Active {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Pool is already inactive"})
		return
	}

	// Деактивация пула
	newActiveStatus := false
	pool.Active = &newActiveStatus // Установка нового значения

	_, err = db.NamedExec("UPDATE nc.pools SET active=:active WHERE id=:id", &pool)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update pool in DB", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	response := "Pool ID " + id + " successfully deactivated"
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": response})
}

// Delete pool godoc
// @Summary      Delete pool
// @Description  Drop pool
// @Tags         Pools
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Pool ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /pools/delete/{id} [delete]
// @Security ApiKeyAuth
func PoolDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var poolExists bool
	err := db.Get(&poolExists, "SELECT EXISTS (SELECT 1 FROM nc.pools WHERE id = $1)", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check pool on exists", "error": err.Error()})
		return
	}

	if !poolExists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Pool not found for deletion"})
		return
	}

	_, err = db.Exec("DELETE FROM nc.pools WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete pool", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Pool successfully deleted"})
}

// Activate new subpool godoc
// @Summary      Manual activate next subpool
// @Description  Manual activate new subpool
// @Tags         Subpools
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /subpools/next [get]
// @Security ApiKeyAuth
func ActivateSubPoolManual(db *sqlx.DB, c *gin.Context) {
	_, err := activateNewSubPool(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to activate new subpool", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "All subpools rotated"})
}

// Activate new subpool for pool godoc
// @Summary      Manual activate next subpool for pool
// @Description  Manual activate new subpool for pool
// @Tags         Subpools
// @Accept       json
// @Produce      json
// @Param        pool_id   path      int  true  "Pool ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /subpools/{pool_id}/next [get]
// @Security ApiKeyAuth
func ActivateSubPoolManualForPool(db *sqlx.DB, c *gin.Context) {
	pool_id := c.Param("pool_id")
	CheckIDAsInt(pool_id, c)
	poolID, _ := strconv.Atoi(pool_id)

	_, err := activateNewSubPoolForPool(db, poolID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to activate new subpool", "error": err.Error()})
		return // Завершение выполнения функции при ошибке
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "New subpool activated for pool ID " + pool_id})
}

// Redistribution numbers godoc
// @Summary      Redistribution numbers
// @Description  Redistribution numbers
// @Tags         Pools
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param params body model.PoolRedistribution true "Params"
// @Router       /pools/numsmove [post]
// @Security ApiKeyAuth
func RedistributionPools(db *sqlx.DB, c *gin.Context) {

	var request model.PoolRedistribution

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Count == nil || request.FromPoolID == nil || request.ToPoolID != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All params are required"})
		return
	}

	// Функция перераспределения номеров

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Redistribution successfully", "data:": request})
}
