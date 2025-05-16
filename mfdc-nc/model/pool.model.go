package model

import "time"

type PoolNewRequest struct {
	Name         string `json:"name"`
	VendorID     int    `json:"vendor_id"`
	SubPoolBlock int    `json:"subpool_block"`
	TeamID       int    `json:"team_id"`
}

type PoolRedistribution struct {
	Count      *int `json:"vendor_id,omitempty"`
	FromPoolID *int `json:"from_pool_id,omitempty"`
	ToPoolID   *int `json:"to_pool_id,omitempty"`
}

type Pool struct {
	ID            *int       `db:"id" json:"id,omitempty"`
	Name          *string    `db:"name" json:"name,omitempty"`
	Active        *bool      `db:"active" json:"active,omitempty"`
	Rotation      *bool      `db:"rotation" json:"rotation,omitempty"`
	Finish        *bool      `db:"finish" json:"finish,omitempty"`
	SubActivate   *bool      `db:"sub_activate" json:"-"`
	CreatedAt     *time.Time `db:"created_at" json:"created_at,omitempty"`
	FinishAt      *time.Time `db:"finish_at" json:"finish_at,omitempty"`
	SubPoolBlock  *int       `db:"subpool_block" json:"block,omitempty"`
	VendorID      *int       `db:"vendor_id" json:"-"`
	TeamID        *int       `db:"team_id" json:"-"`
	NumbersCount  *int       `db:"num_count" json:"num_count,omitempty"`
	SubPoolsCount *int       `db:"subpool_count" json:"subpool_count,omitempty"`
}

type SubPool struct {
	ID           int        `db:"id"`
	PoolID       int        `db:"pool_id"`
	CurrentIndex int        `db:"index"` // Индекс текущего сабпула в пуле
	LastNumberID int        `db:"last_nid"`
	Spin         int        `db:"spin"`
	Status       string     `db:"status"`
	LastChanged  *bool      `db:"last_changed"`
	ActivatedAt  *time.Time `db:"activated_at"`
}

type SwaggerPoolsList struct {
	Status string `json:"status"`
	Data   []Pool `json:"data"`
}
