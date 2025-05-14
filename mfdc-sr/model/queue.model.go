package model

import (
	"time"
)

type Queues struct {
	ID               *int       `db:"id" json:"-"`
	QueueID          *int       `db:"queue_id" json:"group_id,omitempty"`
	QueryName        *string    `db:"queue_name" json:"team,omitempty"`
	MaxCalls100      *int       `db:"max_calls_100" json:"-"`
	MaxAgentLine100  *int       `db:"max_agent_line_100" json:"-"`
	LoadFactor100    *int       `db:"load_factor_100" json:"-"`
	CoefMaxAgentLine *float64   `db:"coef_mal" json:"-"`
	CoefLoadFactor   *float64   `db:"coef_lf" json:"-"`
	ChangeAt         *time.Time `db:"change_at" json:"change_at,omitempty"`
	ChangeLogin      *string    `db:"change_login" json:"change_login,omitempty"`
	CurrentPercent   *int       `db:"current_percent" json:"percent,omitempty"`
}

type SetQueue struct {
	Percent *int `json:"percent"`
}

type JsonResponseSwagger struct {
	Status string `json:"status"`
	Data   Queues `json:"data"`
}
