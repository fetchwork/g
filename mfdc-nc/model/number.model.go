package model

import "time"

// Структура для хранения номера
type Numbers struct {
	ID    int    `db:"id"`
	Value string `db:"value"`
	Used  bool   `db:"status"`
	Label bool   `db:"label"`
}

type Number struct {
	ID          int        `db:"id"`
	Spin        int        `db:"spin"`
	SubPoolID   int        `db:"subpool_id"`
	PoolID      int        `db:"pool_id"`
	VendorID    int        `db:"vendor_id"`
	TeamID      int        `db:"team_id"`
	Value       string     `db:"value"` // Номер телефона
	Used        *bool      `db:"used"`
	Label       *bool      `db:"label"`
	Active      *bool      `db:"active"`
	Enabled     *bool      `db:"enabled"`
	ActivatedAt *time.Time `db:"activated_at"`
	MovedAt     *time.Time `db:"moved_at"`
}

type NumberInfo struct {
	ID          int          `db:"id" json:"id"`
	Spin        int          `db:"spin" json:"spin"`
	Vendor      string       `db:"vendor" json:"vendor"`
	Team        string       `db:"team" json:"team"`
	Value       string       `db:"value" json:"number"` // Номер телефона
	Used        *bool        `db:"used" json:"used,omitempty"`
	Active      *bool        `db:"active" json:"active,omitempty"`
	Enabled     *bool        `db:"enabled" json:"enabled,omitempty"`
	ActivatedAt *time.Time   `db:"activated_at" json:"activated_at,omitempty"`
	MovedAt     *time.Time   `db:"moved_at" json:"moved_at,omitempty"`
	Logs        []NumberLogs `json:"logs,omitempty"`
}

type NumberLogs struct {
	StartAt *time.Time `db:"start_at" json:"start_at,omitempty"`
	EndAt   *time.Time `db:"end_at" json:"end_at,omitempty"`
}

type NumberTeamInfo struct {
	Number string `db:"value" json:"number"` // Номер телефона
	Team   string `db:"team" json:"routing"`
}

type NumbersInPool struct {
	ID        *int    `db:"id" json:"id,omitempty"`
	SubPoolID *int    `db:"subpool_id" json:"subpool_id,omitempty"`
	Vendor    *string `db:"vendor" json:"vendor,omitempty"`
	Team      *string `db:"team" json:"team,omitempty"`
	Value     *string `db:"value" json:"number,omitempty"` // Номер телефона
	Enabled   *bool   `db:"enabled" json:"enabled,omitempty"`
}

type NumbersJsonResponse struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}

type NumbersExclusion struct {
	Numbers []NumbersExclusionData `json:"numbers"`
}

type NumbersExclusionData struct {
	NumberID *int  `json:"number_id,omitempty"`
	Enabled  *bool `json:"enabled,omitempty"`
}
