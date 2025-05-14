package function

import (
	"context"
	"time"

	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

// Тестовая функция активации сабпула
func TestStartSubPoolActivateTask(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(40 * time.Second)
	defer ticker.Stop()

	for {
		select { // Ожидание событий от нескольких каналов
		case <-ticker.C: // Ожидаем данные из канала ticker с полем C (срабатывание таймера, сигнал. ticker тип time.Ticker)
			OutLog.Println("Start subpool activate")
			// Запускаем функцию активации сабпула
			msg, err := activateNewSubPool(db)
			if err != nil {
				ErrLog.Printf("Error: %s", err)
			}
			if msg != "" {
				OutLog.Printf("Info: %s", msg)
			}
		case <-ctx.Done(): // Если контекст горутины завершает родительский процесс
			OutLog.Println("Stopping subpool activate...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск перебора расписаний
func StartRotationSchedule(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(9 * time.Second)
	defer ticker.Stop()

	for {
		select { // Ожидание событий от нескольких каналов
		case <-ticker.C: // Ожидаем данные из канала ticker с полем C (срабатывание таймера, сигнал. ticker тип time.Ticker)
			//OutLog.Println("Running schedule listing")
			err := ScheduleRotate(db)
			if err != nil {
				ErrLog.Printf("Error: %s", err)
			}
		case <-ctx.Done(): // Если контекст горутины завершает родительский процесс
			OutLog.Println("Stopping schedule listing...")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск ротации расписания > пулов > сабпулов > номеров
func StartPeriodicRotation(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	lastRunTimes := make(map[int]time.Time)
	// Создаём канал для возможности остановки горутины с DailyRotation() когда текущий scheduler.Running == false
	stopChans := make(map[int]chan struct{})

	for {
		select {
		case <-ticker.C:
			//OutLog.Println("Checking active schedules")
			data, err := GetSchedulerSlice(db)
			if err != nil {
				ErrLog.Printf("Failed to get schedulers: %s", err)
				return
			}
			// Перебираем расписания
			for _, scheduler := range data {
				if scheduler.Running {
					//OutLog.Printf("Schedule %s worktime: %v", *scheduler.Name, scheduler.Running)
					currentTime := time.Now()
					lastRunTime, exists := lastRunTimes[*scheduler.TeamID]

					// Проверяем, прошло ли достаточно времени с последнего запуска
					if !exists || currentTime.Sub(lastRunTime) >= time.Duration(*scheduler.PeriodicSecond)*time.Second {
						OutLog.Printf("Schedule is running %v, periodic: %v", *scheduler.Name, *scheduler.PeriodicSecond)

						// Если уже существует горутина для этого teamID, то отправляем сигнал остановки
						if ch, ok := stopChans[*scheduler.TeamID]; ok {
							close(ch) // Закрываем канал, чтобы сигнализировать о завершении
						}

						// Создаем новый канал для остановки
						stopCh := make(chan struct{})
						stopChans[*scheduler.TeamID] = stopCh

						// Запускаем DailyRotation
						go func(teamID int, stopCh chan struct{}) {
							OutLog.Printf("Run rotation for TeamID %v", teamID)
							start := time.Now()                      // Начало выполнения
							err := DailyRotation(db, teamID, stopCh) // Передаем канал в функцию
							if err != nil {
								ErrLog.Printf("Error during rotation: %s", err)
							}
							duration := time.Since(start) // Время выполнения
							OutLog.Printf("TeamID %v rotation lasted %s", teamID, duration)

							// Обновляем время последнего выполнения
							lastRunTimes[teamID] = time.Now()
						}(*scheduler.TeamID, stopCh)
					}
				} else {
					// Если scheduler.Running == false, закрываем канал, если он существует
					if ch, ok := stopChans[*scheduler.TeamID]; ok {
						close(ch)
						delete(stopChans, *scheduler.TeamID) // Удаляем канал из мапы
					}
				}
			}
		case <-ctx.Done():
			OutLog.Println("Stopping periodic rotation...")
			// Закрываем все каналы остановки перед завершением
			for _, ch := range stopChans {
				close(ch)
			}
			return
		}
	}
}

func SubPoolActivate(ctx context.Context, db *sqlx.DB) {
	for {
		now := time.Now()

		// Загружаем локацию
		loc, err := time.LoadLocation(config.Rotate.TimeZone)
		if err != nil {
			ErrLog.Println("Failed to load locations pre subpool activation", err)
			return
		}
		// Устанавливаем запуск на
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), config.Rotate.SubpoolActivateTimeHour, config.Rotate.SubpoolActivateTimeMinut, 0, 0, loc)

		// Если текущее время уже после установленного времени, устанавливаем следующее время на завтра
		if now.After(nextRun) {
			nextRun = nextRun.Add(24 * time.Hour)
		}

		// Вычисляем время до следующего запуска
		durationUntilNextRun := nextRun.Sub(now)

		// Ждем до следующего запуска
		select {
		case <-time.After(durationUntilNextRun):
			OutLog.Println("Start subpool activator for date", time.Now())
			// Запускаем функцию активации сабпула
			msg, err := activateNewSubPool(db)
			if err != nil {
				ErrLog.Printf("Error: %s", err)
			}
			if msg != "" {
				OutLog.Printf("Info: %s", msg)
			}
		case <-ctx.Done():
			OutLog.Println("Stopping subpool activator")
			return // Завершаем выполнение функции
		}
	}
}

// Запуск синка с сервисом VC по актуализации вендора для команды
func StartVCSync(ctx context.Context, db *sqlx.DB) {
	ticker := time.NewTicker(80 * time.Second)
	defer ticker.Stop()

	for {
		select { // Ожидание событий от нескольких каналов
		case <-ticker.C: // Ожидаем данные из канала ticker с полем C (срабатывание таймера, сигнал. ticker тип time.Ticker)
			OutLog.Println("VC sync...")
			err := SyncActualVendor(db)
			if err != nil {
				ErrLog.Printf("VC sync error: %s", err)
			}
		case <-ctx.Done(): // Если контекст горутины завершает родительский процесс
			OutLog.Println("Stopping VC sync...")
			return // Завершаем выполнение функции
		}
	}
}
