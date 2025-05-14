package model

import (
	"encoding/json"
	"time"
)

type CDR struct {
	Cid        *int64     `json:"cid,omitempty"`
	Pid        *int       `json:"pid,omitempty"`
	Provider   string     `json:"provider,omitempty"`
	CallID     *string    `json:"callid,omitempty"`
	Created    *time.Time `json:"created,omitempty"`
	EndAt      *time.Time `json:"end_at,omitempty"`
	CallerID   *string    `json:"callerid,omitempty"`
	Callee     *string    `json:"callee,omitempty"`
	Duration   *int       `json:"duration,omitempty"`
	Rate       *float64   `json:"rate,omitempty"`
	Bill       *float64   `json:"bill,omitempty"`
	Rid        *int       `json:"rid,omitempty"`
	Route      *string    `json:"route,omitempty"`
	Sip_code   *string    `json:"sip_code,omitempty"`
	Sip_reason *string    `json:"sip_reason,omitempty"`
	Team       *string    `json:"team,omitempty"`
}
type CDRRequest struct {
	Pids       *[]int    `json:"pids,omitempty"`
	Export     *bool     `json:"export,omitempty"`
	CallID     *string   `json:"callid,omitempty"`
	From_date  *string   `json:"from_date,omitempty"`
	To_date    *string   `json:"to_date,omitempty"`
	CallerID   *string   `json:"callerid,omitempty"`
	Callee     *string   `json:"callee,omitempty"`
	Sip_code   *string   `json:"sip_code,omitempty"`
	Sip_reason *string   `json:"sip_reason,omitempty"`
	Teams      *[]string `json:"teams,omitempty"`
}

type CDRReportRequest struct {
	From_date *string   `json:"from_date,omitempty"`
	To_date   *string   `json:"to_date,omitempty"`
	Team      *[]string `json:"teams,omitempty"`
}

type CDRReport struct {
	Provider    string          `db:"provider" json:"provider,omitempty"`
	TalkMinutes float64         `db:"talk_minutes" json:"talk_minutes,omitempty"`
	BillSumm    float64         `db:"bill_summ" json:"bill_summ,omitempty"`
	CountCalls  int64           `db:"count_calls" json:"count_calls,omitempty"`
	From_date   string          `json:"from_date,omitempty"`
	To_date     string          `json:"to_date,omitempty"`
	Team        []CDRReportTeam `json:"teams,omitempty"`
}

type CDRReportTeam struct {
	Team        string  `json:"team,omitempty"`
	TalkMinutes float64 `db:"talk_minutes" json:"talk_minutes,omitempty"`
	BillSumm    float64 `db:"bill_summ" json:"bill_summ,omitempty"`
}

type JsonResponseSwagger struct {
	Status string    `json:"status"`
	Data   CDRReport `json:"data"`
}
type CDRJsonResponse struct {
	Status string      `json:"status"`
	Count  int         `json:"count"`
	Data   interface{} `json:"data"`
}

type CDRJsonResponseNull struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data"`
}

type Export struct {
	ID          int              `db:"id" json:"id,omitempty"`
	Name        string           `db:"name" json:"name"`
	CreatedAt   time.Time        `db:"created_at" json:"created_at,omitempty"`
	New         bool             `db:"new" json:"-"`
	Saved       bool             `db:"saved" json:"-"`
	Done        bool             `db:"done" json:"done,omitempty"`
	DownloadURL string           `json:"download_url,omitempty"`
	Content     *string          `db:"content" json:"-"`
	ARGs        *json.RawMessage `db:"args" json:"-"`
}
type ExportMinimal struct {
	ID        int       `db:"id" json:"id,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at,omitempty"`
	Name      string    `db:"name" json:"name"`
}
type ExportRequest struct {
	CreatedAt time.Time `db:"created_at" json:"-"`
	New       bool      `db:"new" json:"-"`
}

type ExportDownload struct {
	DataDate time.Time `db:"data_date"`
	Content  *string   `db:"content"`
}
