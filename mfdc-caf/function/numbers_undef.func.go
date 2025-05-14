package function

/*
func getNumbersFromAPI(queue_id int, page int, from_date int64, to_date int64) (response model.NumbersResponse, err error) {
	// Создаем новый запрос
	url := fmt.Sprintf("%s/call_center/queues/%d/members?page=%d&size=350&created_at.from=%d&created_at.to=%d", config.API_Webitel.URL, queue_id, page, from_date, to_date)

	// Читаем тело ответа
	body, statusCode, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "GET", url, "")
	if err != nil {
		return model.NumbersResponse{}, fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	// Парсим JSON-ответ
	err = json.Unmarshal([]byte(body), &response)
	if err != nil {
		return model.NumbersResponse{}, fmt.Errorf("failed to parse JSON response from Webitel API: %s", err.Error())

	}

	return response, nil
}

func checkNumberExistAndGetCounter(db *sqlx.DB, number string) (exists bool, err error, counters model.ExistsNumberCounters) {
	// Выполняем запрос и заполняем attemptsCounter
	err = db.Get(&counters, "SELECT id, load_counter, attempts_counter FROM caf.numbers WHERE number = $1", number)
	if err != nil {
		// Если ошибка - возможно, запись не найдена, возвращаем 0 и false
		if err == sql.ErrNoRows {
			return false, nil, model.ExistsNumberCounters{}
		}
		return false, fmt.Errorf("failed to check attempts_counter: %w", err), model.ExistsNumberCounters{}
	}

	// Если запрос прошел успешно, значит запись существует
	return true, nil, counters
}

func GetNumbers(db *sqlx.DB, ctx context.Context) error {
	// Проверяем статус контекста
	select {
	case <-ctx.Done():
		// Контекст отменен или истек
		return ctx.Err()
	default:
		start := time.Now()
		// Получаем срез команд
		var teamsDB []model.TeamDB
		err := db.Select(&teamsDB, "SELECT * FROM caf.teams WHERE active = $1", true)
		if err != nil {
			return fmt.Errorf("failed to get teams: %s", err)
		}

		teams := make([]model.Team, len(teamsDB))

		for idx, team := range teamsDB {
			teams[idx].ID = team.ID
			teams[idx].Name = team.Name
			// Конвертируем срез pgtype.Int4Array в []int
			if team.WebitelQueuesIDS != nil {
				if teams[idx].WebitelQueuesIDS == nil {
					teams[idx].WebitelQueuesIDS = new([]int)
				}
				*teams[idx].WebitelQueuesIDS = PgIntArr2IntArr(*team.WebitelQueuesIDS)
			}
		}

		// Получаем текущее время
		date := time.Now()
		// Загружаем временную зону
		location, _ := LoadTimeLocation()

		// Устанавливаем minusHour на начало текущего часа минус 1 час (0 минут, 0 секунд)
		minusHour := date.In(location).Truncate(time.Hour).Add(-1 * time.Hour)

		// Преобразуем в unixtime
		from_date := ConvertTimeToUnixMillis(&minusHour)

		// Устанавливаем minusOneSec на конец предыдущего часа (59 минут, 59 секунд)
		minusOneSec := minusHour.Add(time.Hour - time.Second)

		// Преобразуем в unixtime
		to_date := ConvertTimeToUnixMillis(&minusOneSec)

		// Перебираем срез команд
		for _, team := range teams {

			if team.WebitelQueuesIDS != nil {
				// Перебираем срез очередей
				for _, queue_id := range *team.WebitelQueuesIDS {
					next := true
					for page := 1; next; page++ {
						select {
						case <-ctx.Done():
							return ctx.Err() // Если контекст отменен, возвращаем ошибку
						default:
							numbers, err := getNumbersFromAPI(queue_id, page, from_date, to_date)
							if err != nil {
								return fmt.Errorf("failed to get data from API: %s", err)
							}

							if len(numbers.Items) == 0 {
								OutLog.Printf("Queue ID: %v data of numbers is empty", queue_id)
							}

							// Перебираем полученный срез с номерами
							for _, number := range numbers.Items {

								if number.Name != nil {
									// Проверяем есть ли номер в БД
									existsNumber, err, counters := checkNumberExistAndGetCounter(db, *number.Name)
									if err != nil {
										return fmt.Errorf("failed to check exists number: %s", err)
									}

									if existsNumber {
										var newLoadCounter int
										if counters.LoadCounter != nil {
											newLoadCounter = *counters.LoadCounter + 1
										}

										var newAttemtsCounter int
										if counters.AttemtsCounter != nil && number.Attemts != nil {
											newAttemtsCounter = *counters.AttemtsCounter + *number.Attemts
										}
										_, err = db.Exec("UPDATE caf.numbers SET load_counter = $1, last_load_at = $2, attempts_counter = $3, queue_id = $4 WHERE id = $5", newLoadCounter, time.Now(), newAttemtsCounter, number.Queue.ID, counters.ID)
										if err != nil {
											return fmt.Errorf("failed to update exists number: %s", err)
										}
										//OutLog.Printf("Update number %s", *number.Name)
									} else {
										_, err = db.Exec("INSERT INTO caf.numbers (number, load_counter, first_load_at, attempts_counter, queue_id, team_id) VALUES ($1, $2, $3, $4, $5, $6)", number.Name, 1, time.Now(), number.Attemts, number.Queue.ID, team.ID)
										if err != nil {
											return fmt.Errorf("failed to insert new number: %s", err)
										}
										//OutLog.Printf("Insert number %s", *number.Name)
									}
								}

							}

							// Если в JSON-ответе нет больше страниц, то выходим из цикла
							if !numbers.Next {
								next = false // Выход из внутреннего цикла
							}
						}
					}
				}
			}
		}
		duration := time.Since(start) // Время выполнения
		OutLog.Printf("All Numbers received in %s", duration)
		return err
	}
}
*/
