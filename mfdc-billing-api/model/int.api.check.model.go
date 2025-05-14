package model

// Делаем типы со звёздочкой, чтобы нормально обрабатывать NULL значения в БД
type CallCheck struct {
	FromDate *string `json:"from_date,omitempty"`
	ToDate   *string `json:"to_date,omitempty"`
	Number   *string `json:"number,omitempty"`
}

type CheckResult struct {
	OK        bool    `db:"ok" json:"ok"`
	Exists    bool    `db:"exists" json:"exists"`
	Count     *int    `db:"count" json:"count,omitempty"`
	SipCode   *string `db:"sip_code" json:"sip_code,omitempty"`
	SipReason *string `db:"sip_reason" json:"sip_reason,omitempty"`
}
