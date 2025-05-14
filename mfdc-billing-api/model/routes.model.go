package model

type Routes struct {
	RID         int     `json:"rid"`
	Description string  `json:"description"`
	Prefix      string  `json:"prefix"`
	Cost        float64 `json:"cost"`
	Step        int     `json:"step"`
	Pid         int     `json:"pid"`
	Provider    string  `db:"provider" json:"provider,omitempty"`
}

type AddRoute struct {
	Description *string  `json:"description,omitempty"`
	Prefix      *string  `json:"prefix,omitempty"`
	Cost        *float64 `json:"cost,omitempty"`
	Step        *int     `json:"step,omitempty"`
	Pid         *int     `json:"pid,omitempty"`
}

type DeleteReply struct {
	Status string `json:"status"`
}
