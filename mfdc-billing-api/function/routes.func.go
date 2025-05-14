package function

import (
	"billing-api/model"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Routes list godoc
// @Summary      List routes
// @Description  Get a list of all routes with pagination
// @Tags         Routes
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Routes
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of routes per page" default(100)
// @Router       /routes/all [get]
// @Security ApiKeyAuth
func GetRoutes(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
	}
	defer db.Close()

	// Получаем параметры пагинации из запроса
	pageStr := c.Query("page")   // Номер страницы
	limitStr := c.Query("limit") // Размер страницы

	// Устанавливаем значения по умолчанию
	page := 1
	limit := 100

	// Парсим параметры, если они указаны
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Рассчитываем OFFSET для SQL-запроса
	offset := (page - 1) * limit

	var routes_slice []model.Routes

	err = db.Select(&routes_slice, "SELECT r.rid, r.description, r.prefix, r.cost, r.step, r.pid, p.name AS provider FROM billing.routes AS r LEFT JOIN billing.providers AS p ON r.pid=p.pid ORDER By prefix LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		ErrLog.Printf("Failed to fetch routes: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch routes", "error": err.Error()})
		return
	}

	// Проверяем, есть ли данные на текущей странице
	if len(routes_slice) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   routes_slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Routes add godoc
// @Summary      Add routes
// @Description  Add new route
// @Tags         Routes
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Routes
// @Param route body model.AddRoute true "Route"
// @Router       /routes/add [post]
// @Security ApiKeyAuth
func AddRoute(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
	}
	defer db.Close()

	var routes model.AddRoute

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&routes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	_, err = db.NamedExec("INSERT INTO billing.routes (description, prefix, cost, step, pid) VALUES (:description, :prefix, :cost, :step, :pid)", &routes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new route", "error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Route successfully added"})
}

// Edit route godoc
// @Summary      Edt route
// @Description  Edit route params
// @Tags         Routes
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Route ID"
// @Param ip body model.AddRoute true "Provider"
// @Success      200  {array}   model.AddRoute
// @Router       /routes/{id}/edit [patch]
// @Security ApiKeyAuth
func EditRoute(c *gin.Context) {
	// Подключение к базе данных
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	// Получение ID провайдера из URL
	id := c.Param("id")

	// Структура для хранения данных из запроса
	var route model.AddRoute
	var currentroute []model.AddRoute

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&route); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Чтобы методом PATCH иметь возможно частично менять значения, прочитаем текущие
	err = db.Select(&currentroute, "SELECT description, prefix, cost, step, pid FROM billing.routes WHERE rid = $1", &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to select current route", "error": err.Error()})
		return
	}

	if len(currentroute) == 0 {
		// Обработка случая, если currentroute пуст
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "No routes found"})
		return
	}

	// Получаем первый элемент из среза
	current := currentroute[0]

	// Проверяем и устанавливаем значения
	if route.Description == nil {
		route.Description = current.Description // Указатель на строку
	}
	if route.Prefix == nil {
		route.Prefix = current.Prefix // Указатель на строку
	}
	if route.Cost == nil {
		route.Cost = current.Cost // Указатель на float64
	}
	if route.Step == nil {
		route.Step = current.Step // Указатель на int
	}
	if route.Pid == nil {
		route.Pid = current.Pid // Указатель на int
	}

	_, err = db.Exec("UPDATE billing.routes SET description = $1, prefix = $2, cost = $3, step = $4, pid = $5 WHERE rid = $6", route.Description, route.Prefix, route.Cost, route.Step, route.Pid, &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update route", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Route successfully edited"})
}

// Delete route godoc
// @Summary      Delete route
// @Description  Drop route
// @Tags         Routes
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Route ID"
// @Success      200  {array}   model.DeleteReply
// @Router       /routes/{id} [delete]
// @Security ApiKeyAuth
func DeleteRouteByID(c *gin.Context) {
	// Подключение к базе данных
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	// Получение ID провайдера из URL
	id := c.Param("id")

	_, err = db.Exec("DELETE FROM billing.routes WHERE rid = $1", &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete route", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Route successfully deleted"})
}
