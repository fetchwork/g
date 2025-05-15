package model

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// IntString представляет собой строку или целое число
type IntString struct {
	Value string
}

// Метод для десериализации IntString
func (is *IntString) UnmarshalJSON(data []byte) error {
	// Попробуем десериализовать как строку
	var strValue string
	if err := json.Unmarshal(data, &strValue); err == nil {
		is.Value = strValue
		return nil
	}

	// Если не удалось, пробуем десериализовать как целое число
	var intValue int
	if err := json.Unmarshal(data, &intValue); err == nil {
		is.Value = strconv.Itoa(intValue)
		return nil
	}

	return fmt.Errorf("failed to parse string or integer value: %s", data)
}

// Метод для сериализации IntString
func (is IntString) MarshalJSON() ([]byte, error) {
	return json.Marshal(is.Value)
}

// CommunicationType представляет тип коммуникации
type CommunicationType struct {
	ID   *IntString `json:"id,omitempty"`
	Name *string    `json:"name,omitempty"`
}

// Communication представляет коммуникацию
type Communication struct {
	Destination *string            `json:"destination,omitempty"`
	Type        *CommunicationType `json:"type,omitempty"`
}

// Timezone представляет информацию о временной зоне
type Timezone struct {
	ID   *IntString `json:"id,omitempty"`
	Name *string    `json:"name,omitempty"`
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
