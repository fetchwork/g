package model

import "time"

type BlockedRequest struct {
	From_date *string `json:"from_date,omitempty"`
	To_date   *string `json:"to_date,omitempty"`
	Number    *string `json:"number,omitempty"` // Номер телефона
	TeamID    *int    `json:"caf_team_id"`
}

type BlackList struct {
	ID          int64      `db:"id" json:"id"`
	CreatedAt   *time.Time `db:"created_at" json:"created_at,omitempty"`
	Number      *string    `db:"number" json:"number,omitempty"`
	TeamID      *int       `db:"team_id" json:"caf_team_id,omitempty"`
	Description *string    `db:"description" json:"description,omitempty"`
	Logs        *[]BLLogs  `db:"logs" json:"logs,omitempty"`
}

type BlackListAdd struct {
	ID          *int64  `db:"id" json:"id,omitempty""`
	Number      *string `db:"number" json:"number,omitempty"`
	TeamID      *int    `db:"team_id" json:"caf_team_id,omitempty"`
	Description *string `db:"description" json:"description,omitempty"`
}

type BLLogs struct {
	CreatedAt   *time.Time `db:"created_at" json:"created_at,omitempty"`
	Description *string    `db:"description" json:"description,omitempty"`
	Filtered    *bool      `db:"filtered" json:"filtered,omitempty"`
}

type BLJsonResponse struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Data   []BlackList `json:"data"`
}
