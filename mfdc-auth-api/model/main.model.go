package model

import (
	"time"

	"github.com/jackc/pgtype"
)

// Config структура для хранения конфигурации
type Config struct {
	RabbitMQ struct {
		URL         string `json:"url"`
		Exchange    string `json:"exchange"`
		Queue       string `json:"queue"`
		RoutingKey  string `json:"routing_key"`
		ConsumerKey string `json:"consumer_key"`
	} `json:"rabbitmq"`
	PostgreSQL struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname"`
	} `json:"postgresql"`
	API struct {
		Bind                string        `json:"bind"`
		Auth_bind           string        `json:"auth_bind"`
		Key                 string        `json:"key"`
		TokenExpires        time.Duration `json:"token_expires_hour"`
		TokenVersionCache   time.Duration `json:"token_version_cache_minut"`
		MaxCountFailedLogin int           `json:"max_count_failed_login_attempt"`
		ResetAttemptMinut   time.Duration `json:"reset_attempt_minut"`
	} `json:"api"`
}

type UserAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserAuth struct {
	UID       int              `json:"uid,omitempty"`
	Email     string           `db:"email"`
	Firstname string           `db:"firstname"`
	Lastname  string           `db:"lastname"`
	Password  string           `db:"password"`
	Role      string           `db:"role"`
	TeamID    int              `db:"team_id"`
	Sections  pgtype.TextArray `db:"sections"`
}

type JsonResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}
