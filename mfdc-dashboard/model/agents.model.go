package model

// Реализуем тип для среза Agents
type ByName []AgentsData

// Определяем методы для реализации sort.Interface
func (a ByName) Len() int {
	return len(a)
}

func (a ByName) Less(i, j int) bool {
	return a[i].UserName < a[j].UserName
}

func (a ByName) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// Channel представляет информацию о канале.
type Channel struct {
	Channel  string `json:"channel,omitempty"`
	State    string `json:"state,omitempty"`
	JoinedAt string `json:"joined_at,omitempty"`
}

// User представляет информацию о пользователе.
type User struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// AgentTeam представляет информацию о команде.
type AgentTeam struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Supervisor struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type Auditor struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// Item представляет информацию о каждом элементе в массиве items.
type Item struct {
	ID               string       `json:"id,omitempty"`
	User             User         `json:"user,omitempty"`
	Status           string       `json:"status,omitempty"`
	LastStatusChange string       `json:"last_status_change,omitempty"`
	ProgressiveCount int          `json:"progressive_count,omitempty"`
	Name             string       `json:"name,omitempty"`
	Channel          []Channel    `json:"channel,omitempty"`
	StatusDuration   string       `json:"status_duration,omitempty"`
	ChatCount        int          `json:"chat_count,omitempty"`
	Supervisor       []Supervisor `json:"supervisor,omitempty"`
	Team             AgentTeam    `json:"team,omitempty"`
	Auditor          []Auditor    `json:"auditor,omitempty"`
	Extension        string       `json:"extension,omitempty"`
}

// Response представляет корневую структуру JSON, содержащую массив items.
type Response struct {
	Items []Item `json:"items"`
}

type AgentsResult struct {
	Total  int      `json:"total"`
	Status string   `json:"status"`
	Data   []Agents `json:"data"`
}

type Agents struct {
	TeamName string       `json:"team_name"`
	Count    int          `json:"count"`
	Agents   []AgentsData `json:"agents"`
}

type AgentsData struct {
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
	Status           string `json:"status"`
	LastStatusChange string `json:"last_status_change"`
	State            string `json:"state"`
	LastStateChange  string `json:"last_state_change"`
	Extension        string `json:"extension"`
	//TeamName         string `json:"team_name"`
}

type RedisListAgents struct {
	Key      string `json:"key"`
	TeamName string `json:"team_name"`
}
