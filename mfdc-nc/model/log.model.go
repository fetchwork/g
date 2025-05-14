package model

import "time"

type Logs struct {
	ID        string     `db:"id"`
	NumberID  int        `db:"number_id"` // Номер телефона
	SubPoolID int        `db:"subpool_id"`
	PoolID    int        `db:"pool_id"`
	VendorID  int        `db:"vendor_id"`
	TeamID    int        `db:"team_id"`
	Comment   string     `db:"comment"`
	StartAt   time.Time  `db:"start_at"`
	EndAt     *time.Time `db:"end_at"`
}

type LogRequest struct {
	From_date *string `json:"from_date,omitempty"`
	To_date   *string `json:"to_date,omitempty"`
	Number    *string `json:"number,omitempty"` // Номер телефона
	SubPoolID *int    `json:"subpool_id,omitempty"`
	PoolID    *int    `json:"pool_id,omitempty"`
	VendorID  *int    `json:"vendor_id,omitempty"`
	TeamID    *int    `json:"team_id,omitempty"`
}

type LogList struct {
	Number   string     `db:"number" json:"number,omitempty"`
	Vendor   string     `db:"vendor" json:"vendor,omitempty"`
	Team     string     `db:"team" json:"team,omitempty"`
	PoolName string     `db:"pool_name" json:"pool_name,omitempty"`
	Comment  string     `db:"comment" json:"comment,omitempty"`
	StartAt  *time.Time `db:"start_at" json:"start_at,omitempty"`
	EndAt    *time.Time `db:"end_at" json:"end_at,omitempty"`
}

type LogJsonResponse struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}
