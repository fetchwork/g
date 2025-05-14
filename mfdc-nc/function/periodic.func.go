package function

import (
	"database/sql"
	"fmt"
	"nc/model"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

func setSubPoolDeactivate(db *sqlx.DB, poolID int, sub_activate bool) error {
	// Устанавливаем глобальную переменную сигнализирующую что идёт дективация сабпула в этом пуле
	_, err := db.Exec("UPDATE nc.pools SET sub_activate=$1 WHERE id=$2", sub_activate, poolID)
	if err != nil {
		return fmt.Errorf("failed to update subpool in DB: %s", err)
	}
	return nil
}

// Деактивация старого сабпула
func deactivateOldSubPool(db *sqlx.DB, poolID int) (subpool_id int, error error) {

	// Сигнализируем что идёт процесс деактивации сабпула
	setSubPoolDeactivate(db, poolID, true)

	var subPool model.SubPool
	err := db.Get(&subPool, "SELECT * FROM nc.subpools WHERE pool_id=$1 AND status=$2 ORDER By id LIMIT 1", poolID, "active")
	if err != nil {
		if err == sql.ErrNoRows {
			return 1, nil
		} else {
			return 0, fmt.Errorf("failed to get subpool from DB: %s", err)
		}
	}

	subPool.Status = "used"

	if subPool.ActivatedAt == nil {
		ActivatedAt := time.Now()
		subPool.ActivatedAt = &ActivatedAt
	}

	// Нужно для endlog() чтобы определить в каком сабпуле смотреть last_nid
	LastChanged := true
	subPool.LastChanged = &LastChanged

	// Деактивируемому сабпулу меняем last_changed на true, чтобы endlog() могла определить крайний неактивный сабпул
	_, err = db.NamedExec("UPDATE nc.subpools SET status=:status, last_changed=:last_changed WHERE id=:id", &subPool)
	if err != nil {
		return 0, fmt.Errorf("failed to update subpool in DB: %s", err)
	}

	return subPool.ID, nil
}

// Поиск и активация нового сабпула
func activateSubPool(db *sqlx.DB, poolID int) (subpoolID int, err error) {
	var subPool model.SubPool
	err = db.Get(&subPool, "SELECT * FROM nc.subpools WHERE pool_id=$1 AND status=$2 ORDER BY id LIMIT 1", poolID, "inactive")

	if err != nil {
		// Если неактивных(новых) сабпулов нет, то проверяем наличие used сабпулов
		if err == sql.ErrNoRows {
			return checkNoActiveSubPools(db, poolID)
		} else {
			return 0, fmt.Errorf("failed to get subpool from DB: %s", err)
		}
	}
	setSubPoolDeactivate(db, poolID, false)
	// Активируем найденный сабпул
	return activateFoundSubPool(db, &subPool)
}

// Активация найденного сабпула
func activateFoundSubPool(db *sqlx.DB, subPool *model.SubPool) (int, error) {
	subPool.Status = "active"

	// Увеличиваем SPIN для сабпула при активации
	subPool.Spin++

	activatedAt := time.Now()
	subPool.ActivatedAt = &activatedAt

	_, err := db.NamedExec("UPDATE nc.subpools SET status=:status, activated_at=:activated_at, spin=:spin WHERE id=:id", subPool)
	if err != nil {
		return 0, fmt.Errorf("failed to update subpool in DB: %s", err)
	}

	return subPool.ID, nil
}

func checkNoActiveSubPools(db *sqlx.DB, poolID int) (int, error) {
	var existSubPools bool
	err := db.Get(&existSubPools, "SELECT EXISTS (SELECT 1 FROM nc.subpools WHERE pool_id=$1 AND status=$2)", poolID, "used")

	if err != nil {
		return 0, fmt.Errorf("failed to check used subpools: %s", err)
	}

	// Если есть сабпулы со статусом used, то обнуляем для повторного цикла по пулу
	if existSubPools {
		// Активируем все использованные сабпулы и номера в них
		if err := reactivateUsedSubPools(db, poolID); err != nil {
			return 0, err
		}
	}
	setSubPoolDeactivate(db, poolID, false)
	return 0, nil // Возвращаем 0, так как активного сабпула нет
}

func reactivateUsedSubPools(db *sqlx.DB, poolID int) error {
	// Сбрасываем все сабпулы в неактивный статус
	_, err := db.Exec("UPDATE nc.subpools SET status=$1 WHERE pool_id=$2", "inactive", poolID)
	if err != nil {
		return fmt.Errorf("failed to update subpools in DB: %s", err)
	}

	activatedAt := time.Now()

	// Устанавливаем активный статус для нулевого сабпула в пуле и обновляем ему счётчик spin
	_, err = db.Exec("UPDATE nc.subpools SET spin = spin + 1, status=$1, activated_at=$2 WHERE pool_id=$3 AND index=$4", "active", activatedAt, poolID, 0)
	if err != nil {
		return fmt.Errorf("failed to update subpools in DB: %s", err)
	}

	// Сбрасываем номера
	_, err = db.Exec("UPDATE nc.numbers SET label=$1 WHERE pool_id=$2", false, poolID)
	if err != nil {
		return fmt.Errorf("failed to update numbers in DB: %s", err)
	}

	// Обновляем статус в пуле о том, что он был полностью использован
	_, err = db.Exec("UPDATE nc.pools SET finish=$1, finish_at=$2 WHERE id=$3", true, time.Now(), poolID)
	if err != nil {
		return fmt.Errorf("failed to update pool in DB: %s", err)
	}
	setSubPoolDeactivate(db, poolID, false)
	return nil
}

func activateNewSubPool(db *sqlx.DB) (msg string, err error) {
	// Получаем активные пулы
	var pools []model.Pool
	err = db.Select(&pools, "SELECT * FROM nc.pools WHERE active = $1", true)
	if err != nil {
		return "", fmt.Errorf("failed to get active pools from DB: %s", err)
	}

	if len(pools) == 0 {
		return "No active pools found for subpool activation", nil // Возвращаем сообщение об отсутствии активных пулов
	}

	var msgBuilder strings.Builder

	for _, pool := range pools {
		// Проверяем актуальный ли вендор у текущего пула
		actualVendor, err := CheckActualVendor(db, *pool.TeamID, *pool.VendorID)
		if err != nil {
			ErrLog.Printf("Failed to check actual vendor, error: %v\n", err)

			// Если с VC API проблемы то принимаем вендора из пула как актуального и продолжаем ротацию
			actualVendor = true
		}

		// Проверяем актуальный ли сейчас вендор у текущего пула
		if actualVendor {
			// Если вендор пула актуален то обновляем признак что пул ротируется
			_, err := db.Exec("UPDATE nc.pools SET rotation = $1 WHERE id=$2", true, *pool.ID)
			if err != nil {
				return "failed to update pool rotation status", err
			}
		} else { // Если вендор текущего пула не актуальный
			continue // Пропускаем активацию сабпула для этого пула
		}

		// Деактивируем старый сабпул
		oldSubpoolID, err := deactivateOldSubPool(db, *pool.ID)
		if err != nil {
			return "Error deactivating subpool", err // Возвращаем ошибку
		}

		// Обрабатываем результат деактивации
		if oldSubpoolID == 1 {
			msgBuilder.WriteString(fmt.Sprintf("Active subpools not found in pool ID: %d", *pool.ID))
		} else {
			msgBuilder.WriteString(fmt.Sprintf("Deactivated previus subpool ID: %d", oldSubpoolID))
		}

		// Добавляем разделитель между деактивацией и активацией
		msgBuilder.WriteString(". ")

		// Активируем новый сабпул
		subpoolID, err := activateSubPool(db, *pool.ID)
		if err != nil {
			return "Error activating subpool", err // Возвращаем ошибку
		}

		msgBuilder.WriteString(fmt.Sprintf("Activated new subpool ID: %d", subpoolID))

		// Добавляем разделитель между пулами
		msgBuilder.WriteString("\n")
	}

	// Убираем последний разделитель, если он есть
	result := msgBuilder.String()

	return result, nil // Возвращаем собранные сообщения и nil как ошибку
}

func activateNewSubPoolForPool(db *sqlx.DB, pool_id int) (msg string, err error) {
	// Получаем активные пулы
	var pool model.Pool
	err = db.Get(&pool, "SELECT * FROM nc.pools WHERE id = $1 AND active = $2", pool_id, true)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("Requested active pool not found")
		}
		return "", fmt.Errorf("Failed to get active pool from DB: %s", err)
	}

	var msgBuilder strings.Builder

	// Проверяем актуальный ли вендор у пула
	actualVendor, err := CheckActualVendor(db, *pool.TeamID, *pool.VendorID)
	if err != nil {
		ErrLog.Printf("Failed to check actual vendor, error: %v\n", err)

		// Если с VC API проблемы то принимаем вендора из пула как актуального и продолжаем ротацию
		actualVendor = true
	}

	// Проверяем актуальный ли сейчас вендор у текущего пула
	if actualVendor {
		// Если вендор пула актуален то обновляем признак что пул ротируется
		_, err := db.Exec("UPDATE nc.pools SET rotation = $1 WHERE id=$2", true, *pool.ID)
		if err != nil {
			return "Failed to update pool rotation status", err
		}
	} else { // Если вендор текущего пула не актуальный
		return "", fmt.Errorf("Subpool will not be activated because vendor is not actual")
	}

	// Деактивируем старый сабпул
	oldSubpoolID, err := deactivateOldSubPool(db, *pool.ID)
	if err != nil {
		return "Error deactivating subpool", err // Возвращаем ошибку
	}

	// Обрабатываем результат деактивации
	if oldSubpoolID == 1 {
		msgBuilder.WriteString(fmt.Sprintf("Previus active subpools not found in pool ID: %d", *pool.ID))
	} else {
		msgBuilder.WriteString(fmt.Sprintf("Deactivated old subpool ID: %d", oldSubpoolID))
	}

	// Добавляем разделитель
	msgBuilder.WriteString(". ")

	// Активируем новый сабпул
	subpoolID, err := activateSubPool(db, *pool.ID)
	if err != nil {
		return "Error activating subpool", err // Возвращаем ошибку
	}

	msgBuilder.WriteString(fmt.Sprintf("Activated new subpool ID: %d", subpoolID))

	// Убираем последний разделитель, если он есть
	result := msgBuilder.String()

	return result, nil // Возвращаем собранные сообщения и nil как ошибку
}
