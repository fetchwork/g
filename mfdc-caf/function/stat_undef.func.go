package function

/*
func getStatFromAPI(queue_id int, page int, from_date int64, to_date int64) (statusCode int, response model.StatResponse, err error) {
	// Создаем новый запрос
	url := fmt.Sprintf("%s/call_center/queues/attempts/history?page=%d&size=350&joined_at.from=%d&joined_at.to=%d&queue_id=%d", config.API_Webitel.URL, page, from_date, to_date, queue_id)

	// Читаем тело ответа
	body, statusCode, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "GET", url, "")
	if err != nil {
		return statusCode, model.StatResponse{}, fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	// Парсим JSON-ответ
	err = json.Unmarshal([]byte(body), &response)
	if err != nil {
		return statusCode, model.StatResponse{}, fmt.Errorf("failed to parse JSON response from Webitel API: %s", err.Error())

	}

	return statusCode, response, nil
}

func GetStatToTemp(db *sqlx.DB, ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		start := time.Now()

		// Получаем срез команд
		var teamsDB []model.TeamDB
		err := db.Select(&teamsDB, "SELECT * FROM caf.teams")
		if err != nil {
			return fmt.Errorf("failed to get teams: %w", err)
		}

		teams := make([]model.Team, len(teamsDB))
		for idx, team := range teamsDB {
			teams[idx].ID = team.ID
			teams[idx].Name = team.Name
			teams[idx].WebitelQueuesIDS = PgIntArr2IntArr(team.WebitelQueuesIDS)
		}

		date := time.Now()
		location, err := LoadTimeLocation()
		if err != nil {
			return fmt.Errorf("failed to load time location: %w", err)
		}

		minusDay := date.In(location).Truncate(24 * time.Hour).Add(-1 * 24 * time.Hour)
		fromDate := ConvertTimeToUnixMillis(&minusDay)
		minusOneSec := minusDay.Add(24*time.Hour - time.Second)
		toDate := ConvertTimeToUnixMillis(&minusOneSec)

		for _, team := range teams {
			for _, queueID := range team.WebitelQueuesIDS {
				next := true
				for page := 1; next; page++ {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
						httpStatus, stat, err := getStatFromAPI(queueID, page, fromDate, toDate)
						if err != nil {
							return fmt.Errorf("failed to get data from API: %s", err)
						}
						if httpStatus > 299 || httpStatus < 199 {
							ErrLog.Printf("Failed to get data from API for queue_id %v and page %v: %s\n", queueID, page, err)
							break
						}

						for _, item := range stat.Items {
							var joinedAt, bridgedAt *time.Time
							if item.JoinedAt != nil {
								joinedAt, err = ConvertUnixMillisToTime(*item.JoinedAt)
								if err != nil {
									return fmt.Errorf("failed to convert unixtime millis to time: %s", err)
								}
							}
							if item.BridgetAt != nil {
								bridgedAt, err = ConvertUnixMillisToTime(*item.BridgetAt)
								if err != nil {
									return fmt.Errorf("failed to convert unixtime millis to time: %s", err)
								}
							}

							_, err = db.Exec("INSERT INTO caf.tmp_stat (joined_at, bridged_at, destination, result, queue_id) VALUES ($1, $2, $3, $4, $5)", joinedAt, bridgedAt, item.Destination.Destination, item.Result, item.Queue.ID)
							if err != nil {
								return fmt.Errorf("failed to insert new number: %s", err)
							}
							OutLog.Printf("Insert stat for number %s", item.Destination.Destination)
						}

						select {
						case <-ctx.Done():
							return ctx.Err()
						case <-time.After(1 * time.Second):
							// продолжаем выполнение через 1 секунду, перед следующей страницей
						}

						if !stat.Next {
							OutLog.Printf("Successfully received data from the API for queue_id %v", queueID)
							next = false
						}
					}
				}
			}
		}

		duration := time.Since(start)
		OutLog.Printf("All stat received in %s", duration)
		return nil
	}
}
*/
