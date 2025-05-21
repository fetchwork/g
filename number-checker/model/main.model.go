package model

import (
	"time"
)

// Config структура для хранения конфигурации
type Config struct {
	PostgreSQL struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"postgresql"`
	MySQL struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"mysql"`
	API struct {
		Bind                    string        `json:"bind"`
		Key                     string        `json:"key"`
		TimeZone                string        `json:"timezone"`
		DebugMode               bool          `json:"debug_mode"`
		ExpiredExportTasksMonth int           `json:"expired_export_tasks_month"`
		TokenVersionCache       time.Duration `json:"token_version_cache_minut"`
	} `json:"api"`
}

type Reload struct {
	Reload string `json:"reload"`
}

type JsonResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type JsonResponseError struct {
	Status  string `json:"status"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Sum struct {
	SID          int64     `db:"sid" json:"-"`
	PID          *int32    `db:"pid" json:"-"`
	Count        *int64    `db:"count" json:"count"`
	ProviderName *string   `db:"provider_name" json:"provider"`
	Created      time.Time `db:"created" json:"-"`
	TalkMinutes  *float64  `db:"talk_minutes" json:"talk_minutes"`
	BillSumm     *float64  `db:"bill_summ" json:"bill_summ"`
	Team         *string   `db:"team" json:"team"`
}
