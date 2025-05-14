package model

import (
	"encoding/json"
	"time"
)

type SwaggerTeamsList struct {
	Status string         `json:"status"`
	Data   []SwaggerTeams `json:"data"`
}

type Teams struct {
	ID                 *int             `db:"id" json:"id,omitempty"`
	Name               *string          `db:"name" json:"name,omitempty"`
	WebitelResourceIDS *json.RawMessage `db:"webitel_res_ids" json:"webitel_res_ids,omitempty"`
	ActualVendorID     *int             `db:"actual_vendor_id" json:"actual_vendor_id"`
}

type SwaggerTeams struct {
	ID                 int      `json:"id"`
	Name               string   `json:"name"`
	WebitelResourceIDS Resource `json:"webitel_res_ids"`
	ActualVendorID     int      `json:"actual_vendor_id"`
}

type Resource struct {
	VendorID  int   `json:"vendor_id"`
	Resources []int `json:"resources"`
}

type ActiveTeamNumber struct {
	ID                *int       `db:"id" json:"team_id,omitempty"`
	Name              *string    `db:"name" json:"name,omitempty"`
	VendorName        *string    `db:"vendor_name" json:"vendor_name,omitempty"`
	Number            *string    `db:"value" json:"number,omitempty"`
	ActivatedAt       *time.Time `db:"activated_at" json:"activated_at,omitempty"`
	PeriodicSec       *int       `db:"periodic_sec" json:"-"`
	ExpiredAt         *time.Time `json:"expired_at,omitempty"`
	ExpiredAtUnixTime *int64     `json:"expired_at_unixtime,omitempty"`
	Spin              *int       `db:"spin" json:"spin,omitempty"`
}

type SwaggerActiveTeamNumber struct {
	Status string             `json:"status"`
	Data   []ActiveTeamNumber `json:"data"`
}

type TeamDayNumbers struct {
	ID    *int       `db:"id" json:"team_id,omitempty"`
	Name  *string    `db:"name" json:"name,omitempty"`
	Pools []DayPools `json:"pools,omitempty"`
}

type SwaggerTeamDayNumbers struct {
	Status string           `json:"status"`
	Data   []TeamDayNumbers `json:"data"`
}

type DayPools struct {
	ID           *int        `db:"id" json:"id,omitempty"`
	Name         *string     `db:"name" json:"name,omitempty"`
	Active       *bool       `db:"active" json:"active,omitempty"`
	VendorID     *int        `db:"vendor_id" json:"vendor_id,omitempty"`
	Rotation     *bool       `db:"rotation" json:"rotation,omitempty"`
	Finish       *bool       `db:"finish" json:"finish,omitempty"`
	CreatedAt    *time.Time  `db:"created_at" json:"created_at,omitempty"`
	FinishAt     *time.Time  `db:"finish_at" json:"finish_at,omitempty"`
	SubPoolBlock *int        `db:"subpool_block" json:"block,omitempty"`
	NumbersCount *int        `db:"num_count" json:"num_count,omitempty"`
	SubPool      *DaySubPool `json:"active_subpool,omitempty"`
}

type DaySubPool struct {
	ID          *int         `db:"id" json:"id,omitempty"`
	ActivatedAt *time.Time   `db:"activated_at" json:"activated_at,omitempty"`
	Spin        *int         `db:"spin" json:"spin,omitempty"`
	Numbers     []DayNumbers `json:"numbers,omitempty"`
}

type DayNumbers struct {
	ID          int        `db:"id" json:"id"`
	Number      string     `db:"value" json:"number,omitempty"`
	ActivatedAt *time.Time `db:"activated_at" json:"last_activated_at"`
	Active      bool       `db:"active" json:"active"`
	Used        bool       `db:"used" json:"used"`
	Enabled     bool       `db:"enabled" json:"enabled"`
	Spin        int        `db:"spin" json:"spin"`
	Marked      bool       `db:"-" json:"marked"`
	Logs        []DayLogs  `json:"logs,omitempty"`
}

type DayLogs struct {
	StartAt *time.Time `db:"start_at" json:"start_at,omitempty"`
	EndAt   *time.Time `db:"end_at" json:"end_at,omitempty"`
}

type TeamsDayRequest struct {
	From_date *string `json:"from_date,omitempty"`
	To_date   *string `json:"to_date,omitempty"`
}
