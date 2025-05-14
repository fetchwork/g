package model

import "time"

type Logs struct {
	ID          int64      `db:"id" json:"-"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at,omitempty"`
	NumID       *int64     `db:"num_id" json:"-"`
	Number      *string    `db:"number" json:"number,omitempty"`
	TeamID      *int       `db:"team_id" json:"caf_team_id,omitempty"`
	Description *string    `db:"description" json:"description,omitempty"`
	Filtered    *bool      `db:"filtered" json:"filtered,omitempty"`
	Sent        *bool      `db:"sent" json:"sent,omitempty"`
}

type EmailData struct {
	Logs     []Logs
	TeamName string
	FromDate string
	ToDate   string
}

type LogRequest struct {
	From_date *string `json:"from_date,omitempty"`
	To_date   *string `json:"to_date,omitempty"`
	Number    *string `json:"number,omitempty"` // Номер телефона
	TeamID    *int    `json:"caf_team_id"`
}

type LogJsonResponse struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}
