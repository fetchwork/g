package function

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"nc/model"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

func checkSubPoolDeactivate(db *sqlx.DB, poolID int) (result bool, err error) {
	var have bool
	err = db.Get(&have, "SELECT EXISTS(SELECT 1 FROM nc.pools WHERE sub_activate=$1 AND id=$2)", true, poolID)
	if err != nil {
		return false, fmt.Errorf("failed to check subpool in DB: %s", err)
	}
	return have, nil
}

func getWebitelResourcesIDS(db *sqlx.DB, teamID int) ([]model.Resource, error) {
	// Переменная для хранения JSON-данных
	var jsonData sql.NullString

	// Выполняем запрос к базе данных
	err := db.Get(&jsonData, "SELECT webitel_res_ids FROM nc.teams WHERE id=$1", teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team with ID %d: %w", teamID, err)
	}

	// Создаем переменную для хранения результатов
	var resourcesList []model.Resource

	// Проверяем, есть ли данные в jsonData
	if jsonData.Valid {
		// Парсим JSON
		err = json.Unmarshal([]byte(jsonData.String), &resourcesList)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON for team ID %d: %w", teamID, err)
		}
	} else {
		// Если jsonData невалиден, возвращаем пустую карту
		return []model.Resource{}, nil
	}

	return resourcesList, nil
}

// Ежедневная ротация
func DailyRotation(db *sqlx.DB, teamID int, stopCh chan struct{}) error {
	//OutLog.Println("Run DailyRotation func inside")
	if stopCh == nil {
		// Если stopCh не передан, создаём новый канал
		stopCh = make(chan struct{})
		defer close(stopCh) // Закрываем канал в конце
	}

	select {
	case <-stopCh:
		OutLog.Printf("DailyRotation function %v stopped", teamID)
		return nil
	default:
		var pools []model.Pool

		err := db.Select(&pools, "SELECT * FROM nc.pools WHERE team_id=$1 AND active=$2", teamID, true)
		if err != nil {
			return fmt.Errorf("failed to get active pools from DB: %s", err)
		}
		if len(pools) == 0 {
			return fmt.Errorf("no active pools found in the database")
		}

		errChan := make(chan error, len(pools))
		var wg sync.WaitGroup

		for _, pool := range pools {
			if pool.ID == nil || pool.TeamID == nil || pool.VendorID == nil {
				return fmt.Errorf("empty values in params for pool")
			}

			spd, _ := checkSubPoolDeactivate(db, *pool.ID)

			//OutLog.Printf("SPD: %v", spd)

			if spd {
				continue // Пропускаем ротацию для этого пула
			}

			// Получаем имя вендора по ID
			vendorName, err := getVendorByID(db, *pool.VendorID)
			if err != nil {
				return fmt.Errorf("failed to get vendor name, error: %v", err)
			}

			// Получаем имя команды по ID
			teamName, err := getTeamByID(db, *pool.TeamID)
			if err != nil {
				return fmt.Errorf("failed to get team name, error: %v", err)
			}

			// Проверяем актуальный ли вендор у текущего пула
			actualVendor, err := CheckActualVendor(db, *pool.TeamID, *pool.VendorID)
			if err != nil {
				ErrLog.Printf("Failed to check actual vendor, error: %v\n", err)

				// Если с VC API проблемы то принимаем вендора из пула как актуального и продолжаем ротацию
				actualVendor = true
			}

			OutLog.Printf("Pool: %s, Team: %s + Vendor: %s is actual: %v\n", *pool.Name, teamName, vendorName, actualVendor)

			// Проверяем актуальный ли сейчас вендор у текущего пула
			if !actualVendor {
				continue // Пропускаем ротацию для этого пула
			}

			wg.Add(1)

			go func(pool model.Pool) {
				defer wg.Done()

				var subPools []model.SubPool
				subPoolErr := db.Select(&subPools, "SELECT * FROM nc.subpools WHERE pool_id=$1 AND status=$2", pool.ID, "active")
				if subPoolErr != nil {
					errChan <- fmt.Errorf("failed to get sub pools for pool %v: %s", *pool.ID, subPoolErr)
					return
				}

				// Проверка на наличие активных сабпулов
				if len(subPools) == 0 {
					errChan <- fmt.Errorf("no available active subpool in pool: %v", *pool.ID)
					return
				}

				for _, subPool := range subPools {
					if subPool.ActivatedAt == nil {
						ActivatedAt := time.Now()
						subPool.ActivatedAt = &ActivatedAt
					}

					number, numberErr := getNextAvailableNumber(db, *pool.ID, &subPool)
					if numberErr != nil {
						errChan <- fmt.Errorf("failed to get next available number for subpool %v: %s", subPool.ID, numberErr)
						continue
					}

					WebitelResourceIDS, WebitelResourceIDSErr := getWebitelResourcesIDS(db, *pool.TeamID)
					if WebitelResourceIDSErr != nil {
						errChan <- fmt.Errorf("failed to get Webitel resources for number ID %v: %s", number.ID, WebitelResourceIDSErr)
						return
					}

					vendorID := *pool.VendorID

					// Флаг для проверки наличия ресурсов
					vendorFound := false

					// Перебираем вендоров и ищем нужный vendor_id
					for _, resource := range WebitelResourceIDS {
						if resource.VendorID == vendorID {
							vendorFound = true
							// Перебираем ресурсы текущего vendor_id
							for _, resourceID := range resource.Resources {
								// Отправляем номер в Webitel
								numUpdateErr := putToWebitel(resourceID, number.Value)
								if numUpdateErr != nil {
									errChan <- fmt.Errorf("failed to update in Webitel number ID %v: %s", number.ID, numUpdateErr)
									return
								}
								OutLog.Printf("Sent number %s to Webitel resource ID %v", number.Value, resourceID)
							}
						}
					}

					if !vendorFound {
						errChan <- fmt.Errorf("webitel resource_id is empty for team %v", *pool.TeamID)
						return
					}

					activationErr := activateNumber(db, number)
					if activationErr != nil {
						errChan <- fmt.Errorf("failed to activate number %v: %s", number.ID, activationErr)
						continue
					}

					//OutLog.Printf("Pool: %v, SubPool: %v", *pool.ID, number.SubPoolID)
					// Пишем лог окончания
					logEndErr := endLog(db, subPool)
					if logEndErr != nil {
						errChan <- fmt.Errorf("failed to write end log %v: %s", number.ID, logEndErr)
						continue
					}

					// Пишем текущий ID номера в сабпул addLastNumberID() обязательно должна быть после endLog()
					addLastIDErr := addLastNumberID(db, number)
					if addLastIDErr != nil {
						errChan <- fmt.Errorf("failed to add last number ID %v: %s", number.ID, addLastIDErr)
						continue
					}

					// Пишем лог старта
					logEntry := model.Logs{
						NumberID:  number.ID,
						SubPoolID: subPool.ID,
						PoolID:    *pool.ID,
						VendorID:  *pool.VendorID,
						TeamID:    *pool.TeamID,
						StartAt:   time.Now(),
						EndAt:     nil,
						Comment:   "Used",
					}
					logStartErr := startLog(db, logEntry)
					if logStartErr != nil {
						errChan <- fmt.Errorf("failed to log activation for number %v: %s", number.ID, logStartErr)
					}

					OutLog.Printf("Got number: %v, Pool: %v, SubPoolID: %v", number.Value, *pool.Name, subPool.ID)
					break // Переход к следующему сабпулу после активации номера
				}
			}(pool) // Передаем значение pool структуры в горутину
		}

		// Запускаем горутину для обработки ошибок
		go func() {
			wg.Wait()      // Ждем завершения всех горутин
			close(errChan) // Закрываем канал ошибок только после завершения всех горутин
		}()

		// Обработка ошибок в основном потоке
		var finalError error
		for e := range errChan {
			finalError = fmt.Errorf("%s", e.Error()) // Сохраняем все ошибки
		}

		//OutLog.Println("End DailyRotation func inside")

		return finalError // Возвращаем все собранные ошибки или nil
	}
}

func getNextNumberQuery(db *sqlx.DB, subPoolID int) (*model.Number, error) {
	var number model.Number
	// Делаем выборку только тех номеров у которых активные и пул и сабпул
	query :=
		`SELECT n.id, n.subpool_id, n.pool_id, n.value, n.used, n.label, n.activated_at, n.spin 
	  FROM nc.numbers AS n
	  INNER JOIN nc.subpools AS sp ON n.subpool_id = sp.id
	  INNER JOIN nc.pools AS p ON sp.pool_id = p.id AND p.active = TRUE
	  WHERE sp.status = $1 
	  AND n.subpool_id = $2 
	  AND n.label = $3
	  AND n.enabled = $4 
	  ORDER BY n.id 
	  LIMIT 1;`

	err := db.Get(&number, query, "active", subPoolID, false, true)
	return &number, err
}

// Получение следующего доступного номера
func getNextAvailableNumber(db *sqlx.DB, poolID int, subPool *model.SubPool) (*model.Number, error) {
	// Запрос активного номера
	number, err := getNextNumberQuery(db, subPool.ID)

	if err != nil {
		// Обработка случая, когда нет доступных номеров
		if err == sql.ErrNoRows {
			var exist_subpools int
			// Проверяем есть ли ещё активные сабпулы в текущем пуле
			err = db.Get(&exist_subpools, "SELECT COUNT(id) FROM nc.subpools WHERE pool_id=$1 AND status=$2", poolID, "active")
			if err != nil {
				return nil, fmt.Errorf("failed to get check active subpools: %s", err)
			}

			if exist_subpools == 1 { // Если нет активных сабпулов кроме текущего
				// Обнуляем номера в сабпуле для повторного цикла
				_, err = db.Exec("UPDATE nc.numbers SET label=$1, used=$2 WHERE subpool_id=$3", false, true, subPool.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to unchecked numbers in subpools: %s", err)
				}
				// Когда в сабпуле закончились номера, запускаем новый цикл ротации в текущем сабпуле и обновляем в нём spin
				_, err = db.Exec("UPDATE nc.subpools SET spin=spin+1 WHERE id=$1", subPool.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to update spin for subpool in DB: %s", err)
				}

				// Запрос активного номера
				number, err = getNextNumberQuery(db, subPool.ID)
				if err != nil {
					if err == sql.ErrNoRows {
						return nil, fmt.Errorf("cycle number not found in subpool: %v", subPool.ID)
					}
					return nil, fmt.Errorf("cycle failed to get number in subpool: %s", err)
				}
				return number, nil
			} else if exist_subpools > 1 {
				// Если номеров в пуле больше нет, но есть ещё активные сабпулы, устанавливаем статус для текущего сабпула = used
				_, err = db.Exec("UPDATE nc.subpools SET status=$1 WHERE id=$2", "used", subPool.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to update status for subpool in DB: %s", err)
				}
			}
		}
		return nil, fmt.Errorf("failed to get number in subpool: %s", err)
	}

	if number.ActivatedAt == nil {
		now := time.Now()         // Получаем текущее время
		number.ActivatedAt = &now // Устанавливаем указатель на текущее время
	}
	if number.Label == nil {
		label := true         // Создаем переменную для хранения значения
		number.Label = &label // Устанавливаем указатель на эту переменную
	}
	if number.Used == nil {
		used := true        // Создаем переменную для хранения значения
		number.Used = &used // Устанавливаем указатель на эту переменную
	}

	return number, nil
}

func addLastNumberID(db *sqlx.DB, number *model.Number) error {
	_, err := db.NamedExec("UPDATE nc.subpools SET last_nid=:id WHERE id=:subpool_id", number)
	if err != nil {
		return fmt.Errorf("failed to update subpool: %s", err)
	}

	return nil
}

// Активация номера
func activateNumber(db *sqlx.DB, number *model.Number) error {
	if number.Label != nil {
		Label := true
		number.Label = &Label
	}
	if number.Used != nil {
		Used := true
		number.Used = &Used
	}

	Active := true
	number.Active = &Active

	if number.ActivatedAt != nil {
		ActivatedAt := time.Now()
		number.ActivatedAt = &ActivatedAt
	}

	//OutLog.Printf("Label: %v, Used: %v, ActivatedAt: %v, ID: %v", number.Label, number.Used, number.ActivatedAt, number.ID)
	if number.Spin == 0 {
		number.Spin = 1
	} else {
		number.Spin++
	}

	_, err := db.NamedExec("UPDATE nc.numbers SET label=:label, used=:used, active=:active, activated_at=:activated_at, spin=:spin WHERE id=:id", number)
	if err != nil {
		return fmt.Errorf("failed to update number in DB: %s", err)
	}

	return nil
}

// Логирование активации номера
func startLog(db *sqlx.DB, logEntry model.Logs) error {
	_, err := db.NamedExec("INSERT INTO nc.logs (number_id, subpool_id, pool_id, vendor_id, team_id, start_at, end_at, comment) VALUES (:number_id, :subpool_id, :pool_id, :vendor_id, :team_id, :start_at, :end_at, :comment)", logEntry)
	if err != nil {
		return fmt.Errorf("failed to save log to DB: %s", err)
	}
	return nil
}

// Функция получения количества сабпулов в пуле
func countSubPools(db *sqlx.DB, PoolId int) (subpool_count int, err error) {
	var subpoolCount int
	err = db.Get(&subpoolCount, "SELECT COUNT(id) FROM nc.subpools WHERE pool_id=$1", PoolId)
	if err != nil {
		return 0, fmt.Errorf("failed to get subpool count for endlog: %s", err)
	}
	return subpoolCount, nil
}

// endLog записывает стоп-время номера по id из сабпула
func endLog(db *sqlx.DB, subPool model.SubPool) error {
	var subpool_exist bool

	err := db.Get(&subpool_exist, "SELECT EXISTS(SELECT 1 FROM nc.subpools WHERE pool_id=$1 AND last_changed=$2)", subPool.PoolID, true)
	if err != nil {
		return fmt.Errorf("failed to get subpool exists for endlog: %s", err)
	}

	//OutLog.Printf("Subpool: %v", subPool)
	var Index int
	if subpool_exist {
		//OutLog.Printf("Lastchanget: %v", subpool_exist)
		subPoolCount, err := countSubPools(db, subPool.PoolID)

		if subPool.CurrentIndex != 0 {
			Index = subPool.CurrentIndex - 1
		} else {
			Index = subPoolCount - 1
		}
		// Всем сабпулам в пуле меняем last_changed на false
		_, err = db.Exec("UPDATE nc.subpools SET last_changed=$1 WHERE pool_id=$2", false, subPool.PoolID)
		if err != nil {
			return fmt.Errorf("failed to update subpool in DB: %s", err)
		}
	} else {
		Index = subPool.CurrentIndex
	}
	//OutLog.Printf("Index: %v", Index)

	var previousNumberID int
	// Получаем ID предыдущего номера
	err = db.Get(&previousNumberID, "SELECT last_nid FROM nc.subpools WHERE pool_id=$1 AND index=$2", subPool.PoolID, Index)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("previous number in subpool not found")
		}
		return fmt.Errorf("failed query previous number ID: %s", err)
	}
	//OutLog.Println(previousNumberID)
	// Обновляем поле end_at в таблице nc.logs
	_, err = db.Exec("UPDATE nc.logs SET end_at=$1 WHERE number_id=$2 AND end_at IS NULL", time.Now(), previousNumberID)
	if err != nil {
		return fmt.Errorf("failed to update end log: %s", err)
	}

	//  У предыдущего номера удаляем информацию о том что номер активный
	_, err = db.Exec("UPDATE nc.numbers SET active=$1 WHERE id=$2", false, previousNumberID)
	if err != nil {
		return fmt.Errorf("failed to update end log: %s", err)
	}

	return nil
}
