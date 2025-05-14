package model

import (
	"time"
)

type Numbers struct {
	ID             int64      `db:"id" json:"id"`
	Number         string     `db:"number" json:"number"`
	ClientID       *string    `db:"client_id" json:"client_id,omitempty"`
	QueueID        *int       `db:"queue_id" json:"queue_id,omitempty"`
	TeamID         *int       `db:"team_id" json:"team_id,omitempty"`
	FirstLoadAt    *time.Time `db:"first_load_at" json:"first_load_at,omitempty"`
	LastLoadAt     *time.Time `db:"last_load_at" json:"last_load_at,omitempty"`
	LoadCounter    *int       `db:"load_counter" json:"load_counter,omitempty"`
	AttemtsCounter *int       `db:"attempts_counter" json:"attempts_counter,omitempty"`
	Success        bool       `db:"success" json:"-"`
	Blocked        bool       `db:"blocked" json:"blocked"`
	BlockedAt      *time.Time `db:"blocked_at" json:"blocked_at,omitempty"`
	StopExpirid    *time.Time `db:"stop_expirid" json:"stop_expirid,omitempty"`
	StatWaiting    bool       `db:"stat_waiting" json:"-"`
}

type NumbersBlocked struct {
	ID          int64      `db:"id" json:"id"`
	Number      string     `db:"number" json:"number"`
	ClientID    *string    `db:"client_id" json:"client_id,omitempty"`
	TeamID      *int       `db:"team_id" json:"team_id,omitempty"`
	Success     bool       `db:"success" json:"-"`
	StopDays    *int       `db:"stop_days" json:"stop_days"`
	StopExpirid *time.Time `db:"stop_expirid" json:"stop_expirid,omitempty"`
	StatWaiting bool       `db:"stat_waiting" json:"-"`
}

type ExistsNumberCounters struct {
	ID             int64 `db:"id"`
	LoadCounter    *int  `db:"load_counter"`
	AttemtsCounter *int  `db:"attempts_counter"`
}

type NumbersResponse struct {
	Next  bool                    `json:"next"`
	Items []NumbersResponseDetail `json:"items"`
}

type NumbersResponseDetail struct {
	Queue     NumbersResponseQueue     `json:"queue"`
	Variables NumbersResponseVariables `json:"variables"`
	Name      *string                  `json:"name"`
	CreatedAt *string                  `json:"created_at"`
	Attemts   *int                     `json:"attempts"`
}

type NumbersResponseQueue struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
}

type NumbersResponseVariables struct {
	UserID *string `json:"user_id"`
}
