package function

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Запуск функции проверки статистики номеров
func StartCompareNumberToStat(db *sqlx.DB, ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 00:30
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 30, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			statMu.Lock() // Блокируем мьютекс перед выполнением задачи
			OutLog.Println("Start compare stat")
			err := CompareNumberToStat(db, ctx)
			statMu.Unlock() // Освобождаем мьютекс после завершения задачи
			if err != nil {
				ErrLog.Printf("Failed to compare stat: %s", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopping compare stat...")
			return // Завершаем выполнение функции
		}
	}
}

// Ежедневная очистка успешных звонков за сутки
func JOBClearTodaySuccessCall(db *sqlx.DB, ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 01:00
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 0, 1, 0, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			clearSuccessMu.Lock()
			OutLog.Println("Start clear today success call")
			_, err := db.Exec("UPDATE caf.numbers SET today_success_call = $1 WHERE today_success_call = $2", false, true)
			clearSuccessMu.Unlock()
			if err != nil {
				ErrLog.Printf("Failed to clear today success rows: %s", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopping clear today success call...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск функции проверки номеров для блокирования по стратегии unsuccessful
func StartCheckNumberForBlockByUnsuccessful(db *sqlx.DB, ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 02:00
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 0, 2, 0, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			checkUnsuccessfulMu.Lock()
			OutLog.Println("Start check Unsuccessful strategy")
			err := CheckNumberForBlockByUnsuccessful(db)
			checkUnsuccessfulMu.Unlock()
			if err != nil {
				ErrLog.Printf("Failed to check Unsuccessful strategy: %s", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopping check Unsuccessful strategy...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск функции проверки номеров для блокирования по стратегии cause
func StartCheckNumberForBlockByCause(db *sqlx.DB, ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 02:30
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 0, 2, 30, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			checkCauseMu.Lock()
			OutLog.Println("Start check Cause strategy")
			err := CheckNumberForBlockByCause(db)
			checkCauseMu.Unlock()
			if err != nil {
				ErrLog.Printf("Failed to check Cause strategy: %s", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopping check Cause strategy...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск функции проверки заблокированных номеров если при изменений данных владельца номера была запрошена дополнительная проверка на сутки
func StartRecheckNumberForBlockByUnsuccessful(db *sqlx.DB, ctx context.Context) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.API.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load time locations", err)
			return
		}
		// Устанавливаем запуск на 03:00
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 0, 3, 0, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			recheckUnsuccessfulMu.Lock()
			OutLog.Println("Start recheck Unsuccessful strategy")
			err := RecheckNumberForBlockByUnsuccessful(db)
			recheckUnsuccessfulMu.Unlock()
			if err != nil {
				ErrLog.Printf("Failed to recheck Unsuccessful strategy: %s", err)
				return
			}
		case <-ctx.Done():
			fmt.Println("Stopping recheck Unsuccessful strategy...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск уведомления на email об отфильтрованных номерах
func StartFilteredNotify(db *sqlx.DB, ctx context.Context) {
	ticker := time.NewTicker(60 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			OutLog.Println("Start filtered notification")

			filteredNotifyMu.Lock() // Блокируем мьютекс перед выполнением задачи
			err := FilteredNotify(db)
			filteredNotifyMu.Unlock() // Освобождаем мьютекс после завершения задачи

			if err != nil {
				OutLog.Println("Error filtered notification:", err)
			}

		case <-ctx.Done():
			// Логируем завершение фоновой задачи
			OutLog.Println("Stopping filtered notification...")
			return // Завершаем выполнение функции
		}
	}
}
