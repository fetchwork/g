package model

import (
	"time"
)

type Sub struct {
	ID                *int               `db:"id" json:"id,omitempty"`
	GroupName         *string            `db:"group_name" json:"team,omitempty"`
	GroupID           *int               `db:"group_id" json:"group_id,omitempty"`
	Analizator        *string            `db:"analizator" json:"analizator,omitempty"`
	Vendor            *string            `db:"vendor" json:"vendor,omitempty"`
	Actual            *bool              `db:"actual" json:"actual,omitempty"`
	ReserveResource   SumReserveResource `db:"-" json:"reserve,omitempty"`
	ReserveAnalizator *string            `db:"reserve_analizator" json:"-"`
	ReserveVendor     *string            `db:"reserve_vendor" json:"-"`
	Priority          *int               `db:"priority" json:"priority,omitempty"`
	ChangeAt          *time.Time         `db:"change_at" json:"change_at,omitempty"`
	ChangeLogin       *string            `db:"change_login" json:"change_login"`
}

type SumReserveResource struct {
	Analizator string `db:"reserve_analizator" json:"analizator,omitempty"`
	Vendor     string `db:"reserve_vendor" json:"vendor,omitempty"`
}

type RequestReserveResource struct {
	Analizator string `json:"analizator"`
	Vendor     string `json:"vendor"`
}

type RequestResources struct {
	Analizator string                 `json:"analizator"`
	Vendor     string                 `json:"vendor"`
	Priority   int                    `json:"priority,omitempty"`
	Reserve    RequestReserveResource `json:"reserve,omitempty"`
}
type RequestEditVendor struct {
	Resources []RequestResources `json:"resources"`
	Actual    bool               `json:"actual,omitempty"`
}

type DeleteReply struct {
	Status string `json:"status"`
}

type WebitelResource struct {
	ID string `json:"id"`
}

type WebitelItem struct {
	Id              string          `json:"id"`
	Resource        WebitelResource `json:"resource"`
	ReserveResource WebitelResource `json:"reserve_resource"`
	Priority        int             `json:"priority"`
}

type WebitelResponse struct {
	Items []WebitelItem `json:"items"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Структура для хранения соответствий
type ResourceMapping struct {
	ItemID            string
	ResourceID        int
	Priority          int
	ReserveResourceID int
}
