package function

import (
	"billing-api/model"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

// Providers list godoc
// @Summary      List providers
// @Description  Get list all provider
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Providers
// @Router       /providers/all [get]
// @Security ApiKeyAuth
func GetProviders(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	providers_row, err := db.Query("SELECT p.pid, p.name, p.method, p.description, pa.ip FROM billing.providers AS p LEFT JOIN billing.providers_address AS pa ON p.pid = pa.pid ORDER By p.name")
	if err != nil {
		ErrLog.Printf("Failed to fetch providers: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch providers", "error": err.Error()})
		return
	}
	defer providers_row.Close() // Закрываем rows после использования

	RenderProviders(providers_row, c)
}

// Get provider godoc
// @Summary      Show one provider params
// @Description  Get provider by ID
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Provider ID"
// @Success      200  {array}   model.Providers
// @Router       /providers/{id} [get]
// @Security ApiKeyAuth
func GetProvidersByID(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	id := c.Param("id")

	providers_row, err := db.Query("SELECT p.pid, p.name, p.method, p.description, pa.ip FROM billing.providers AS p LEFT JOIN billing.providers_address AS pa ON p.pid = pa.pid WHERE p.pid = $1", &id)
	if err != nil {
		ErrLog.Printf("Failed to fetch providers: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch providers", "error": err.Error()})
		return
	}
	defer providers_row.Close()

	RenderProviders(providers_row, c)
}

func RenderProviders(providers_row *sql.Rows, c *gin.Context) {
	providers_map := make(map[int]*model.Providers)

	for providers_row.Next() {
		provider := model.Providers{}
		var ipAddr []byte

		err := providers_row.Scan(&provider.ID, &provider.Name, &provider.Method, &provider.Description, &ipAddr)
		if err != nil {
			ErrLog.Printf("Failed to get provider variables from DB: %s", err)
			continue
		}

		ipStr := string(ipAddr)

		if existingProvider, ok := providers_map[provider.ID]; ok {
			existingProvider.IP.IPs = append(existingProvider.IP.IPs, ipStr)
		} else {
			provider.IP.IPs = []string{ipStr}
			providers_map[provider.ID] = &provider
		}
	}
	var err error
	if err = providers_row.Err(); err != nil {
		ErrLog.Printf("Error during row iteration: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Error during row iteration", "error": err.Error()})
		return
	}

	providers_slice := make([]model.Providers, 0, len(providers_map))
	for _, provider := range providers_map {
		providers_slice = append(providers_slice, *provider)
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   providers_slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Get provider godoc
// @Summary      Delete provider
// @Description  Delete provider by ID
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Provider ID"
// @Success      200  {array}   model.ProviderDelete
// @Router       /providers/{id} [delete]
// @Security ApiKeyAuth
func DeleteProviderByID(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	id := c.Param("id")

	// Начинаем транзакцию
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to begin transaction", "error": err.Error()})
		return
	}

	// Удаляем записи из таблицы providers_address
	_, err = tx.Exec("DELETE FROM billing.providers_address WHERE pid = $1", &id)
	if err != nil {
		tx.Rollback() // Откатываем транзакцию при ошибке
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete address", "error": err.Error()})
		return
	}

	// Удаляем записи из таблицы providers
	_, err = tx.Exec("DELETE FROM billing.providers WHERE pid = $1", &id)
	if err != nil {
		tx.Rollback() // Откатываем транзакцию при ошибке
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete provider", "error": err.Error()})
		return
	}

	// Удаляем записи из таблицы routes
	_, err = tx.Exec("DELETE FROM billing.routes WHERE pid = $1", &id)
	if err != nil {
		tx.Rollback() // Откатываем транзакцию при ошибке
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete provider", "error": err.Error()})
		return
	}

	// Подтверждаем транзакцию
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to commit transaction", "error": err.Error()})
		return
	}

	// Успешный ответ
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Deleted successffuly"})
}

// Get provider godoc
// @Summary      Delete address provider
// @Description  Delete provider address
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Provider ID"
// @Param ip body model.ProvidersAddressAdd true "Address"
// @Success      200  {array}   model.DeleteAddressReply
// @Router       /providers/{id}/address [delete]
// @Security ApiKeyAuth
func DeleteAddressByAddress(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	id := c.Param("id")
	var address model.ProvidersAddressAdd

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var address_exists bool

	err = db.Get(&address_exists, "SELECT EXISTS(SELECT 1 FROM billing.providers_address WHERE pid = $1 AND ip = $2)", &id, &address.IP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check address existence", "error": err.Error()})
		return
	}

	if !address_exists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Address not found"})
		return
	}

	// Начинаем транзакцию
	tx, err := db.Beginx()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to begin transaction", "error": err.Error()})
		return
	}

	// Удаляем запись из таблицы providers_address
	_, err = tx.Exec("DELETE FROM billing.providers_address WHERE pid = $1 AND ip = $2", &id, &address.IP)
	if err != nil {
		tx.Rollback() // Откатываем транзакцию при ошибке
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete address", "error": err.Error()})
		return
	}

	// Подтверждаем транзакцию
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to commit transaction", "error": err.Error()})
		return
	}

	response := model.DeleteAddressReply{
		ID:      id,
		Address: address.IP,
		Status:  "success",
		Message: "Deleted successfully",
	}

	// Успешный ответ
	c.JSON(http.StatusOK, response)
}

// Get provider godoc
// @Summary      Add provider
// @Description  Create new provider
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param name body model.ProvidernoID true "Provider"
// @Success      200  {array}   model.AddProviderReply
// @Router       /providers/add [post]
// @Security ApiKeyAuth
func AddProvider(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	var provider model.ProvidernoID
	if err := c.BindJSON(&provider); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid input", "error": err.Error()})
		return
	}

	if provider.Description == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All fields must be filled"})
		return
	}

	var defaultMethod int
	if provider.Method == nil {
		defaultMethod = 1
		provider.Method = &defaultMethod
	}

	// Вставляем данные провайдера
	_, err = db.NamedExec("INSERT INTO billing.providers (name, method,description) VALUES (:name, :method, :description)", &provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert provider", "error": err.Error()})
		return
	}

	// Если есть вставляем IP-адреса
	if provider.IP.Address != nil {
		var prov model.ProvidersOnly
		err = db.Get(&prov, "SELECT pid FROM billing.providers WHERE name = $1", &provider.Name)
		if err != nil {
			ErrLog.Printf("Failed to fetch provider: %s", err)
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch provider", "error": err.Error()})
			return
		}

		for _, address := range provider.IP.Address {
			if address != "" {
				_, err = db.Exec("INSERT INTO billing.providers_address (pid, ip) VALUES ($1, $2)", &prov.PID, address)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new address", "error": err.Error()})
					return
				}
			}
		}
	}

	response := model.AddProviderReply{
		Name:    provider.Name,
		Status:  "success",
		Message: "New provider added successfully",
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Get provider godoc
// @Summary      Add provider address
// @Description  Add new IP to provider
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Provider ID"
// @Param ip body model.ProvidersAddressAdd true "Provider"
// @Success      200  {array}   model.JsonResponse
// @Router       /providers/{id}/add [post]
// @Security ApiKeyAuth
func AddProviderIP(c *gin.Context) {
	db, err := PGConnect()
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	var ipaddr model.ProvidersAddressAdd
	if err := c.BindJSON(&ipaddr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid input", "error": err.Error()})
		return
	}

	id := c.Param("id")
	// Используя сканирование структуры, вставляем его в таблицу продуктов
	_, err = db.Exec("INSERT INTO billing.providers_address (pid, ip) VALUES ($1, $2)", &id, &ipaddr.IP)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert provider", "error": err.Error()})
		return
	}
	response := model.JsonResponse{
		Status: "success",
		Data:   ipaddr,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Edit provider godoc
// @Summary      Edt provider
// @Description  Edit provider params
// @Tags         Providers
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Provider ID"
// @Param ip body model.ProviderEdit true "Provider"
// @Success      200  {array}   model.EditProviderReply
// @Router       /providers/{id}/edit [put]
// @Security ApiKeyAuth
func EditProvider(c *gin.Context) {
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
	var requestData model.ProviderEdit

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	if requestData.Name == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Provider name can't be empty"})
		return
	}

	// Обновление в таблице billing.providers
	if requestData.Description != nil {
		_, err = db.Exec(`UPDATE billing.providers SET name = $1, method = $2, description = $3 WHERE pid = $4`, &requestData.Name, &requestData.Method, &requestData.Description, &id)
	} else {
		_, err = db.Exec(`UPDATE billing.providers SET name = $1, method = $2 WHERE pid = $3`, &requestData.Name, &requestData.Method, &id)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update provider", "error": err.Error()})
		return
	}

	// Обновление адресов в таблице billing.providers_address
	// Удаляем старые адреса для данного провайдера
	_, err = db.Exec(`DELETE FROM billing.providers_address WHERE pid = $1`, &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete olded addresses", "error": err.Error()})
		return
	}

	// Вставляем новые адреса
	for _, address := range requestData.IPs.Address {
		if address != "" {
			_, err = db.Exec(`INSERT INTO billing.providers_address (pid, ip) VALUES ($1, $2)`, &id, &address)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new address", "error": err.Error()})
				return
			}
		}
	}

	response := model.EditProviderReply{
		ID:      id,
		Status:  "success",
		Message: "Provider updated successfully",
	}

	// Отправляем JSON-ответ
	c.JSON(http.StatusOK, response)
}
