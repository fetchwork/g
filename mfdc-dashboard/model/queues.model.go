package model

type ByQueueName []Calls

// Определяем методы для реализации sort.Interface
func (a ByQueueName) Len() int {
	return len(a)
}

func (a ByQueueName) Less(i, j int) bool {
	// Проверяем на nil перед разыменованием
	if a[i].QueueName == nil && a[j].QueueName == nil {
		return false
	} else if a[i].QueueName == nil {
		return true // если a[i] nil, он меньше
	} else if a[j].QueueName == nil {
		return false // если a[j] nil, он меньше
	}
	return *a[i].QueueName < *a[j].QueueName // разыменовываем указатели для сравнения
}

func (a ByQueueName) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

type Calls struct {
	Calls     *int    `json:"calls,omitempty"`
	QueueName *string `json:"queue_name,omitempty"`
}
