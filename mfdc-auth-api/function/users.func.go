package function

import (
	"auth-api/model"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx"
	"github.com/jmoiron/sqlx"
)

// Users add godoc
// @Summary      Add users
// @Description  Add new user
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.UserEdit
// @Param route body model.UserEdit true "User"
// @Router       /users/add [post]
// @Security ApiKeyAuth
func UserAdd(db *sqlx.DB, c *gin.Context) {
	var data model.User

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	if data.Email == "" || data.Password == "" || data.Role == "" || data.TeamID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Fields email, password, role, team_id is required"})
		return
	}

	// Шифруем plain-text пароль в SH-256
	pwd_hash := TextToSH256(data.Password)
	data.Password = pwd_hash

	// Преобразуем срез строк в массив PostgreSQL
	sectionsArray := pgtype.TextArray{}
	if len(data.Sections) > 0 {
		sectionsArray.Set(data.Sections)
	} else {
		// Если data.Sections не установлено, sectionsArray останется пустым
		sectionsArray.Set([]string{})
	}

	_, err := db.Exec("INSERT INTO auth.users (firstname, lastname, email, password, role, team_id, sections, enabled, token_version) VALUES ($1, $2, $3, $4, $5, $6, $7, true, 1)",
		data.Firstname, data.Lastname, data.Email, data.Password, data.Role, data.TeamID, sectionsArray)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new user", "error": err.Error()})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "User successfully added"})
}

// Users data godoc
// @Summary      Get user data
// @Description  Get user data from JWT-token
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.UserInfoSwagger
// @Router       /users/info [get]
// @Security ApiKeyAuth
func GetUserInfo(c *gin.Context) {
	uid, _ := c.Get("uid")
	email, existsEmail := c.Get("email")
	firstname, _ := c.Get("firstname")
	lastname, _ := c.Get("lastname")
	role, existsRole := c.Get("role")
	team_id, _ := c.Get("team_id")
	exp, existsExp := c.Get("exp")
	sections, _ := c.Get("sections")

	if !existsEmail || !existsRole || !existsExp {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Invalid access token"})
		return
	}

	response := gin.H{
		"email":     email,
		"firstname": firstname,
		"lastname":  lastname,
		"role":      role,
		"team_id":   team_id,
		"exp":       exp,
	}

	if uid != nil {
		response["uid"] = uid
	}

	// Добавляем sections в ответ только если оно не nil
	if sections != nil {
		response["sections"] = sections
	}

	c.JSON(http.StatusOK, response)
}

// Users edit godoc
// @Summary      Edit user
// @Description  Edit user items
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.UserEdit
// @Param        id   path      int  true  "User ID"
// @Param route body model.UserEdit true "User"
// @Router       /users/{id}/edit [patch]
// @Security ApiKeyAuth
func UserEdit(db *sqlx.DB, c *gin.Context) {
	// Получение ID пользователя из URL
	id := c.Param("id")

	// Структура для хранения данных из запроса
	var user model.UserEdit
	var current model.UserEditCurrent

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Получаем текущую роль аккаунта
	userRole, ok := c.Get("role")
	if !ok {
		ErrLog.Print("Field role is empty")
	} else {
		role, ok := userRole.(string)
		if !ok {
			ErrLog.Print("Field role is not string")
		} else {
			// Если роль не админ, то менять можно только пароль
			if role != "admin" {
				user.Firstname = nil
				user.Lastname = nil
				user.Email = nil
				user.Role = nil
				user.Enabled = nil
				user.TeamID = nil
				user.TokenVersion = nil
				user.PwdChangeAt = nil
				user.Sections = nil
			}
		}
	}

	// Получаем текущие данные пользователя из базы данных
	row := db.QueryRow("SELECT firstname, lastname, email, password, role, team_id, enabled, token_version, pwd_change_at, sections FROM auth.users WHERE uid = $1", id)

	//var section pgtype.TextArray
	err := row.Scan(
		&current.Firstname,
		&current.Lastname,
		&current.Email,
		&current.Password,
		&current.Role,
		&current.TeamID,
		&current.Enabled,
		&current.TokenVersion,
		&current.PwdChangeAt,
		&current.Sections,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Пользователь не найден
			c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "User not found"})
		} else {
			// Обработка других ошибок
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to select current user", "error": err.Error()})
		}
		return
	}

	// Создаём счётчик кол-ва редактируемых параметров
	item_counter := 0

	// Проверяем и устанавливаем значения
	if user.Firstname == nil || *user.Firstname == *current.Firstname {
		user.Firstname = current.Firstname
	} else {
		item_counter++
	}

	if user.Lastname == nil || *user.Lastname == *current.Lastname {
		user.Lastname = current.Lastname
	} else {
		item_counter++
	}

	if user.Email == nil || *user.Email == *current.Email {
		user.Email = current.Email
	} else {
		item_counter++
	}

	if user.Password == nil || TextToSH256(*user.Password) == *current.Password { // Если пароля нет в JSON или он такой же как и текущий
		user.Password = current.Password
		if user.PwdChangeAt == nil {
			user.PwdChangeAt = current.PwdChangeAt
		}
	} else {
		hashedPassword := TextToSH256(*user.Password)
		user.Password = &hashedPassword
		currentTime := time.Now()
		user.PwdChangeAt = &currentTime
		item_counter++
	}

	if user.Role == nil || *user.Role == *current.Role {
		user.Role = current.Role
	} else {
		item_counter++
	}

	if user.TeamID == nil || *user.TeamID == *current.TeamID {
		user.TeamID = current.TeamID
	} else {
		item_counter++
	}

	if user.Enabled == nil || *user.Enabled == *current.Enabled {
		user.Enabled = current.Enabled
	} else {
		item_counter++
	}

	// Основная логика
	var sections []string

	// Получаем sections из current
	sections = getSectionsFromCurrent(current.Sections)

	// Проверяем, если user.Sections равно nil
	if user.Sections == nil {
		// Если current.Sections также nil, инициализируем user.Sections как пустой срез
		if current.Sections == nil {
			user.Sections = &[]string{} // Указываем пустой срез
		} else {
			// Присваиваем указатель на срез строк из current.Sections
			user.Sections = &sections
		}
	} else {
		// Проверяем, что user.Sections не nil перед разыменованием
		if *user.Sections != nil {
			// Сравниваем срезы только если user.Sections не nil
			if !CompareStringSlices(*user.Sections, sections) {
				item_counter++
			}
		} else {
			// Если user.Sections - это nil, но у нас есть новые секции
			if len(sections) > 0 {
				item_counter++
			}
		}
	}

	// Преобразуем срез строк в массив pgtype.TextArray
	sectionsArray := pgtype.TextArray{}
	if user.Sections != nil && len(*user.Sections) > 0 {
		// Используем метод Set для установки значений
		sectionsArray.Set(*user.Sections)
	} else {
		// Устанавливаем статус как отсутствующий, если секции пустые
		sectionsArray.Status = pgtype.Null
	}

	// Если хоть один параметр был изменён, то меняем версию токена, что заставит пользователя повторно авторизоваться
	if item_counter > 0 || user.TokenVersion != nil {
		if user.TokenVersion == nil {
			*current.TokenVersion++
			user.TokenVersion = current.TokenVersion
		}

		_, err = db.Exec("UPDATE auth.users SET firstname = $1, lastname = $2, email = $3, password = $4, role = $5, team_id = $6, enabled = $7, token_version = $8, pwd_change_at = $9, sections = $10 WHERE uid = $11",
			user.Firstname, user.Lastname, user.Email, user.Password, user.Role, user.TeamID, user.Enabled, user.TokenVersion, user.PwdChangeAt, sectionsArray, id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update user", "error": err.Error()})
			return
		}
		/*
			// Сбрасываем кэш версии токена
			if user.Email != nil {
				clearCacheTokenVersion(*user.Email)
			}
		*/
		// Отправляем JSON-ответ
		c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "User successfully edited"})
	} else {
		user.TokenVersion = current.TokenVersion
		// Отправляем JSON-ответ
		c.IndentedJSON(http.StatusOK, gin.H{"status": "failed", "message": "The provided data is identical to the existing data. No changes were made."})
	}

}

// Delete user godoc
// @Summary      Delete user
// @Description  Drop user
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {array}   model.UserDeleteSwagger
// @Router       /users/{id}/delete [delete]
// @Security ApiKeyAuth
func UserDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")

	_, err := db.Exec("DELETE FROM auth.users WHERE uid = $1", &id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete user", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "User successfully deleted"})
}

// List teams godoc
// @Summary      List teams
// @Description  List teams
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.JsonResponse
// @Router       /teams/list [get]
// @Security ApiKeyAuth
func TeamsList(db *sqlx.DB, c *gin.Context) {
	var teams []model.Teams
	err := db.Select(&teams, "SELECT * FROM auth.teams ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get teams", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": teams})
}

// Users list godoc
// @Summary      List users
// @Description  Get a list of all users with pagination
// @Tags         Users
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.UsersListSwaggerResponse
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of routes per page" default(100)
// @Router       /users/list [get]
// @Security ApiKeyAuth
func UserList(db *sqlx.DB, c *gin.Context) {
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

	var users_slice []model.UsersList

	rows, err := db.Query("SELECT uid, firstname, lastname, email, role, team_id, enabled, pwd_change_at, sections FROM auth.users ORDER BY lastname LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch users", "error": err.Error()})
		return
	}
	defer rows.Close()

	for rows.Next() {
		var user model.UsersList

		// Сканируем данные в user
		err := rows.Scan(
			&user.Uid,
			&user.Firstname,
			&user.Lastname,
			&user.Email,
			&user.Role,
			&user.TeamID,
			&user.Enabled,
			&user.PwdChangeAt,
			&user.Sections, // Сканируем массив непосредственно в поле структуры
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to scan user", "error": err.Error()})
			return
		}

		// Преобразуем из user.Sections тип pgtype.TextArray в срез строк user.SectionsList []string
		if user.Sections.Status == pgtype.Present {
			for _, elem := range user.Sections.Elements {
				if elem.Status == pgtype.Present {
					user.SectionsList = append(user.SectionsList, elem.String)
				}
			}
		}

		users_slice = append(users_slice, user) // Добавляем пользователя в срез
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Error occurred while fetching users", "error": err.Error()})
		return
	}

	// Проверяем, есть ли данные на текущей странице
	if len(users_slice) == 0 {
		c.JSON(http.StatusOK, []string{})
		return
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   users_slice,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Get user godoc
// @Summary      Get user
// @Description  Get user
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "User ID"
// @Success      200  {array}   model.UserDeleteSwagger
// @Router       /users/{id}/info [get]
// @Security ApiKeyAuth
func GetUserNameByUID(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")

	var user model.UsersList
	err := db.Get(&user, "SELECT uid, firstname, lastname, email, role, team_id, enabled, pwd_change_at, sections FROM auth.users WHERE uid = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch user", "error": err.Error()})
		return
	}

	// Преобразуем из user.Sections тип pgtype.TextArray в срез строк user.SectionsList []string
	if user.Sections.Status == pgtype.Present {
		for _, elem := range user.Sections.Elements {
			if elem.Status == pgtype.Present {
				user.SectionsList = append(user.SectionsList, elem.String)
			}
		}
	}

	response := model.JsonResponse{
		Status: "success",
		Data:   user,
	}

	c.IndentedJSON(http.StatusOK, response)

}
