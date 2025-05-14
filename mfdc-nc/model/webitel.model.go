package model

type WebitelError struct {
	ID     *string `json:"id,omitempty"`
	Code   *int    `json:"code,omitempty"`
	Detail *string `json:"detail,omitempty"`
	Status *string `json:"status,omitempty"`
}
