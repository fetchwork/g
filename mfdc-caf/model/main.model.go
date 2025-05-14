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
		TimeZone          string        `json:"timezone"`
		DebugMode         bool          `json:"debug_mode"`
		SlaveNode         bool          `json:"slave_node"`
	} `json:"api"`
	BP_API struct {
		URL                string `json:"url"`
		ServiceSecurityKey string `json:"service_security_key"`
	} `json:"bp_api"`
	BILLING_API struct {
		URL    string `json:"url"`
		Header string `json:"header"`
		Key    string `json:"key"`
	} `json:"billing_api"`
	API_Webitel struct {
		URL         string `json:"url"`
		Header      string `json:"header"`
		Key         string `json:"key"`
		BlacklistID int64  `json:"blacklist_id"`
	} `json:"webitel_api"`
	MAIL struct {
		ServerAddr   string `json:"smtp_server_addr"`
		ServerPort   string `json:"smtp_server_port"`
		AuthUser     string `json:"auth_user"`
		AuthPassword string `json:"auth_password"`
	}
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
