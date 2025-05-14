package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	"vc-api/model"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func APIFetch(method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Если переданы данные, кодируем их в JSON
	if jsonData != nil {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("Failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)

	}
	//fmt.Printf("Request Body: %s\n", reqBody)
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}
	//fmt.Println(req)

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.API_Webitel.Header, config.API_Webitel.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to read response body: %w", err)
	}

	// Возвращаем body ответа
	return body, resp.StatusCode, nil
}

func CheckActualSub(db *sqlx.DB) (err error) {

	var groupIDs []int
	err = db.Select(&groupIDs, "SELECT group_id FROM vc.sub GROUP BY group_id")
	if err != nil {
		return fmt.Errorf("Failed to fetch group IDs: %w", err)
	}
	// Обнуляем актуальность у всех перед обновлением
	_, err = db.Exec("UPDATE vc.sub SET actual = FALSE, reserve = FALSE, reserve_resource_id = NULL")
	if err != nil {
		return fmt.Errorf("Failed to clear actual: %w", err)
	}

	for _, gid := range groupIDs {
		// Создаем новый запрос
		url := fmt.Sprintf("%s/call_center/resource_group/%d/resource", config.API_Webitel.URL, gid)

		// Читаем тело ответа
		body, statusCode, err := APIFetch("GET", url, "")
		if err != nil {
			return fmt.Errorf("Failed to read response body: %w, status code: ", err, statusCode)
		}

		// Парсим JSON-ответ
		var response model.WebitelResponse
		err = json.Unmarshal(body, &response)
		if err != nil {
			return fmt.Errorf("Failed to parse JSON response: %w", err)
		}

		// Обрабатываем элементы ответа
		if len(response.Items) == 0 {
			return fmt.Errorf("No items found in JSON response for group_id %d", gid)
		}

		for _, item := range response.Items {

			// Загружаем актуальные ресурсы из Webitel

			// Проверяем наличие резервного ресурса
			reserve_exists := false
			if item.ReserveResource.ID != "" {
				// Если есть резервный ресурс, обновляем 2 строки в одном запросе
				// Сначала в актуальном resource_id прописываем значение reserve_resource_id, потом в резервном resource_id указываем, что он является резервным
				updateQuery := `WITH updated_actual AS (
						UPDATE vc.sub 
						SET actual = TRUE, reserve_resource_id = $1, priority = $4  
						WHERE resource_id = $2 AND group_id = $3
					)
					UPDATE vc.sub 
					SET reserve = TRUE, priority = $4 
					WHERE resource_id = $1 AND group_id = $3`
				params := []interface{}{item.ReserveResource.ID, item.Resource.ID, gid, item.Priority}

				_, err := db.Exec(updateQuery, params...)
				if err != nil {
					return fmt.Errorf("Failed to update reserve_resource_id %s for group_id %d: %w", item.ReserveResource.ID, gid, err)
				}
				reserve_exists = true
			}

			// Проверяем наличие основного ресурса
			if item.Resource.ID != "" && !reserve_exists {
				// Если есть приоритет, обновляем его вместе с актуальностью
				if item.Priority > 0 {
					updateQuery := "UPDATE vc.sub SET actual = TRUE, priority = $1 WHERE resource_id = $2 AND group_id = $3"
					params := []interface{}{item.Priority, item.Resource.ID, gid}

					_, err := db.Exec(updateQuery, params...)
					if err != nil {
						return fmt.Errorf("Failed to update resource_id %s for group_id %d: %w", item.Resource.ID, gid, err)
					}
				} else {
					// Если нет приоритета, просто обновляем как актуальный
					updateQuery := "UPDATE vc.sub SET actual = TRUE WHERE resource_id = $1 AND group_id = $2"
					params := []interface{}{item.Resource.ID, gid}

					_, err := db.Exec(updateQuery, params...)
					if err != nil {
						return fmt.Errorf("Failed to update resource_id %s for group_id %d: %w", item.Resource.ID, gid, err)
					}
				}
			}

			if err != nil {
				return fmt.Errorf("Failed to update resource_id %s for group_id %d: %w", item.Resource.ID, gid, err)
			}
		}
	}

	return nil
}

// Groups list godoc
// @Summary      List subdivision
// @Description  Get a list of all routes with pagination
// @Tags         Team
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.Sub
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of routes per page" default(100)
// @Router       /list [get]
// @Security ApiKeyAuth
func GetList(c *gin.Context) {
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	func_err := CheckActualSub(db)
	if func_err != nil {
		ErrLog.Printf("Failed to check actual resource: %s", func_err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check actual resource", "error": func_err.Error()})
		return
	}

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

	var slice []model.Sub

	sub_query := `SELECT 
		s.id,
		s.group_name,
		s.group_id,
		analizator.name AS analizator,
		vendor.name AS vendor,
		s.actual,
		s.priority,
		s.change_at,
		s.change_login,
		reserve_analizator.name AS reserve_analizator,
		reserve_vendor.name AS reserve_vendor
		FROM 
		vc.sub s
		LEFT JOIN 
		vc.vendors analizator ON s.analizator_id = analizator.id
		LEFT JOIN 
		vc.vendors vendor ON s.vendor_id = vendor.id
		LEFT JOIN 
		vc.sub reserve_sub ON s.reserve_resource_id = reserve_sub.resource_id
		LEFT JOIN 
		vc.vendors reserve_analizator ON reserve_sub.analizator_id = reserve_analizator.id
		LEFT JOIN 
		vc.vendors reserve_vendor ON reserve_sub.vendor_id = reserve_vendor.id
		ORDER BY 
		s.group_name 
		LIMIT $1 OFFSET $2;`

	err = db.Select(&slice, sub_query, limit, offset)
	if err != nil {
		ErrLog.Printf("Failed to fetch routes: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch data", "error": err.Error()})
		return
	}

	// Заполнение ReserveResource
	for i := range slice {
		if slice[i].ReserveAnalizator != nil && slice[i].ReserveVendor != nil {
			slice[i].ReserveResource = model.SumReserveResource{
				Analizator: *slice[i].ReserveAnalizator,
				Vendor:     *slice[i].ReserveVendor,
			}
		}
	}

	// Проверяем, есть ли данные на текущей странице
	if len(slice) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}

func CheckResouceReserve(db *sqlx.DB, groupID int) (reserveCount int, err error) {
	err = db.Get(&reserveCount, "SELECT COUNT(id) FROM vc.sub WHERE group_id=$1 AND actual=$2 AND reserve_resorce_id IS NOT NULL", groupID, true)
	if err != nil {
		return 0, nil
	}
	return reserveCount, nil
}

// Change vendor godoc
// @Summary      Change vendor
// @Description  Change vendor name
// @Tags         Team
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Group ID"
// @Param data body model.RequestEditVendor true "Provider"
// @Success      200  {array}   model.Response
// @Router       /edit/{id} [patch]
// @Security ApiKeyAuth
func ChangeVendor(c *gin.Context) {
	// Подключение к базе данных
	db, err := PGConnect("")
	if err != nil {
		ErrLog.Fatalf("Failed to connect to PostgreSQL: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Database connection error", "error": err.Error()})
		return
	}
	defer db.Close()

	// Получение ID из URL
	GroupID := c.Param("id")

	// Структура для хранения данных из запроса
	var request model.RequestEditVendor

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	if GroupID != "" {
		// Пытаемся преобразовать строку в целое число чтобы проверить что group_id это число
		if _, err := strconv.Atoi(GroupID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Group ID must be a number"})
			return
		}
		if len(request.Resources) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Resources must not be empty"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Group ID is empty"})
		return
	}

	// Словарь для хранения соответствий по GroupID
	resourceMappings := make(map[string][]model.ResourceMapping)

	// Получаем все items из API для запрашиваемой группы
	url := fmt.Sprintf("%s/call_center/resource_group/%s/resource", config.API_Webitel.URL, string(GroupID))
	body, statusCode, err := APIFetch("GET", url, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to read response from Webitel API", "code": statusCode, "error": err.Error()})
		return
	}

	// Парсим JSON-ответ
	var response model.WebitelResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to parse JSON response from Webitel API", "error": err.Error()})
		return
	}

	// Проверка на совпадение длины ресурсов и элементов
	if len(request.Resources) != len(response.Items) {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Count of resources in the request and Webitel do not match"})
		return
	}

	// Перебираем каждый ресурс из JSON-запроса и сопоставляем по индексу
	for i, resource := range request.Resources {

		// Если resource_id и reserve_resource_id равны то отбиваем такой запрос
		if resource.Analizator == resource.Reserve.Analizator && resource.Vendor == resource.Reserve.Vendor {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Vendor and reserve must not match"})
			return
		}

		// Преобразуем GroupID в int
		GID, err := strconv.Atoi(GroupID)
		resourceIDCount, err := CheckResouceReserve(db, GID)

		if resourceIDCount > 0 {
			if resource.Reserve.Vendor == "" {
				c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Count of reserve resources in the request and actual do not match"})
				return
			}
		}

		// Узнаём ResourceID для текущего ресурса
		var DBResourceID int
		err = db.Get(&DBResourceID, `SELECT s.resource_id 
			FROM vc.sub s
			JOIN vc.vendors a ON s.analizator_id = a.id AND a.name = $2
			JOIN vc.vendors v ON s.vendor_id = v.id AND v.name = $3
			WHERE s.group_id = $1;`,
			GroupID, resource.Analizator, resource.Vendor)

		// Проверка на наличие ошибки или отсутствие ResourceID
		if err != nil || DBResourceID == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get resource_id by name from DB", "error": err.Error()})
			return
		}

		DBReserveResourceID := 0
		if request.Resources[i].Reserve.Analizator != "" && request.Resources[i].Reserve.Vendor != "" {
			// Узнаём ReserveResourceID для текущего ресурса
			err = db.Get(&DBReserveResourceID, `SELECT s.resource_id 
				FROM vc.sub s
				JOIN vc.vendors a ON s.analizator_id = a.id AND a.name = $2
				JOIN vc.vendors v ON s.vendor_id = v.id AND v.name = $3
				WHERE s.group_id = $1;`,
				GroupID, request.Resources[i].Reserve.Analizator, request.Resources[i].Reserve.Vendor)

			// Проверка на наличие ошибки или отсутствие ResourceID
			if err != nil || DBReserveResourceID == 0 {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get reserve_resource_id by name from DB", "error": err.Error()})
				return
			}
		}

		// Устанавливаем приоритет: если он не указан (нулевой), присваиваем его автоматически
		priority := resource.Priority
		if priority == 0 {
			priority = i + 1 // Автоматическое присвоение приоритета (1 для первого элемента и т.д.)
		}

		var rr_id int
		if DBReserveResourceID > 0 {
			rr_id = DBReserveResourceID
		}

		// Сохраняем соответствие в словаре по GroupID
		resourceMappings[GroupID] = append(resourceMappings[GroupID], model.ResourceMapping{
			ItemID:            response.Items[i].Id,
			ResourceID:        DBResourceID,
			Priority:          priority,
			ReserveResourceID: rr_id,
		})
	}

	// Новый массив для хранения соответствий нужен для доп. проверки чтобы в одной группе не было значения ResourceID такого же как в другом ItemsID
	newMappings := make(map[string][]model.ResourceMapping)

	for map_idx, resupdate := range resourceMappings[GroupID] {
		exists := false
		for api_idx, apires := range response.Items {
			if api_idx != map_idx {
				APIresourceID, _ := strconv.Atoi(apires.Resource.ID)
				if resupdate.ResourceID == APIresourceID {
					// Меняем индекс элемента resourceMappings[GroupID][map_idx] на индекс из response.Items[api_idx] и добавляем в новый срез
					newMappings[GroupID] = append(newMappings[GroupID], model.ResourceMapping{
						ItemID:            resourceMappings[GroupID][api_idx].ItemID,
						ResourceID:        resupdate.ResourceID,
						Priority:          resupdate.Priority,
						ReserveResourceID: resupdate.ReserveResourceID,
					})
					exists = true
				}
			}
		}
		if !exists {
			newMappings[GroupID] = append(newMappings[GroupID], model.ResourceMapping{
				ItemID:            resourceMappings[GroupID][map_idx].ItemID,
				ResourceID:        resupdate.ResourceID,
				Priority:          resupdate.Priority,
				ReserveResourceID: resupdate.ReserveResourceID,
			})
		}
	}

	// Обнуляем актуальность у группы перед обновлением
	_, err = db.Exec("UPDATE vc.sub SET actual = FALSE, reserve = FALSE, priority = NULL WHERE group_id = $1", GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to clear group_id: " + GroupID, "error": err.Error()})
		return
	}

	// Если есть данные для обновления, выполняем PUT-запросы
	for _, update := range newMappings[GroupID] {
		updateURL := fmt.Sprintf("%s/call_center/resource_group/%s/resource/%s", config.API_Webitel.URL, string(GroupID), update.ItemID)

		// Формируем тело запроса обновления в Webitel с новым resource ID и правильным приоритетом
		requestBody := map[string]interface{}{
			"resource": map[string]int{
				"id": update.ResourceID,
			},
			"priority": update.Priority,
		}

		// Если есть reserve_resource то добавляем к запросу
		if update.ReserveResourceID != 0 {
			requestBody["reserve_resource"] = map[string]int{
				"id": update.ReserveResourceID,
			}
		}
		ResourceID := update.ResourceID

		// Отправляем PUT-запрос
		_, statusCode, err := APIFetch("PUT", updateURL, requestBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": fmt.Sprintf("Failed to update resource with ID %s", update.ItemID), "error": err.Error()})
			return
		}

		// Проверяем статус-код
		if statusCode < 200 || statusCode >= 300 {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Response status from Webitel API is not OK", "code": statusCode})
			return
		}

		// Получение текущего времени
		currentTime := time.Now()
		// Получение текущего логина из JWT
		currentLogin, _ := c.Get("email")

		_, err = db.Exec("UPDATE vc.sub SET change_at = $1, change_login = $2 WHERE group_id = $3 AND resource_id = $4", currentTime, currentLogin, GroupID, ResourceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update changer info", "error": err.Error()})
			return
		}
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully edited"})
}
