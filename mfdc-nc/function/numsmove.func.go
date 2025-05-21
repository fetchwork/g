package function

import (
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Получение количества номеров в сабпуле для пула
func getNumbersCountBySubpool(db *sqlx.DB, poolID *int) (int, error) {
	if poolID == nil {
		return 0, fmt.Errorf("pool ID is empty")
	}

	var numCount int
	query := `SELECT COUNT(n.id) FROM nc.numbers n 
              LEFT JOIN nc.subpools s ON n.subpool_id = s.id
              WHERE n.pool_id = $1 AND s.index = 0`

	err := db.Get(&numCount, query, *poolID)
	if err != nil {
		return 0, err
	}

	return numCount, nil
}

// Получения среза карт с номерами из исходного пула
func getSrcNumbers(db *sqlx.DB, countNumbers *int, srcPoolID *int) (movedNumbers []map[string]int, err error) {
	// Создаем срез карт
	movedNumbers = make([]map[string]int, 0)

	// Проверка входных параметров
	if countNumbers == nil || srcPoolID == nil {
		return nil, fmt.Errorf("count numbers or source pool ID is empty")
	}

	var subPools []int
	// Получаем список сабпулов
	err = db.Select(&subPools, "SELECT id FROM nc.subpools WHERE pool_id = $1", *srcPoolID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subpool list: %w", err)
	}

	// Перебираем сабпулы
	for _, subPoolID := range subPools {
		var srcNumberIDS []int
		// Получаем n-номеров (countNumbers) из сабпула
		err := db.Select(&srcNumberIDS, "SELECT id FROM nc.numbers WHERE subpool_id = $1 LIMIT $2", subPoolID, *countNumbers)
		if err != nil {
			return nil, fmt.Errorf("failed to get number from subpool: %w", err)
		}

		// Перебираем полученные n-номера
		for _, srcNumberID := range srcNumberIDS {
			// Заполняем мапу
			srcNumberMap := map[string]int{
				"number_id": srcNumberID,
				"moved":     0,
			}
			// Добавляем в срез карт
			movedNumbers = append(movedNumbers, srcNumberMap)
		}
	}

	return movedNumbers, nil
}

// Получение среза ID сабпулов для пула назначения
func getSubpoolsIDSByPool(db *sqlx.DB, dstPoolID *int) ([]int, error) {
	// Проверяем, что указатель на идентификатор пула не равен nil
	if dstPoolID == nil {
		return nil, fmt.Errorf("destination pool ID is empty")
	}

	var dstSubpools []int
	// Выполняем SQL-запрос для получения идентификаторов сабпулов
	err := db.Select(&dstSubpools, "SELECT id FROM nc.subpools WHERE pool_id = $1", *dstPoolID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subpools IDs from pool: %w", err)
	}

	return dstSubpools, nil
}

func updateNumber(db *sqlx.DB, numberID int, newSubpoolID int, newPoolID *int) error {
	// Проверка входных параметров
	if newPoolID == nil {
		return fmt.Errorf("pool ID is empty")
	}

	_, err := db.Exec("UPDATE nc.numbers SET subpool_id = $1, pool_id = $2 WHERE id = $3", newSubpoolID, *newPoolID, numberID)
	if err != nil {
		return fmt.Errorf("failed to update number: %w", err)
	}

	return nil
}

// Получение максимального значения индекса сабпула в пуле
func getMaxIndexValueInSubpool(db *sqlx.DB, poolID *int) (int, error) {
	if poolID == nil {
		return 0, fmt.Errorf("pool ID is empty")
	}

	var maxIndexValue int
	err := db.Get(&maxIndexValue, "SELECT MAX(index) FROM nc.subpools WHERE pool_id = $1", *poolID)
	if err != nil {
		return 0, err
	}

	return maxIndexValue, nil
}

func NumbersMoveByPool(db *sqlx.DB, countNumbers *int, srcPoolID *int, dstPoolID *int, teamID *int, srcVendorID *int, dstVendorID *int, SrcSubPoolsCount *int, DstSubPoolsCount *int) error {
	// Проверяем чтобы vendor_id у исходного и пула назначения совподал
	if srcVendorID == nil || dstVendorID == nil {
		return fmt.Errorf("source and destination vendor cannot be nil")
	}
	if *srcVendorID != *dstVendorID {
		return fmt.Errorf("source and destination vendor mismatch")
	}

	/*
		Если пул источника имеет больше сабпулов чем пул назначения, то узнать кол-во номеров в сабпуле назначения и прибавить кол-во переносимых номеров (countNumbers), получаем кол-во номеров
		которые должны быть в новых сабпулах для пула назначения - newNumbersCountInSubpool
	*/
	if SrcSubPoolsCount == nil || DstSubPoolsCount == nil {
		return fmt.Errorf("source and destination subpools count cannot be nil")
	}
	var newNumbersCountInSubpool int
	if *SrcSubPoolsCount > *DstSubPoolsCount {
		numCount, err := getNumbersCountBySubpool(db, dstPoolID)
		if err != nil {
			return fmt.Errorf("failed to get numbers count by destination subpool: %s", err)
		}
		if countNumbers != nil {
			newNumbersCountInSubpool = numCount + *countNumbers
		}
	}

	// Циклом проходимся по сабпулам пула источника и берём id n-номеров (countNumbers) из каждого сабпула в срез movedNumbers
	movedNumbers, err := getSrcNumbers(db, countNumbers, srcPoolID)
	if err != nil {
		return fmt.Errorf("failed to get numbers source pool: %s", err)
	}

	// Проходимся по сабпулам назначения и берём их subpool_id в срез dstSubpools
	dstSubpools, err := getSubpoolsIDSByPool(db, dstPoolID)
	if err != nil {
		return fmt.Errorf("failed to get destination subpools IDs: %s", err)
	}

	// Операции перемещения
	var numFinish bool
	addedCount := 0

	if !numFinish {
		for _, dstSubpoolID := range dstSubpools {
			if numFinish {
				break // Прерываем внешний цикл, если все номера перемещены
			}
			for i := 0; i < *countNumbers; i++ {
				if numFinish {
					break // Прерываем внутренний цикл, если все номера перемещены
				}
				moved := false // Флаг для отслеживания, был ли перемещён хотя бы один номер в этом цикле

				for j := range movedNumbers {
					number := movedNumbers[j]
					// Если номер ещё не был перенесён
					if number["moved"] == 0 {
						// Перемещаем номер
						err := updateNumber(db, number["number_id"], dstSubpoolID, dstPoolID)
						if err != nil {
							return fmt.Errorf("failed to update number: %s", err)
						}
						// Помечаем номер как перемещённый
						movedNumbers[j]["moved"] = 1
						moved = true // Устанавливаем флаг, что номер был перемещён
						addedCount++ // Увеличиваем счётчик кол-ва добавленных номеров
						break        // Выходим из цикла по номерам после успешного перемещения
					}
				}

				// Проверяем, были ли перемещены номера в этом цикле
				if !moved {
					numFinish = true // Устанавливаем флаг завершения, если ни один номер не был перемещён
				}
			}
		}
	}

	// Если кол-во перемещённых номеров меньше чем кол-во номеров в срезе карт movedNumbers, то создаём доп. сабпулы в пуле назначения
	if addedCount < len(movedNumbers) {
		// Вычисляем сколько осталось не перемещённых номеров
		notAddedCount := len(movedNumbers) - addedCount
		// Вычисляем количество дополнительных сабпулов
		newSubPoolsCount := notAddedCount / newNumbersCountInSubpool
		if notAddedCount%newNumbersCountInSubpool != 0 {
			newSubPoolsCount++ // Если есть остаток, увеличиваем на 1
		}

		maxIndexValue, err := getMaxIndexValueInSubpool(db, dstPoolID)
		if err != nil {
			return fmt.Errorf("failed to get max index subpool in pool: %s", err)
		}
		newSubpoolIndex := maxIndexValue + 1

		var newSubpoolIDS []int
		for i := 0; i < newSubPoolsCount; i++ {
			// Создаём новый сабпул в пуле назначения
			newSubPoolID, err := createSubPool(db, *dstPoolID, newSubpoolIndex)
			if err != nil {
				return fmt.Errorf("failed to create new subpool: %s", err)
			}
			// Увеличиваем индекс нового сабпула
			newSubpoolIndex++
			// Добавляем ID созданного сабпула в срез
			newSubpoolIDS = append(newSubpoolIDS, newSubPoolID)
		}

		var numFinish bool
		addedCount := 0
		// Перебираем срез newSubpoolIDS со списков новых сабпул ID
		for _, newSubpoolID := range newSubpoolIDS {
			if numFinish {
				break // Прерываем внешний цикл, если все номера перемещены
			}

			for i := 0; i < newNumbersCountInSubpool; i++ {
				if numFinish {
					break // Прерываем внутренний цикл, если все номера перемещены
				}
				moved := false // Флаг для отслеживания, был ли перемещён хотя бы один номер в этом цикле

				for j := range movedNumbers {
					number := movedNumbers[j]
					// Если номер ещё не был перенесён
					if number["moved"] == 0 {
						// Перемещаем номер
						err := updateNumber(db, number["number_id"], newSubpoolID, dstPoolID)
						if err != nil {
							return fmt.Errorf("failed to update number: %s", err)
						}
						// Помечаем номер как перемещённый
						movedNumbers[j]["moved"] = 1
						moved = true // Устанавливаем флаг, что номер был перемещён
						addedCount++ // Увеличиваем счётчик кол-ва добавленных номеров
						break        // Выходим из цикла по номерам после успешного перемещения
					}
				}

				// Проверяем, были ли перемещены номера в этом цикле
				if !moved {
					numFinish = true // Устанавливаем флаг завершения, если ни один номер не был перемещён
				}
			}

		}

	}

	return nil
}
