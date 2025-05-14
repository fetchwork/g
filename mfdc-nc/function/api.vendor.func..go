package function

import (
	"nc/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// List vendors godoc
// @Summary      List vendors
// @Description  List vendors
// @Tags         Vendors
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerVendorList
// @Router       /vendors/list [get]
// @Security ApiKeyAuth
func VendorsList(db *sqlx.DB, c *gin.Context) {
	var vendors []model.Vendor
	err := db.Select(&vendors, "SELECT * FROM nc.vendors ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get vendors", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": vendors})
}

// Vendor add godoc
// @Summary      Add vendor
// @Description  Add new vendor
// @Tags         Vendors
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param route body model.VendorSimple true "Vendor"
// @Router       /vendors/add [post]
// @Security ApiKeyAuth
func AddVendor(db *sqlx.DB, c *gin.Context) {

	var request model.Vendor

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Name are required"})
		return
	}

	var vendorID int
	err := db.QueryRow("INSERT INTO nc.vendors (name) VALUES ($1) RETURNING id", *request.Name).Scan(&vendorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new vendor", "error": err.Error()})
		return
	}

	request.ID = &vendorID

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Vendor successfully added", "data:": request})
}

// Vendor schedule godoc
// @Summary      Edt vendor
// @Description  Edit vendor params
// @Tags         Vendors
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Vendor ID"
// @Param ip body model.VendorSimple true "Data without ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /vendors/edit/{id} [patch]
// @Security ApiKeyAuth
func VendorEdit(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var request model.Vendor

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	// Чтобы методом PATCH иметь возможно частично менять значения, прочитаем текущие
	var data model.Vendor
	err := db.Get(&data, "SELECT * FROM nc.vendors WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get vendor", "error": err.Error()})
	}

	// Проверяем и устанавливаем значения
	if request.Name == nil {
		request.Name = data.Name
	}

	_, err = db.Exec("UPDATE nc.vendors SET name = $1 WHERE id = $2", request.Name, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update vendor", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Vendor successfully edited"})
}

// Delete vendor godoc
// @Summary      Delete vendor
// @Description  Drop vendor
// @Tags         Vendors
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Vendor ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /vendors/delete/{id} [delete]
// @Security ApiKeyAuth
func VendorDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var vendorExists bool
	err := db.Get(&vendorExists, "SELECT EXISTS (SELECT 1 FROM nc.vendors WHERE id = $1)", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check vendor on exists", "error": err.Error()})
		return
	}

	if !vendorExists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Vendor not found for deletion"})
		return
	}

	_, err = db.Exec("DELETE FROM nc.vendors WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete vendor", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Vendor successfully deleted"})
}
