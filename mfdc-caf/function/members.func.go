package function

import (
	"caf/model"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func checkMemberInBlock(db *sqlx.DB, number string) (blocked bool, err error) {
	// Проверяем в ЧС
	err = db.Get(&blocked, "SELECT EXISTS (SELECT 1 FROM caf.blacklist WHERE number = $1)", number)
	if err != nil {
		// Возвращаем ошибку, а не просто выводим ее
		return false, fmt.Errorf("%w", err)
	}

	if !blocked { // Если в ЧС не нашли, проверяем блокировку в номерах
		query := "SELECT EXISTS (SELECT 1 FROM caf.numbers WHERE number = $1 AND blocked = $2 AND repeated_check = $3)"

		err = db.Get(&blocked, query, number, true, false)
		if err != nil {
			// Возвращаем ошибку, а не просто выводим ее
			return false, fmt.Errorf("%w", err)
		}
	}

	return blocked, nil
}

func addMemberToDB(db *sqlx.DB, queueID string, teamID int, memberID string, number string, clientID *string) error {
	// Обновляем запись
	updateRes, err := db.Exec("UPDATE caf.numbers SET queue_id = $1, team_id = $2, member_id = $3, last_load_at = $4, load_counter = load_counter + 1, client_id = $5 WHERE number = $6", queueID, teamID, memberID, time.Now(), clientID, number)
	if err != nil {
		return fmt.Errorf("failed to update number: %w", err) // Возвращаем ошибку
	}

	// Получаем количество обновленных строк
	updateAffected, err := updateRes.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get number update count: %w", err) // Возвращаем ошибку
	}

	// Если ничего не обновили, значит номера нет в БД, тогда добавляем его
	if updateAffected == 0 {
		_, err = db.Exec("INSERT INTO caf.numbers (load_counter, first_load_at, last_load_at, queue_id, team_id, member_id, number, client_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)", 1, time.Now(), time.Now(), queueID, teamID, memberID, number, clientID)
		if err != nil {
			return fmt.Errorf("failed to insert new number: %w", err) // Возвращаем ошибку
		}
	}

	return nil // Возвращаем nil, если всё прошло успешно
}

func getTeamIDByQueueID(db *sqlx.DB, queueID string) (int, bool, error) {
	queueIDInt, err := strconv.Atoi(queueID)
	if err != nil {
		return 0, false, fmt.Errorf("failed to convert queue ID string to int: %w", err)
	}

	// Получаем срез команд
	var teamsDB []model.TeamDB
	err = db.Select(&teamsDB, "SELECT * FROM caf.teams")
	if err != nil {
		return 0, false, fmt.Errorf("failed to get teams: %w", err)
	}

	// Создаем срез команд
	teams := make([]model.Team, len(teamsDB))
	for idx, team := range teamsDB {
		teams[idx].ID = team.ID
		teams[idx].Filtration = team.Filtration
		if team.WebitelQueuesIDS != nil {
			if teams[idx].WebitelQueuesIDS == nil {
				teams[idx].WebitelQueuesIDS = new([]int)
			}
			*teams[idx].WebitelQueuesIDS = PgIntArr2IntArr(*team.WebitelQueuesIDS)
		}
	}

	// Перебираем срез команд и ищем соответствие
	for _, team := range teams {
		if team.WebitelQueuesIDS != nil {
			for _, queue := range *team.WebitelQueuesIDS {
				if queue == queueIDInt {
					Filtration := false
					if team.Filtration != nil {
						Filtration = *team.Filtration
					}
					if team.ID == nil {
						return 0, false, fmt.Errorf("team ID is nil") // Возвращаем ошибку, если ID не существует
					}
					return *team.ID, Filtration, nil // Возвращаем ID команды и режим фильтрации
				}
			}
		}
	}

	// Если команда не найдена
	return 0, false, fmt.Errorf("no team found for queue ID: %s", queueID)
}

func checkNumberSuccessToday(db *sqlx.DB, number string, recall *bool) (success bool, err error) {
	if recall != nil && *recall {
		_, err := db.Exec("UPDATE caf.numbers SET today_success_call = $1 WHERE number = $2", false, number)
		if err != nil {
			return false, fmt.Errorf("failed to disable mark today call: %w", err)
		}
		return false, nil
	}

	// Проверяем наличие номера с today_success_call = true
	err = db.Get(&success, "SELECT EXISTS (SELECT 1 FROM caf.numbers WHERE today_success_call = $1 AND number = $2)", true, number)
	if err != nil {
		return false, fmt.Errorf("failed to check number exists: %w", err)
	}
	return success, nil
}

func checkNumberByClientID(db *sqlx.DB, number string, clientID *string) (success bool, err error) {
	// Проверяем, что clientID не nil
	if clientID != nil {
		// Проверяем наличие входящего client_id у другого номера
		err = db.Get(&success,
			`SELECT EXISTS (
                SELECT 1 
                FROM caf.numbers 
                WHERE client_id = $1 AND number != $2 AND DATE(last_load_at) = CURRENT_DATE
            )`, *clientID, number)
		if err != nil {
			return false, fmt.Errorf("failed to check if number exists: %w", err)
		}

		// Если client_id существует в другом номере, проверяем успешные вызовы по нему за сегодня
		if success {
			err = db.Get(&success,
				`SELECT EXISTS (
                    SELECT 1 
                    FROM caf.numbers 
                    WHERE client_id = $1 AND today_success_call = $2
                )`, *clientID, true)
			if err != nil {
				return false, fmt.Errorf("failed to check for successful calls: %w", err)
			}
		}
		return success, nil
	}

	// Если clientID равен nil, возвращаем false и nil для ошибки
	return false, nil
}

func checkNumberSuccessWeek(db *sqlx.DB, number string) (bool, error) {
	var success bool

	// Выполняем запрос для проверки существования номера с успешными вызовами за 7 дней
	query :=
		`SELECT EXISTS (
				SELECT 1 
				FROM caf.numbers 
				WHERE first_success_call_at IS NOT NULL 
				AND second_success_call_at IS NOT NULL 
				AND number = $1
				AND (CURRENT_TIMESTAMP - first_success_call_at) < INTERVAL '7 days'
			)`

	// Выполняем запрос и проверяем наличие ошибок
	err := db.Get(&success, query, number)
	if err != nil {
		return false, fmt.Errorf("failed to check if number exists: %w", err)
	}

	return success, nil
}

func checkNumberDuoble(db *sqlx.DB, number string, QueueID string) (bool, error) {
	var memberID string

	// Проверяем, что дата последней отправки номера была сегодня и получаем member_id
	query := `SELECT member_id FROM caf.numbers WHERE DATE(last_load_at) = CURRENT_DATE AND number = $1`
	err := db.Get(&memberID, query, number)

	//OutLog.Printf("number: %s,  memberid: %s, err: %s", number, memberID, err)

	if err != nil {
		// Если запись не найдена, возвращаем false
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check today load number exists: %w", err)
	}

	// Если сегодня номер уже был загружен, проверяем есть ли он на текущий момент в Webitel
	url := fmt.Sprintf("%s/call_center/queues/%s/members/%s", config.API_Webitel.URL, QueueID, memberID)
	_, statusCode, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "GET", url, nil)

	if err != nil {
		return false, fmt.Errorf("failed to send request to Webitel: %w", err)
	}

	if statusCode < 200 || statusCode >= 300 {
		return false, nil
	} else {
		return true, nil
	}

	/*
		var responseMember model.Members
		err = json.Unmarshal(responseBody, &responseMember)

		if err != nil {
			return false, fmt.Errorf("failed to unmarshal response from Webitel: %w", err)
		}

		// Если в ответе нет code, значит Webitel ответил без ошибки и мембер там уже есть
		return responseMember.Code == nil, nil
	*/
}

func addLog(db *sqlx.DB, teamID int, number string, description string, filtered bool) error {
	var numID int64
	err := db.Get(&numID, "SELECT id FROM caf.numbers WHERE number = $1", number)
	if err != nil {
		return fmt.Errorf("failed to get number ID: %w", err)
	}

	_, err = db.Exec("INSERT INTO caf.logs (created_at, team_id, num_id, number, description, filtered) VALUES ($1, $2, $3, $4, $5, $6)", time.Now(), teamID, numID, number, description, filtered)
	if err != nil {
		return fmt.Errorf("failed to add log for number %s: %w", number, err)
	}
	return nil
}

func checkMember(db *sqlx.DB, queueID string, number string, recall *bool, clientID *string) (string, bool, error) {
	// Проверяем заблокирован ли номер
	numberBlocked, err := checkMemberInBlock(db, number)
	if err != nil {
		return "", false, fmt.Errorf("failed to check number %s for blocking: %w", number, err)
	}
	if numberBlocked {
		return "Номер в чёрном списке", true, nil
	}

	// Проверяем, что такого жу client_id нет у другого номера в БД загруженного сегодня
	clientIDAlready, err := checkNumberByClientID(db, number, clientID)
	if err != nil {
		return "", false, fmt.Errorf("failed to check client_id for number %s: %w", number, err)
	}
	if clientIDAlready {
		return "Такой client_id сегодня уже был загружен с другим номером и по нему был успешный дозвон", true, nil
	}

	// Проверяем, что сегодня успешных звонков не было
	todaySuccess, err := checkNumberSuccessToday(db, number, recall)
	if err != nil {
		return "", false, fmt.Errorf("failed to check today success call for number %s: %w", number, err)
	}
	if todaySuccess {
		return "Сегодня уже был успешный вызов на этот номер", true, nil
	}

	// Проверяем, что за последние 7 дней на номер уже было 2 успешных звонка
	weekSuccess, err := checkNumberSuccessWeek(db, number)
	if err != nil {
		return "", false, fmt.Errorf("failed to check week success call for number %s: %w", number, err)
	}
	if weekSuccess {
		return "На этот номер уже было 2 успешных звонка за последние 7 дней", true, nil
	}

	// Проверяем дату последней загрузки и если она сегодняшняя,
	// то проверяем есть ли member_id из предыдущей загрузки в очереди Webitel
	duoble, err := checkNumberDuoble(db, number, queueID)
	if err != nil {
		return "", false, fmt.Errorf("failed to check double for number %s: %w", number, err)
	}
	if duoble {
		return "Сегодня номер уже был добавлен", true, nil
	}

	return "", false, nil
}

// Add members godoc
// @Summary      Add members
// @Description  Add members
// @Tags         Members
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Queue ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /{id}/members [post]
// @Security ApiKeyAuth
func ReceiveMembers(db *sqlx.DB, c *gin.Context) {
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var members model.Members

	memberName := "Unknown" // Значение по умолчанию

	// Чтение данных из тела запроса
	err := c.ShouldBindJSON(&members)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"id":     "mfdc.caf.api.request.error",
			"status": "Bad Request",
			"code":   400,
			"detail": "Failed to parse JSON for number: " + memberName,
		})
		return
	}

	if members.Communications == nil {
		c.JSON(http.StatusBadRequest, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Request", "code": 400, "detail": "Param Communications not found"})
		return
	}

	// Получаем номер, если он присутствует
	if members.Name != nil {
		memberName = *members.Name
	} else {
		if members.Communications != nil && len(*members.Communications) > 0 {
			communications := *members.Communications
			if communications[0].Destination != nil {
				memberName = *communications[0].Destination
			}
		}
	}

	url := fmt.Sprintf("%s/call_center/queues/%s/members", config.API_Webitel.URL, id)

	Variables := map[string]string{}
	if members.Variables != nil {
		Variables = *members.Variables
	}

	// Веб интерфейс Webitel по умолчанию не присылает TimeZoneID
	timeZone := map[string]interface{}{}
	if members.Timezone != nil && members.Timezone.ID != nil {
		timeZone = map[string]interface{}{
			"id": *members.Timezone.ID,
		}
	}

	body := map[string]interface{}{
		"communications": []map[string]interface{}{
			{
				"destination": (*members.Communications)[0].Destination,
				"type": map[string]interface{}{
					"id": (*members.Communications)[0].Type.ID,
				},
			},
		},
		"name":            memberName,
		"variables":       Variables,
		"priority":        members.Priority,
		"min_offering_at": members.MinOfferingAt,
		"timezone":        timeZone,
	}

	// Проверяем требуется ли повторная загрузка мембера
	var reCall *bool

	// Инициализация переменной reCall
	if recallValue, exists := Variables["recall"]; exists {
		// Если значение "recall" равно "yes", создаем переменную и присваиваем ей true
		if recallValue == "yes" {
			trueValue := true
			reCall = &trueValue // Указываем на созданную переменную
		} else {
			falseValue := false
			reCall = &falseValue // Указываем на созданную переменную
		}
	} else {
		reCall = nil // Если ключ не существует, оставляем reCall равным nil
	}

	// Проверяем наличие переменной client_id
	var clientID *string
	if clientIDValue, existsCID := Variables["client_id"]; existsCID {
		// Если переменная существует, создаем указатель на её значение
		clientID = &clientIDValue
	}

	var filterMessage string
	isFiltered := false

	if reCall != nil && *reCall {
		// Если reCall yes, то фильтрация не требуется
		filterMessage = ""
	} else {
		// Выполняем проверку контакта
		filterMsg, filtered, err := checkMember(db, id, memberName, reCall, clientID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"id":     "mfdc.caf.api.request.error",
				"status": "Bad Gateway",
				"code":   500,
				"detail": "Failed check filter: " + memberName + " Info: " + err.Error(),
			})
			return
		}
		filterMessage = filterMsg
		isFiltered = filtered
	}

	// Получаем Team ID по Queue ID
	teamID, filterMode, err := getTeamIDByQueueID(db, id)
	if err != nil {
		ErrLog.Printf("Failed to get team ID: %s", err)
	}

	if filterMode {
		// Если мембер был отфитрован то не отправляем его в Webitel
		if isFiltered {
			// Записываем в лог событие почему был отфильтрован номер
			err := addLog(db, teamID, memberName, filterMessage, true)
			if err != nil {
				ErrLog.Printf("Failed save to log: %s", err)
			}
			c.JSON(http.StatusBadRequest, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Request", "code": 400, "detail": filterMessage})
			return
		}
	} else { // Записываем в лог для отправки списка отфильтрованных номеров
		if isFiltered {
			// Записываем в лог событие почему был отфильтрован номер
			err := addLog(db, teamID, memberName, filterMessage, false)
			if err != nil {
				ErrLog.Printf("Failed save to log: %s", err)
			}
		}
	}

	// Отправляем нового мембера в Webitel
	responseBody, _, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "POST", url, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Gateway", "code": 500, "detail": "Failed to send request to Webitel: " + memberName + " Info: " + err.Error()})
		return
	}

	//OutLog.Printf("\nResponse Body: %s", responseBody)

	var responseMember model.Members
	err = json.Unmarshal(responseBody, &responseMember)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Gateway", "code": 500, "detail": "Failed to unmarshal response from Webitel: " + memberName + " Info: " + err.Error()})
		return
	}

	// Если в ответе нет объекта code значит Webitel ответил без ошибки
	if responseMember.Code == nil {
		// Добавляем или обновляем мембера в базе
		if responseMember.ID != nil {
			err = addMemberToDB(db, id, teamID, *responseMember.ID, memberName, clientID)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Gateway", "code": 500, "detail": "Failed to add member to DB: " + memberName + " Info: " + err.Error()})
				return
			}
		} else {
			c.JSON(http.StatusBadGateway, gin.H{"id": "mfdc.caf.api.request.error", "status": "Bad Gateway", "code": 500, "detail": "Failed to receive memberID from Webitel for: " + memberName})
			return
		}
	}

	// Транзитно возвращаем JSON-ответ от Webitel
	c.Data(http.StatusOK, "application/json", responseBody)
}
