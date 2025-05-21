package model

type ErrorResponse struct {
	Error   string `json:"error"`   // Техническая ошибка
	Message string `json:"message"` // Человекочитаемое описание
	Status  string `json:"status"`  // Всегда "failed"
}

func NewErrorResponse(message, err string) ErrorResponse {
	return ErrorResponse{
		Error:   err,
		Message: message,
		Status:  "failed",
	}
}
