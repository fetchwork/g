package function

import (
	"fmt"
	"nc/model"
	"time"

	"github.com/jmoiron/sqlx"
)

func CheckActiveTime(startTime time.Time, stopTime time.Time) (bool, error) {
	// Устанавливаем временную зону
	loc, err := time.LoadLocation(config.Rotate.TimeZone)
	if err != nil {
		return false, fmt.Errorf("failed to load locations: %s", err)
	}
	// Получаем текущее время в нужной временной зоне
	now := time.Now().In(loc)

	// Устанавливаем дату для startTime и stopTime на текущую дату
	startTime = time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), startTime.Second(), 0, loc)
	stopTime = time.Date(now.Year(), now.Month(), now.Day(), stopTime.Hour(), stopTime.Minute(), stopTime.Second(), 0, loc)

	// Сравниваем текущее время с диапазоном
	if now.After(startTime) && now.Before(stopTime) { // Текущее время находится в диапазоне
		return true, nil
	} else {
		return false, nil
	}
}

func GetSchedulerSlice(db *sqlx.DB) (data []model.Scheduler, err error) {
	err = db.Select(&data, "SELECT sch.*, t.name AS team_name FROM nc.scheduler AS sch LEFT JOIN nc.teams AS t ON sch.team_id=t.id ORDER By sch.name")
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Запуск пулов по расписанию
func ScheduleRotate(db *sqlx.DB) error {
	data, err := GetSchedulerSlice(db)
	if err != nil {
		return fmt.Errorf("failed to get schedulers: %s", err)
	}

	// Перебираем расписания
	for _, schedule := range data {
		activeTime, err := CheckActiveTime(schedule.StartTime.Time, schedule.StopTime.Time)
		if err != nil {
			return fmt.Errorf("failed to check time: %s", err)
		}

		var Running bool
		err = db.Get(&Running, "SELECT EXISTS (SELECT 1 FROM nc.scheduler WHERE id=$1 AND running=$2)", schedule.ID, true)
		if err != nil {
			return fmt.Errorf("failed to check running schedule: %s", err)
		}

		// Сравниваем текущее время с диапазоном
		if activeTime {
			// Если ротация не запущена в активное время, то запускаем
			if !Running {
				_, err = db.Exec("UPDATE nc.scheduler SET running=$1 WHERE id=$2", true, schedule.ID)
				if err != nil {
					return fmt.Errorf("failed to enable running schedule: %s", err)
				}
			} else { // Если активное время и расписание уже запущено, то запускаем ротацию раз в periodic_sec

			}
		} else {
			// Если ротация запущена в неактивное время то останавливаем
			if Running {
				_, err = db.Exec("UPDATE nc.scheduler SET running=$1 WHERE id=$2", false, schedule.ID)
				if err != nil {
					return fmt.Errorf("failed to enable running schedule: %s", err)
				}
			}
		}

	}
	return nil
}
