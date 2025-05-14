package model

import "time"

type LogRequest struct {
	From_date *string `json:"from_date,omitempty"`
	To_date   *string `json:"to_date,omitempty"`
	QueueID   *int    `json:"queue_id,omitempty"`
}

type Logs struct {
	ID          int       `db:"id" json:"-"`
	QueueID     int       `db:"queue_id" json:"queue_id,omitempty"`
	QueueName   string    `db:"queue_name" json:"queue_name,omitempty"`
	ChangeAt    time.Time `db:"change_at" json:"change_at,omitempty"`
	ChangeLogin string    `db:"change_login" json:"change_login,omitempty"`
	LastPercent int       `db:"last_percent" json:"last_percent,omitempty"`
	NewPercent  int       `db:"new_percent" json:"new_percent,omitempty"`
}
