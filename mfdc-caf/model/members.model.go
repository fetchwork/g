package model

// CommunicationType представляет тип коммуникации
type CommunicationType struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// Communication представляет коммуникацию
type Communication struct {
	Destination *string            `json:"destination,omitempty"`
	Type        *CommunicationType `json:"type,omitempty"`
}

// Timezone представляет информацию о временной зоне
type Timezone struct {
	ID   *string `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
}

// Queue представляет очередь (пока пустая структура)
type Queue struct{}

// Data представляет основную структуру данных
type Members struct {
	ID             *string            `json:"id,omitempty"`
	Queue          *Queue             `json:"queue,omitempty"`
	Priority       *int               `json:"priority,omitempty"`
	Code           *int               `json:"code,omitempty"`
	CreatedAt      *string            `json:"created_at,omitempty"`
	Variables      *map[string]string `json:"variables,omitempty"` // динамические переменные
	Name           *string            `json:"name,omitempty"`
	Timezone       *Timezone          `json:"timezone,omitempty"`
	Communications *[]Communication   `json:"communications,omitempty"`
	MinOfferingAt  *string            `json:"min_offering_at,omitempty"`
}
