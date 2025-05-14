package model

// Делаем типы со звёздочкой, чтобы нормально обрабатывать NULL значения в БД
type CallCheck struct {
	FromDate *string `json:"from_date,omitempty"`
	ToDate   *string `json:"to_date,omitempty"`
	Number   *string `json:"number,omitempty"`
}

type CheckResult struct {
	ExistsSuccess    bool `db:"success"`
	ExistsNotSuccess bool `db:"nosuccess"`
}
