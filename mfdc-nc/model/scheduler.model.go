package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type TimeTZ struct {
	Time time.Time `json:"time"`
}

func (ct *TimeTZ) MarshalJSON() ([]byte, error) {
	// Форматируем время в строку
	formattedTime := ct.Time.Format("15:04:05-07")
	return json.Marshal(formattedTime)
}

// UnmarshalJSON для десериализации
func (ct *TimeTZ) UnmarshalJSON(b []byte) error {
	// Создаем временную структуру для десериализации
	var temp struct {
		Time string `json:"time"`
	}

	// Десериализуем в временную структуру
	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	// Парсим строку в time.Time
	parsedTime, err := time.Parse("15:04:05-07", temp.Time)
	if err != nil {
		return err // Возвращаем ошибку, если парсинг не удался
	}

	ct.Time = parsedTime
	return nil
}

// Scan для работы с базой данных
func (ct *TimeTZ) Scan(value interface{}) error {
	if v, ok := value.(string); ok {
		// Парсим строку в time.Time
		parsedTime, err := time.Parse("15:04:05-07", v)
		if err != nil {
			return err // Возвращаем ошибку, если парсинг не удался
		}
		ct.Time = parsedTime
		return nil
	}
	return fmt.Errorf("cannot scan type %T into TimeTZ", value)
}

// Value для работы с базой данных
func (ct TimeTZ) Value() (driver.Value, error) {
	return ct.Time.Format("15:04:05-07"), nil
}

type Scheduler struct {
	ID             *int    `db:"id" json:"id,omitempty"`
	Name           *string `db:"name" json:"name,omitempty"`
	StartTime      *TimeTZ `db:"start_time" json:"start_time,omitempty"`
	StopTime       *TimeTZ `db:"stop_time" json:"stop_time,omitempty"`
	PeriodicSecond *int    `db:"periodic_sec" json:"periodic_sec,omitempty"`
	TeamID         *int    `db:"team_id" json:"team_id,omitempty"`
	TeamName       *string `db:"team_name" json:"team_name,omitempty"`
	Running        bool    `db:"running" json:"-"`
}
