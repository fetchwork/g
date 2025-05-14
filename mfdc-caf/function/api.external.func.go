package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIFetch выполняет HTTP-запрос с указанными параметрами.
func APIFetch(header string, key string, method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Устанавливаем reqBody только если метод не GET и не DELETE
	if jsonData != nil && method != http.MethodGet && method != http.MethodDelete {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)
		//OutLog.Println(string(body))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to create request: %w", err)
	}

	// Устанавливаем заголовки для доступа к API
	req.Header.Set(header, key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-From-Service", "MFDC")

	/*
		// Логируем информацию о запросе
		OutLog.Printf("Sending %s request to %s", method, url)
		OutLog.Printf("Headers: %v", req.Header)

		if reqBody != nil {
			bodyBytes, _ := io.ReadAll(reqBody)
			OutLog.Printf("Request Body: %s", bodyBytes)
		}
	*/

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to read response body: %w", err)
	}
	/*
		// Логируем статус ответа и тело
		OutLog.Printf("Response Status: %s", resp.Status)
		OutLog.Printf("Response Body: %s", body)
	*/

	// Возвращаем body ответа
	return body, resp.StatusCode, nil
}

func JRPCFetch(secretKey string, method string, url string, id string, params interface{}) ([]byte, error) {
	// Формируем JSON-RPC запрос
	requestBody := map[string]interface{}{
		"_system": map[string]interface{}{
			"service_security_key": secretKey,
		},
		"jsonrpc": "2.0",
		"id":      id, // Идентификатор запроса
		"method":  method,
		"params":  params,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal JSON-RPC request: %w", err)
	}

	//OutLog.Println(string(body))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Возвращаем body ответа
	return responseBody, nil
}
