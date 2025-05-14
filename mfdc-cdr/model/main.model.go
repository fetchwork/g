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
	PostgreSQLDWH struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"postgresql_dwh"`
	API struct {
		Bind              string        `json:"bind"`
		Key               string        `json:"key"`
		TimeZone          string        `json:"timezone"`
		DebugMode         bool          `json:"debug_mode"`
		TokenVersionCache time.Duration `json:"token_version_cache_minut"`
	} `json:"api"`
	API_Webitel struct {
		URL                 string        `json:"url"`
		Header              string        `json:"header"`
		Key                 string        `json:"key"`
		Collect             bool          `json:"collect_data"`
		FromHoursToNow      int           `json:"from_hours_to_now"`
		QueryOffset         int           `json:"query_offset"`
		PeriodicCheckSecond time.Duration `json:"periodic_check_second"`
		StartDateIfDbEmpty  *time.Time    `json:"start_date_if_db_empty"`
		StopDateIfDbEmpty   *time.Time    `json:"stop_date_if_db_empty"`
	} `json:"webitel_api"`
	AUTH_API struct {
		URL    string `json:"url"`
		Header string `json:"header"`
		Key    string `json:"key"`
	} `json:"auth_api"`
	S3 struct {
		Bucket   string `json:"bucket"`
		Region   string `json:"region"`
		Endpoint string `json:"endpoint"`
		Key      string `json:"key"`
		Secret   string `json:"secret"`
	} `json:"s3_params"`
}

type Reload struct {
	Reload string `json:"reload"`
}

type SwaggerStandartResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type SwaggerDataResponse struct {
	Status string        `json:"status"`
	Data   []interface{} `json:"data"`
}
