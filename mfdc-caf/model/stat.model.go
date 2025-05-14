package model

/*
type TempStatDB struct {
	ID          int64      `db:"id" json:"id"`
	JoinedAt    *time.Time `db:"joined_at" json:"joined_at,omitempty"`
	BridgetAt   *time.Time `db:"bridged_at" json:"bridged_at,omitempty"`
	Destination string     `db:"destination" json:"destination,omitempty"`
	Result      string     `db:"result" json:"result,omitempty"`
}

type StatResponse struct {
	Next  bool                 `json:"next"`
	Items []StatResponseDetail `json:"items"`
}

type StatResponseDetail struct {
	JoinedAt    *string                 `json:"joined_at,omitempty"`
	BridgetAt   *string                 `json:"bridged_at,omitempty"`
	Destination StatResponseDestination `json:"destination,omitempty"`
	Result      string                  `json:"result,omitempty"`
	Queue       StatResponseQueue       `json:"queue,omitempty"`
}


type StatResponseDestination struct {
	Destination string `json:"destination,omitempty"`
}

type StatResponseQueue struct {
	ID string `json:"id,omitempty"`
}
*/

type StatResponseAPI struct {
	Status *string        `json:"status,omitempty"`
	Data   *[]CheckResult `json:"data,omitempty"`
}
type CheckResult struct {
	OK        *bool   `json:"ok,omitempty"`
	Exists    *bool   `json:"exists,omitempty"`
	Count     *int    `json:"count,omitempty"`
	SipCode   *string `json:"sip_code,omitempty"`
	SipReason *string `json:"sip_reason,omitempty"`
}
