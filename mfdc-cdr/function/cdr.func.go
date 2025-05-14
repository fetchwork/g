package function

import (
	"cdr-api/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func API2DB(db *sqlx.DB) {
	var check_data model.MaxDate
	err := db.Get(&check_data, "SELECT COALESCE(MAX(created_at), NULL) AS max_created_at FROM cdr.calls")
	if err != nil {
		ErrLog.Printf("Failed to get max created_at: %s", err.Error())
		return
	}

	// Получаем текущее время в зоне
	loc, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		ErrLog.Printf("Failed to load location: %s", err.Error())
		return
	}
	currentTime := time.Now().In(loc)

	// Получаемые данные всегда должны отставать на 1 час от текущего времени
	threeHoursAgo := currentTime.Add(-1 * time.Hour)

	var from_date int64
	var to_date int64
	// Проверяем условие
	if check_data.MaxCreatedAt != nil {
		if check_data.MaxCreatedAt.After(threeHoursAgo) || check_data.MaxCreatedAt.Equal(threeHoursAgo) {
			// Если MaxCreatedAt не отстает на 1 час, выходим из функции
			OutLog.Println("Data will not be extracted because current date in DB does not lag behind by 1 hour")
			return
		}

		// Логика извлечения данных
		date := check_data.MaxCreatedAt.Add(1 * time.Millisecond) // Добавляем 1 миллисекунду к максимальной дате в БД
		from_date = ConvertToUnixMillis(&date)
		from_date_plus := check_data.MaxCreatedAt.Add(time.Duration(config.API_Webitel.FromHoursToNow) * time.Hour) // Продолжительность периода
		to_date = ConvertToUnixMillis(&from_date_plus)
	} else { // Если в БД нет данных
		if config.API_Webitel.StartDateIfDbEmpty != nil && config.API_Webitel.StopDateIfDbEmpty != nil {
			date := *config.API_Webitel.StartDateIfDbEmpty // Разыменовываем указатель
			from_date = ConvertToUnixMillis(&date)
			from_date_plus := *config.API_Webitel.StopDateIfDbEmpty // Разыменовываем указатель
			to_date = ConvertToUnixMillis(&from_date_plus)
		} else {
			// Создаем дату 1 января 2025 года
			date := time.Date(2025, 1, 1, 0, 0, 0, 0, loc)
			from_date = ConvertToUnixMillis(&date)

			// Прибавляем 11 месяцев
			from_date_plus := date.AddDate(0, 11, 0)
			to_date = ConvertToUnixMillis(&from_date_plus)
		}
	}

	// Получаем дынные
	url := fmt.Sprintf("%s/calls/history", config.API_Webitel.URL)

	// Формируем тело запроса к API
	requestBody := model.JSONRequest{
		Page: 1,
		Size: config.API_Webitel.QueryOffset,
		Sort: "created_at",
		Fields: []string{
			"files",
			"id",
			"parent_id",
			"agent",
			"queue",
			"team",
			"created_at",
			"answered_at",
			"direction",
			"hangup_phrase",
			"user",
			"from",
			"to",
			"destination",
			"duration",
			"bill_sec",
			"talk_sec",
			"hold_sec",
			"cause",
			"hangup_at",
			"sip_code",
			"hangup_by",
			"bridged_at",
			"has_children",
			"transfer_from",
			"transfer_to",
			"wait_sec",
		},
		CreatedAt: model.JSONRequestCreatedAt{
			From: from_date,
			To:   to_date,
		},
		SkipParent: false,
	}

	body, _, err := APIFetch("POST", url, requestBody)
	if err != nil {
		ErrLog.Printf("Failed to read response from Webitel API: %s", err.Error())
		return
	}

	// Парсим JSON-ответ
	var response model.JSONResponseCallsSlice
	err = json.Unmarshal([]byte(body), &response)
	if err != nil {
		ErrLog.Printf("Failed to parse JSON response from Webitel API: %s", err.Error())
		return
	}
	//OutLog.Printf("Calls: %v", response.Calls)
	// Перебираем объекты внутри массива
	for _, call := range response.Calls {

		// Инициализация структуры для каждой записи
		data := model.CDRDB{}

		// Присваиваем значения полям только если они не nil
		data.CreatedAt, _ = CheckJsonTimeVars(call.CreatedAt)
		data.AnsweredAt, _ = CheckJsonTimeVars(call.AnsweredAt)
		data.HangupAt, _ = CheckJsonTimeVars(call.HangupAt)
		data.BridgetAt, _ = CheckJsonTimeVars(call.BridgetAt)
		data.CallID = CheckJsonStringVars(call.ID)
		data.ParentID = CheckJsonStringVars(call.ParentID)
		data.Destination = CheckJsonStringVars(call.Destination)
		data.Direction = CheckJsonStringVars(call.Direction)
		data.HangupBy = CheckJsonStringVars(call.HangupBy)
		data.Cause = CheckJsonStringVars(call.Cause)
		data.TransferFrom = CheckJsonStringVars(call.TransferFrom)
		data.TransferTo = CheckJsonStringVars(call.TransferTo)
		data.Duration = CheckJsonIntVars(call.Duration)
		data.BillSec = CheckJsonIntVars(call.BillSec)
		data.TalkSec = CheckJsonIntVars(call.TalkSec)
		data.HoldSec = CheckJsonIntVars(call.HoldSec)
		data.SipCode = CheckJsonIntVars(call.SipCode)
		data.WaitSec = CheckJsonIntVars(call.WaitSec)

		if call.HasChildren != nil {
			data.HasChildren = call.HasChildren
		}

		if call.From != nil {
			if call.From.Type != nil {
				data.FromType = call.From.Type
			}
			if call.From.Number != nil {
				data.FromNumber = call.From.Number
			}
		}

		if call.To != nil {
			if call.To.Type != nil {
				data.ToType = call.To.Type
			}
			if call.To.Number != nil {
				data.ToNumber = call.To.Number
			}
		}

		if call.Files != nil {
			files := *call.Files // Разыменовываем указатель
			if len(files) > 0 && files[0].Name != nil {
				data.RecordFile = new(string)     // Создаем новый указатель на строку
				*data.RecordFile = *files[0].Name // Присваиваем значение
			} else {
				data.RecordFile = nil
			}
		}

		if call.Queue != nil && call.Queue.Name != nil {
			data.Queue = call.Queue.Name
		}
		if call.User != nil && call.User.Name != nil {
			data.UserName = call.User.Name
		}
		if call.Team != nil && call.Team.Name != nil {
			data.Team = call.Team.Name
		}
		if call.Agent != nil && call.Agent.Name != nil {
			data.Agent = call.Agent.Name
		}

		// SQL-запрос на вставку данных
		query := `INSERT INTO cdr.calls 
			(
				call_id,
				parent_id,
				created_at,
				from_type,
				from_number,
				to_type,
				to_number,
				destination,
				direction,
				queue,
				user_name,
				team,
				agent,
				duration,
				bill_sec,
				talk_sec,
				hold_sec,
				answered_at,
				cause,
				sip_code,
				hangup_by,
				hangup_at,
				bridged_at,
				has_children,
				transfer_from,
				transfer_to,
				wait_sec,
				record_file
			) 
			VALUES 
			(
				:call_id, 
				:parent_id, 
				:created_at, 
				:from_type, 
				:from_number,
				:to_type, 
				:to_number,
				:destination, 
				:direction, 
				:queue, 
				:user_name, 
				:team, 
				:agent, 
				:duration, 
				:bill_sec, 
				:talk_sec, 
				:hold_sec, 
				:answered_at, 
				:cause, 
				:sip_code, 
				:hangup_by, 
				:hangup_at,
				:bridged_at,
				:has_children,
				:transfer_from,
				:transfer_to,
				:wait_sec,
				:record_file
			)`

		//OutLog.Printf("Data: %v", data)

		// Выполнение запроса с именованными параметрами
		_, err = db.NamedExec(query, &data)
		if err != nil {
			ErrLog.Printf("Failed to insert data: %s", err.Error())
			return
		}
	}

	OutLog.Printf("Calls successfully added")
}

// CDR list godoc
// @Summary      List CDR
// @Description  Get a list of all routes with pagination
// @Tags         CDR
// @Accept       json
// @Produce      json
// @Success      200  {array}  model.CDRJsonResponse
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Number of routes per page" default(100)
// @Param data body model.CallHistoryRequest true "Provider"
// @Router       /list [post]
// @Security ApiKeyAuth
func GetList(db *sqlx.DB, c *gin.Context) {

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

	var cdrRequest model.CallHistoryRequest

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&cdrRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	// Начинаем строить запрос
	query := "SELECT * FROM cdr.calls AS c WHERE 1=1"
	query_count := "SELECT COUNT(id) FROM cdr.calls AS c WHERE 1=1"
	var args []interface{}
	var args_count []interface{}
	paramIndex := 1 // Индекс для параметров

	var dbSlice []model.CallHistory

	if (cdrRequest.From_date != nil && isValidDateFormat(*cdrRequest.From_date)) &&
		(cdrRequest.To_date != nil && isValidDateFormat(*cdrRequest.To_date)) {

		query += fmt.Sprintf(" AND c.created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		query_count += fmt.Sprintf(" AND c.created_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
		args = append(args, *cdrRequest.From_date, *cdrRequest.To_date)
		args_count = args
		paramIndex += 2
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Please provide a date range"})
		return
	}

	if cdrRequest.FromNumber != nil && *cdrRequest.FromNumber != "" {
		addCondition(&query, "c.from_number", paramIndex, &args, *cdrRequest.FromNumber)
		addCondition(&query_count, "c.from_number", paramIndex, &args_count, *cdrRequest.FromNumber)
		paramIndex++
	}

	if cdrRequest.ToNumber != nil && *cdrRequest.ToNumber != "" {
		addCondition(&query, "c.to_number", paramIndex, &args, *cdrRequest.ToNumber)
		addCondition(&query_count, "c.to_number", paramIndex, &args_count, *cdrRequest.ToNumber)
		paramIndex++
	}

	if cdrRequest.Destination != nil && *cdrRequest.Destination != "" {
		addCondition(&query, "c.destination", paramIndex, &args, *cdrRequest.Destination)
		addCondition(&query_count, "c.destination", paramIndex, &args_count, *cdrRequest.Destination)
		paramIndex++
	}

	if cdrRequest.Direction != nil && *cdrRequest.Direction != "" {
		addCondition(&query, "c.direction", paramIndex, &args, *cdrRequest.Direction)
		addCondition(&query_count, "c.direction", paramIndex, &args_count, *cdrRequest.Direction)
		paramIndex++
	}

	if cdrRequest.FromType != nil && *cdrRequest.FromType != "" {
		addCondition(&query, "c.from_type", paramIndex, &args, *cdrRequest.FromType)
		addCondition(&query_count, "c.from_type", paramIndex, &args_count, *cdrRequest.FromType)
		paramIndex++
	}

	if cdrRequest.ToType != nil && *cdrRequest.ToType != "" {
		addCondition(&query, "c.to_type", paramIndex, &args, *cdrRequest.ToType)
		addCondition(&query_count, "c.to_type", paramIndex, &args_count, *cdrRequest.ToType)
		paramIndex++
	}

	if cdrRequest.Queue != nil && *cdrRequest.Queue != "" {
		addCondition(&query, "c.queue", paramIndex, &args, *cdrRequest.Queue)
		addCondition(&query_count, "c.queue", paramIndex, &args_count, *cdrRequest.Queue)
		paramIndex++
	}

	if cdrRequest.Team != nil && *cdrRequest.Team != "" {
		addCondition(&query, "c.team", paramIndex, &args, *cdrRequest.Team)
		addCondition(&query_count, "c.team", paramIndex, &args_count, *cdrRequest.Team)
		paramIndex++
	}

	if cdrRequest.SipCode != nil && *cdrRequest.SipCode != 0 {
		addCondition(&query, "c.sip_code", paramIndex, &args, *cdrRequest.SipCode)
		addCondition(&query_count, "c.sip_code", paramIndex, &args_count, *cdrRequest.SipCode)
		paramIndex++
	}

	if cdrRequest.MinTalkSec != nil && *cdrRequest.MinTalkSec != 0 {
		query += fmt.Sprintf(" AND c.talk_sec >= $%d", paramIndex)
		query_count += fmt.Sprintf(" AND c.talk_sec >= $%d", paramIndex)
		args = append(args, *cdrRequest.MinTalkSec)
		args_count = args
		paramIndex++
	}

	if cdrRequest.MinWaitSec != nil && *cdrRequest.MinWaitSec != 0 {
		query += fmt.Sprintf(" AND c.wait_sec >= $%d", paramIndex)
		query_count += fmt.Sprintf(" AND c.wait_sec >= $%d", paramIndex)
		args = append(args, *cdrRequest.MinWaitSec)
		args_count = args
		paramIndex++
	}

	find_children := false
	if cdrRequest.HasChildren != nil && *cdrRequest.HasChildren {
		query += fmt.Sprintf(" AND c.has_children = $%d", paramIndex)
		query_count += fmt.Sprintf(" AND c.has_children >= $%d", paramIndex)
		args = append(args, true)
		args_count = args
		paramIndex++
		find_children = true
	}

	if cdrRequest.HangupBy != nil && *cdrRequest.HangupBy != "" {
		addCondition(&query, "c.hangup_by", paramIndex, &args, *cdrRequest.HangupBy)
		addCondition(&query_count, "c.hangup_by", paramIndex, &args_count, *cdrRequest.HangupBy)
		paramIndex++
	}

	if cdrRequest.TagID != nil && *cdrRequest.TagID != 0 {
		addCondition(&query, "c.tag_id", paramIndex, &args, *cdrRequest.TagID)
		addCondition(&query_count, "c.tag_id", paramIndex, &args_count, *cdrRequest.TagID)
		paramIndex++
	}

	if cdrRequest.Number != nil && *cdrRequest.Number != "" {
		query += fmt.Sprintf(" AND (c.from_number = $%d OR c.to_number = $%d OR c.destination = $%d)", paramIndex, paramIndex+1, paramIndex+2)
		query_count += fmt.Sprintf(" AND (c.from_number = $%d OR c.to_number = $%d OR c.destination = $%d)", paramIndex, paramIndex+1, paramIndex+2)
		args = append(args, *cdrRequest.Number, *cdrRequest.Number, *cdrRequest.Number)
		args_count = args
		paramIndex += 3
	}

	// Добавляем лимит и смещение
	query += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", paramIndex, paramIndex+1)
	args = append(args, limit, offset)

	// Выполняем запрос
	err := db.Select(&dbSlice, query, args...)
	if err != nil {
		ErrLog.Printf("Failed to fetch CDR: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to fetch CDR", "message": err.Error(), "args": args, "query": query})
		return
	}

	var rows_count int
	err = db.QueryRow(query_count, args_count...).Scan(&rows_count)
	if err != nil {
		ErrLog.Printf("Failed to count CDR: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to count CDR", "message": err.Error(), "args": args_count, "query": query_count})
		return
	}

	// Создаем срез для хранения всех вызовов с дочерними вызовами
	var callsWithChildren []model.CallHistory

	for _, call := range dbSlice {
		// Генерируем путь до записи
		//RecordPath := GenRecordPath(call.HangupAt, call.RecordFile)
		var RecordPath *string
		if call.RecordFile != nil {
			//path := APIPath + "file/" + strconv.Itoa(int(*call.ID))
			path := strconv.Itoa(int(*call.ID))
			RecordPath = &path // Инициализация указателя
		}

		var URL *string
		if call.CallID != nil {
			url := APIPath + "call/" + strconv.Itoa(int(*call.ID))
			URL = &url
		}

		// Создаем переменную для хранения текущего вызова
		currentCall := model.CallHistory{
			ID:     call.ID,
			CallID: call.CallID,
			//ParentID:   call.ParentID,
			CreatedAt:   call.CreatedAt,
			FromType:    call.FromType,
			FromNumber:  call.FromNumber,
			ToType:      call.ToType,
			ToNumber:    call.ToNumber,
			Destination: call.Destination,
			Direction:   call.Direction,
			Queue:       call.Queue,
			UserName:    call.UserName,
			Team:        call.Team,
			Agent:       call.Agent,
			Duration:    call.Duration,
			BillSec:     call.BillSec,
			TalkSec:     call.TalkSec,
			HoldSec:     call.HoldSec,
			AnsweredAt:  call.AnsweredAt,
			Cause:       call.Cause,
			SipCode:     call.SipCode,
			HangupBy:    call.HangupBy,
			HangupAt:    call.HangupAt,
			BridgetAt:   call.BridgetAt,
			//HasChildren:  call.HasChildren,
			TransferFrom: call.TransferFrom,
			TransferTo:   call.TransferTo,
			WaitSec:      call.WaitSec,
			RecordFile:   RecordPath,
			Played:       call.Played,
			TagID:        call.TagID,
			CallURL:      URL,
			Children:     nil, // Изначально нет дочерних
		}

		// Индикатор пропустить ли звонок если не найден дочерний при фильтрации has_children=true
		skip_call := false

		// Проверяем, есть ли у текущего вызова дочерние записи
		if call.CallID != nil && call.HasChildren != nil && *call.HasChildren {
			var children []model.CallHistory

			// Выполняем запрос для получения дочерних вызовов
			err = db.Select(&children, "SELECT * FROM cdr.calls WHERE parent_id = $1", call.CallID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch data", "error": err.Error()})
				return
			}

			// Замена значения для каждого элемента в срезе
			for idx, _ := range children {
				children[idx].RecordFile = nil //GenRecordPath(child.HangupAt, child.RecordFile)
				children[idx].CallID = nil
				children[idx].ParentID = nil
			}

			if len(children) != 0 { // Если в срезе есть данные
				// Заполняем поле Children текущего вызова
				currentCall.Children = &children
			} else if len(children) == 0 && find_children {
				skip_call = true
			}
		}

		if !skip_call {
			// Добавляем текущий вызов в срез вызовов с дочерними
			callsWithChildren = append(callsWithChildren, currentCall)
		}
	}

	// Проверяем, есть ли данные на текущей странице
	if len(callsWithChildren) == 0 {
		response := model.CDRJsonResponseNull{
			Status: "success",
			Data:   []string{},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	response := model.CDRJsonResponse{
		Status: "success",
		Count:  rows_count,
		Data:   callsWithChildren,
	}

	c.IndentedJSON(http.StatusOK, response)
}

// Get file godoc
// @Summary      Execute get S3
// @Description  Get file from S3
// @Tags         File
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "ID"
// @Router       /file/{id} [get]
// @Security ApiKeyAuth
func GetFile(db *sqlx.DB, c *gin.Context) {
	id := c.Param("id")

	// Пробуем преобразовать строку в целое число
	_, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "error": "id is not a valid number", "message": err.Error()})
		return
	}

	var file model.RecordFile
	err = db.Get(&file, "SELECT hangup_at, record_file FROM cdr.calls WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to get file path from DB", "message": err.Error()})
		return
	}

	filePath := GenRecordPath(file.HangupAt, file.RecordFile)
	filePathwithHour := GenRecordPathWithHour(file.HangupAt, file.RecordFile)

	// Проверяем существует ли файл на S3
	recordFileExists, err := S3FileExists(*filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "error": "Error getting file exists", "message": err.Error()})
		return
	}

	var validPath *string

	// Если файл не существует, то пробуем по второму пути с часами
	if recordFileExists {
		validPath = filePath
	} else {
		validPath = filePathwithHour
	}

	// Узнаём размер файла
	fileSize, err := S3GetSize(*validPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "error": "Error getting file size", "message": err.Error()})
		return
	}

	var start int64 = 0
	var end int64 = fileSize - 1
	var isPartial bool = false

	// Получаем заголовок Range от браузера
	requestedRange := c.GetHeader("Range")
	if requestedRange != "" {
		isPartial = true
		rangeParts := strings.Split(strings.TrimPrefix(requestedRange, "bytes="), "-")

		if len(rangeParts) == 2 {
			start, _ = strconv.ParseInt(rangeParts[0], 10, 64)
			if start < 0 {
				start = 0
			}

			end, _ = strconv.ParseInt(rangeParts[1], 10, 64)
			if end == 0 || end > fileSize {
				end = fileSize - 1
			}
		}
	}

	//requestedRange = fmt.Sprintf("bytes=%d-%d", start, end)
	if start >= end {
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"status": "failed", "error": "Requested range not satisfiable"})
		return
	}

	// Получаем объект из S3
	result, err := S3Get(*validPath, &start, &end)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "error": "Failed to get file from S3", "message": err.Error()})
		return
	}
	defer result.Close()

	// Получение текущего логина из JWT
	currentUID, _ := c.Get("uid")
	var playLogin string
	if currentUID != nil {
		userFields, err := GetFIO(db, currentUID.(string))
		if err != nil {
			ErrLog.Printf("Failed to get user info: %v", err)
		}
		if userFields.FirstName != nil && userFields.LastName != nil {
			playLogin = *userFields.FirstName + " " + *userFields.LastName
		} else {
			playLogin = "User"
		}
	} else {
		playLogin = "User"
	}

	// Загружаем временную зону из конфигурации
	location, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to load timezone", "error": err.Error()})
		return
	}
	currentDate := time.Now().In(location)

	playText := currentDate.Format("02.01.2006 15:04:05") + " - " + playLogin

	_, err = db.Exec("UPDATE cdr.calls SET played = $1 WHERE id = $2", playText, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to update played info", "message": err.Error()})
		return
	}

	// Определяем, нужно ли стримить или скачивать файл
	isStreaming := c.Query("stream") == "true"

	if isStreaming {
		// Устанавливаем нужные заголовки ответа
		c.Header("Accept-Ranges", "bytes")
		c.Header("Content-Type", "audio/mpeg")
		c.Header("Content-Length", fmt.Sprintf("%d", end-start+1))
		c.Status(http.StatusOK)

		if isPartial {
			c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
			c.Status(http.StatusPartialContent)
		}

		// Потоковая передача данных
		c.Stream(func(w io.Writer) bool {
			_, err = io.Copy(w, result)
			return err == nil
		})
	} else {
		// Логика для скачивания файла целиком
		filename := filepath.Base(*file.RecordFile)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Type", "audio/mpeg")
		c.Header("Content-Length", fmt.Sprintf("%d", fileSize))

		// Потоковая передача данных
		c.Stream(func(w io.Writer) bool {
			_, err = io.Copy(w, result)
			return err == nil
		})
	}

}

// Get call godoc
// @Summary      Execute get call
// @Description  Get one call
// @Tags         Calls
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "ID"
// @Router       /call/{id} [get]
// @Security ApiKeyAuth
func GetCall(db *sqlx.DB, c *gin.Context) {
	id := c.Param("id")

	// Пробуем преобразовать строку в целое число
	_, err := strconv.Atoi(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "error": "id is not a valid number", "message": err.Error()})
		return
	}

	var call model.CallHistory
	err = db.Get(&call, "SELECT * FROM cdr.calls WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "error": "Failed to get file path from DB", "message": err.Error()})
		return
	}

	// Генерируем путь до записи
	var RecordPath *string
	if call.RecordFile != nil {
		path := APIPath + "file/" + strconv.Itoa(int(*call.ID))
		RecordPath = &path // Инициализация указателя
	}

	// Создаем срез для хранения всех вызовов с дочерними вызовами
	var callWithChildren []model.CallHistory
	// Создаем переменную для хранения текущего вызова
	currentCall := model.CallHistory{
		CallID:       call.CallID,
		CreatedAt:    call.CreatedAt,
		FromType:     call.FromType,
		FromNumber:   call.FromNumber,
		ToType:       call.ToType,
		ToNumber:     call.ToNumber,
		Destination:  call.Destination,
		Direction:    call.Direction,
		Queue:        call.Queue,
		UserName:     call.UserName,
		Team:         call.Team,
		Agent:        call.Agent,
		Duration:     call.Duration,
		BillSec:      call.BillSec,
		TalkSec:      call.TalkSec,
		HoldSec:      call.HoldSec,
		AnsweredAt:   call.AnsweredAt,
		Cause:        call.Cause,
		SipCode:      call.SipCode,
		HangupBy:     call.HangupBy,
		HangupAt:     call.HangupAt,
		BridgetAt:    call.BridgetAt,
		TransferFrom: call.TransferFrom,
		TransferTo:   call.TransferTo,
		WaitSec:      call.WaitSec,
		RecordFile:   RecordPath,
		TagID:        call.TagID,
		Children:     nil, // Изначально нет дочерних
	}
	skip_call := false
	// Проверяем, есть ли у текущего вызова дочерние записи
	if call.CallID != nil && call.HasChildren != nil && *call.HasChildren {
		var children []model.CallHistory

		// Выполняем запрос для получения дочерних вызовов
		err = db.Select(&children, "SELECT * FROM cdr.calls WHERE parent_id = $1", call.CallID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to fetch data", "error": err.Error()})
			return
		}

		// Замена значения для каждого элемента в срезе
		for idx, _ := range children {
			children[idx].RecordFile = nil
			children[idx].CallID = nil
			children[idx].ParentID = nil
		}

		if len(children) != 0 { // Если в срезе есть данные
			// Заполняем поле Children текущего вызова
			currentCall.Children = &children
		} else if len(children) == 0 {
			skip_call = true
		}
	}

	if !skip_call {
		// Добавляем текущий вызов в срез вызовов с дочерними
		callWithChildren = append(callWithChildren, currentCall)
	}

	response := model.CDRJsonResponse{
		Status: "success",
		Count:  1,
		Data:   callWithChildren,
	}

	c.IndentedJSON(http.StatusOK, response)
}
