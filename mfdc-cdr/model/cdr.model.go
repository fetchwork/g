package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Scan реализует интерфейс Scanner для типа Properties
func (p *Properties) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("Failed to scan Properties")
	}

	return json.Unmarshal(bytes, p)
}

// Value реализует интерфейс Valuer для типа Properties
func (p Properties) Value() (driver.Value, error) {
	return json.Marshal(p)
}

type Properties struct {
	Location string `json:"location,omitempty"`
}

// Делаем типы со звёздочкой, чтобы нормально обрабатывать NULL значения в БД
type CallHistory struct {
	ID           *int64         `db:"id" json:"id"`
	TagID        *int64         `db:"tag_id" json:"tag_id,omitempty"`
	CallID       *string        `db:"call_id" json:"call_id,omitempty"`
	ParentID     *string        `db:"parent_id" json:"parent_id,omitempty"`
	CreatedAt    *time.Time     `db:"created_at" json:"created_at,omitempty"`
	FromType     *string        `db:"from_type" json:"from_type,omitempty"`
	FromNumber   *string        `db:"from_number" json:"from_number,omitempty"`
	ToType       *string        `db:"to_type" json:"to_type,omitempty"`
	ToNumber     *string        `db:"to_number" json:"to_number,omitempty"`
	Destination  *string        `db:"destination" json:"destination,omitempty"`
	Direction    *string        `db:"direction" json:"direction,omitempty"`
	Queue        *string        `db:"queue" json:"queue,omitempty"`
	UserName     *string        `db:"user_name" json:"user_name,omitempty"`
	Team         *string        `db:"team" json:"team,omitempty"`
	Agent        *string        `db:"agent" json:"agent,omitempty"`
	Duration     *int           `db:"duration" json:"duration,omitempty"`
	BillSec      *int           `db:"bill_sec" json:"bill_sec,omitempty"`
	TalkSec      *int           `db:"talk_sec" json:"talk_sec,omitempty"`
	HoldSec      *int           `db:"hold_sec" json:"hold_sec,omitempty"`
	AnsweredAt   *time.Time     `db:"answered_at" json:"answered_at,omitempty"`
	Cause        *string        `db:"cause" json:"cause,omitempty"`
	SipCode      *int           `db:"sip_code" json:"sip_code,omitempty"`
	HangupBy     *string        `db:"hangup_by" json:"hangup_by,omitempty"`
	HangupAt     *time.Time     `db:"hangup_at" json:"hangup_at,omitempty"`
	BridgetAt    *time.Time     `db:"bridged_at,omitempty" json:"bridged_at,omitempty"`
	HasChildren  *bool          `db:"has_children,omitempty" json:"has_children,omitempty"`
	TransferFrom *string        `db:"transfer_from,omitempty" json:"transfer_from,omitempty"`
	TransferTo   *string        `db:"transfer_to,omitempty" json:"transfer_to,omitempty"`
	WaitSec      *int           `db:"wait_sec,omitempty" json:"wait_sec,omitempty"`
	RecordFile   *string        `db:"record_file,omitempty" json:"record_file_id,omitempty"`
	Played       *string        `db:"played,omitempty" json:"played,omitempty"`
	CallURL      *string        `db:"-" json:"call_url,omitempty"`
	Children     *[]CallHistory `json:"children,omitempty"`
}

type CallHistoryRequest struct {
	From_date   *string `json:"from_date,omitempty"`
	To_date     *string `json:"to_date,omitempty"`
	FromNumber  *string `json:"from_number,omitempty"`
	ToNumber    *string `json:"to_number,omitempty"`
	Destination *string `json:"destination,omitempty"`
	Direction   *string `json:"direction,omitempty"`
	Number      *string `json:"number,omitempty"`
	FromType    *string `json:"from_type,omitempty"`
	ToType      *string `json:"to_type,omitempty"`
	Queue       *string `json:"queue,omitempty"`
	Team        *string `json:"team,omitempty"`
	SipCode     *int    `json:"sip_code,omitempty"`
	MinTalkSec  *int    `json:"min_talk_sec,omitempty"`
	MinWaitSec  *int    `json:"min_wait_sec,omitempty"`
	HasChildren *bool   `json:"has_children,omitempty"`
	HangupBy    *string `json:"hangup_by,omitempty"`
	TagID       *int64  `json:"tag_id,omitempty"`
}

type RecordFile struct {
	HangupAt   *time.Time `db:"hangup_at" json:"hangup_at,omitempty"`
	RecordFile *string    `db:"record_file,omitempty" json:"record_file_id,omitempty"`
}

type MaxDate struct {
	MaxCreatedAt *time.Time `db:"max_created_at"`
}

type JSONRequest struct {
	Page       int                  `json:"page"`
	Size       int                  `json:"size"`
	Sort       string               `json:"sort"`
	Fields     []string             `json:"fields"`
	CreatedAt  JSONRequestCreatedAt `json:"created_at"`
	SkipParent bool                 `json:"skip_parent"`
}
type JSONRequestCreatedAt struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type JSONResponseCallsSlice struct {
	Next  bool               `json:"next"`
	Calls []JSONResponseCall `json:"items"` // Главный массив вызовов
}

// Структура для отдельного вызова
type JSONResponseCall struct {
	ID           *string             `json:"id,omitempty"`
	ParentID     *string             `json:"parent_id,omitempty"`
	Direction    *string             `json:"direction,omitempty"`
	Destination  *string             `json:"destination,omitempty"`
	From         *JSONResponseFrom   `json:"from,omitempty"`
	To           *JSONResponseTo     `json:"to,omitempty"`
	CreatedAt    *string             `json:"created_at,omitempty"`
	AnsweredAt   *string             `json:"answered_at,omitempty"`
	HangupAt     *string             `json:"hangup_at,omitempty"`
	HangupBy     *string             `json:"hangup_by,omitempty"`
	Cause        *string             `json:"cause,omitempty"`
	Duration     *int                `json:"duration,omitempty"`
	BillSec      *int                `json:"bill_sec,omitempty"`
	TalkSec      *int                `json:"talk_sec,omitempty"`
	HoldSec      *int                `json:"hold_sec,omitempty"`
	SipCode      *int                `json:"sip_code,omitempty"`
	Files        *[]JSONResponseFile `json:"files,omitempty"` // Если есть файлы, не всегда будет
	User         *JSONResponseUser   `json:"user,omitempty"`
	Queue        *JSONResponseQueue  `json:"queue,omitempty"`
	Team         *JSONResponseTeam   `json:"team,omitempty"`
	Agent        *JSONResponseAgent  `json:"agent,omitempty"`
	BridgetAt    *string             `json:"bridged_at,omitempty"`
	HasChildren  *bool               `json:"has_children,omitempty"`
	TransferFrom *string             `json:"transfer_from,omitempty"`
	TransferTo   *string             `json:"transfer_to,omitempty"`
	WaitSec      *int                `json:"wait_sec,omitempty"`
}

// Структура для файла
type JSONResponseFile struct {
	ID       *string `json:"id,omitempty"`        // ID файла
	Name     *string `json:"name,omitempty"`      // Имя файла
	Size     *string `json:"size,omitempty"`      // Размер файла
	MimeType *string `json:"mime_type,omitempty"` // MIME тип файла
	StartAt  *string `json:"start_at,omitempty"`  // Время начала (может отсутствовать)
	StopAt   *string `json:"stop_at,omitempty"`   // Время окончания (может отсутствовать)
}

// Структура для информации о звонящем
type JSONResponseFrom struct {
	Type   *string `json:"type,omitempty"`
	Number *string `json:"number,omitempty"`
	ID     *string `json:"id,omitempty"`   // ID может отсутствовать
	Name   *string `json:"name,omitempty"` // Имя может отсутствовать
}

// Структура для информации о принимающем
type JSONResponseTo struct {
	Type   *string `json:"type,omitempty"`
	Number *string `json:"number,omitempty"`
	ID     *string `json:"id,omitempty"`   // ID может отсутствовать
	Name   *string `json:"name,omitempty"` // Имя может отсутствовать
}

// Структура для очереди
type JSONResponseQueue struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// Структура для подразделения
type JSONResponseTeam struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// Структура для агента
type JSONResponseAgent struct {
	ID   *string `json:"id",omitempty`
	Name *string `json:"name,omitempty"`
}

// Структура для пользователя (агента)
type JSONResponseUser struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// Представляет структуру для хранения данных вызова
type CDRDB struct {
	CallID       *string    `db:"call_id"`
	ParentID     *string    `db:"parent_id"`
	CreatedAt    *time.Time `db:"created_at"`
	FromType     *string    `db:"from_type"`   //
	FromNumber   *string    `db:"from_number"` //
	ToType       *string    `db:"to_type"`     //
	ToNumber     *string    `db:"to_number"`   //
	Destination  *string    `db:"destination"`
	Direction    *string    `db:"direction"`
	Queue        *string    `db:"queue"`     //
	UserName     *string    `db:"user_name"` //
	Team         *string    `db:"team"`      //
	Agent        *string    `db:"agent"`     //
	Duration     *int       `db:"duration"`
	BillSec      *int       `db:"bill_sec"`
	TalkSec      *int       `db:"talk_sec"`
	HoldSec      *int       `db:"hold_sec"`
	AnsweredAt   *time.Time `db:"answered_at"`
	Cause        *string    `db:"cause"`
	SipCode      *int       `db:"sip_code"`
	HangupBy     *string    `db:"hangup_by"`
	HangupAt     *time.Time `db:"hangup_at"`
	BridgetAt    *time.Time `db:"bridged_at,omitempty"`
	HasChildren  *bool      `db:"has_children,omitempty"`
	TransferFrom *string    `db:"transfer_from,omitempty"`
	TransferTo   *string    `db:"transfer_to,omitempty"`
	WaitSec      *int       `db:"wait_sec,omitempty"`
	RecordFile   *string    `db:"record_file,omitempty"`
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

type AuthServiceResponse struct {
	Status *string                  `json:"status,omitempty"`
	Data   *AuthServiceResponseData `json:"data,omitempty"`
}

type AuthServiceResponseData struct {
	FirstName *string `json:"firstname,omitempty"`
	LastName  *string `json:"lastname,omitempty"`
}
