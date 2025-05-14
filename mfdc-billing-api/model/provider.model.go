package model

type ProvidersAddress struct {
	IPs []string `json:"address"`
}

type Providers struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Method      *int             `json:"method,omitempty"`
	Description *string          `json:"description,omitempty"`
	IP          ProvidersAddress `json:"ip"`
}

type ProvidernoID struct {
	Name        string  `json:"name"`
	Method      *int    `json:"method,omitempty"`
	Description *string `json:"description,omitempty"`
	IP          *IPInfo `json:"ip"`
}

type IPInfo struct {
	Address []string `json:"address"`
}

type ProvidersOnly struct {
	PID         int    `json:"pid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Method      int    `json:"method,omitempty"`
}

type ProvidersAddressAdd struct {
	IP string `json:"ip"`
}

type ProviderDelete struct {
	Reload string `json:"delete"`
}

type ProviderEdit struct {
	Name        string  `json:"name"`
	Method      *int    `json:"method,omitempty"`
	Description *string `json:"description,omitempty"`
	IPs         struct {
		Address []string `json:"address"`
	} `json:"ip"`
}

type AddProviderReply struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type EditProviderReply struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type DeleteAddressReply struct {
	ID      string `json:"provider_id"`
	Address string `json:"ip"`
	Status  string `json:"status"`
	Message string `json:"message"`
}
