package model

import "time"

// Config структура для хранения конфигурации
type Config struct {
	PostgreSQL struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"postgresql"`
	API struct {
		Bind              string        `json:"bind"`
		Key               string        `json:"key"`
		TokenVersionCache time.Duration `json:"token_version_cache_minut"`
		DebugMode         bool          `json:"debug_mode"`
	} `json:"api"`
	VC_API struct {
		URL    string `json:"url"`
		Header string `json:"header"`
		Key    string `json:"key"`
	} `json:"vc_api"`
	API_Webitel struct {
		URL    string `json:"url"`
		Header string `json:"header"`
		Key    string `json:"key"`
	} `json:"webitel_api"`
	Rotate struct {
		EnableRotation           bool   `json:"rotation"`
		SubpoolActivateTimeHour  int    `json:"subpool_activate_time_hour"`
		SubpoolActivateTimeMinut int    `json:"subpool_activate_time_minut"`
		TimeZone                 string `json:"timezone"`
	} `json:"rotate"`
}

type Reload struct {
	Reload string `json:"reload"`
}

type SwaggerDefaultResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SwaggerStandartList struct {
	Status string        `json:"status"`
	Data   []interface{} `json:"data"`
}
