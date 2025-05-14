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
	PostgreSQL_webitel struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"postgresql_webitel"`
	API struct {
		Bind              string        `json:"bind"`
		Key               string        `json:"key"`
		TokenVersionCache time.Duration `json:"token_version_cache_minut"`
	} `json:"api"`
	API_Webitel struct {
		URL    string `json:"url"`
		Header string `json:"header"`
		Key    string `json:"key"`
	} `json:"webitel_api"`
}

type Reload struct {
	Reload string `json:"reload"`
}

type JsonResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}
