package model

import (
	"time"
)

type VCResponse struct {
	Status string `json:"status"`
	Data   []Sub  `json:"data"`
}

type Sub struct {
	ID              *int               `json:"id,omitempty"`
	Team            *string            `json:"team,omitempty"`
	GroupID         *int               `json:"group_id,omitempty"`
	Analizator      *string            `json:"analizator,omitempty"`
	Vendor          *string            `json:"vendor,omitempty"`
	Actual          *bool              `json:"actual,omitempty"`
	ReserveResource SumReserveResource `json:"reserve,omitempty"`
	Priority        *int               `json:"priority,omitempty"`
	ChangeAt        *time.Time         `json:"change_at,omitempty"`
	ChangeLogin     *string            `json:"change_login"`
}

type SumReserveResource struct {
	Analizator string `json:"analizator,omitempty"`
	Vendor     string `json:"vendor,omitempty"`
}
