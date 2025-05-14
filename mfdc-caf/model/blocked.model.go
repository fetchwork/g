package model

type ReasonsStat struct {
	ID        int64  `db:"id"`
	NumID     int64  `db:"num_id"`
	Count     int    `db:"count"`
	SipCode   string `db:"sip_code"`
	SipReason string `db:"sip_reason"`
}

type JRPSResponse struct {
	ID      string             `json:"id"`
	JSONRPC string             `json:"jsonrpc"`
	Result  *string            `json:"result,omitempty"`
	Error   *JRPSResponseError `json:"error,omitempty"`
}

type JRPSResponseError struct {
	Code    *int                   `json:"code,omitempty"`
	Message *string                `json:"message,omitempty"`
	Data    *JRPSResponseErrorData `json:"data,omitempty"`
}

type JRPSResponseErrorData struct {
	Description *string `json:"description,omitempty"`
}
