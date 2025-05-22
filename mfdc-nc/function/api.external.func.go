package function

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"nc/model"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
)

func WebitelAPIFetch(method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Устанавливаем reqBody только если метод не GET и не DELETE
	if jsonData != nil && method != http.MethodGet && method != http.MethodDelete {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("Failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.API_Webitel.Header, config.API_Webitel.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to read response body: %w", err)
	}

	// Возвращаем body ответа
	return body, resp.StatusCode, nil
}

func VCAPIFetch(method, url string, jsonData interface{}) ([]byte, int, error) {
	var reqBody io.Reader

	// Устанавливаем reqBody только если метод не GET и не DELETE
	if jsonData != nil && method != http.MethodGet && method != http.MethodDelete {
		body, err := json.Marshal(jsonData)
		if err != nil {
			return nil, 500, fmt.Errorf("Failed to marshal JSON data: %w", err)
		}
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to create request: %w", err)
	}

	// Устанавливаем заголовки для доступа к API Webitel
	req.Header.Set(config.VC_API.Header, config.VC_API.Key)
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to read response body: %w", err)
	}

	// Возвращаем body ответа
	return body, resp.StatusCode, nil
}

func putToWebitel(resource_id int, number string) error {
	// Преоразуем из int в string
	resourceIDStr := strconv.Itoa(resource_id)

	// Создаем новый запрос на очистку ресурса
	url := fmt.Sprintf("%s/call_center/resources/%s/display", config.API_Webitel.URL, resourceIDStr)

	// Выполняем очистку номеров в ресурсе
	body, statusCode, err := WebitelAPIFetch("DELETE", url, "")
	if err != nil {
		return fmt.Errorf("failed to request for clearing resource: %w, status code: %d", err, statusCode)
	}

	var response model.WebitelError
	err = json.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Логируем тело ответа для диагностики
	//ErrLog.Println("Response body:", string(body))

	// no numbers with given filters found так Webitel отвечает если номеров в ресурсе уже нет
	if statusCode != http.StatusOK && response.Detail != nil && *response.Detail != "no numbers with given filters found" {
		return fmt.Errorf("failed to clear resource, status code: %d", statusCode)
	}

	// Создаем новый запрос на добавление номера
	url = fmt.Sprintf("%s/call_center/resources/%s/display", config.API_Webitel.URL, resourceIDStr)

	// Формируем тело запроса на добавление номера в ресурс Webitel
	requestBody := map[string]interface{}{
		"display": number,
	}

	// Добавляем номер в ресурс
	_, statusCode, err = WebitelAPIFetch("POST", url, requestBody)
	if err != nil {
		return fmt.Errorf("failed to add number to resource: %w, status code: %d", err, statusCode)
	}

	// Проверяем статус код после добавления номера
	if statusCode != http.StatusOK {
		return fmt.Errorf("failed to add number to resource, status code: %d", statusCode)
	}

	return nil
}

func getVendorByID(db *sqlx.DB, vendorID int) (vendorName string, err error) {
	err = db.Get(&vendorName, "SELECT name FROM nc.vendors WHERE id=$1", vendorID)
	if err != nil {
		return "", err
	}
	return vendorName, nil
}
func getTeamByID(db *sqlx.DB, teamID int) (teamName string, err error) {
	err = db.Get(&teamName, "SELECT name FROM nc.teams WHERE id=$1", teamID)
	if err != nil {
		return "", err
	}
	return teamName, nil
}

func getVendorIDByName(db *sqlx.DB, vendorName string) (vendorID int, err error) {
	err = db.Get(&vendorID, "SELECT id FROM nc.vendors WHERE name=$1", vendorName)
	if err != nil {
		return 0, err
	}
	return vendorID, nil
}

func getTeamIDByName(db *sqlx.DB, teamName string) (teamID int, err error) {
	err = db.Get(&teamID, "SELECT id FROM nc.teams WHERE name=$1", teamName)
	if err != nil {
		if err == sql.ErrNoRows {
			return 99999, nil
		}
		return 0, err
	}
	return teamID, nil
}

// Получения статуса актуального ресурса
func SyncActualVendor(db *sqlx.DB) error {
	url := fmt.Sprintf("%s/list?page=1&limit=100", config.VC_API.URL)

	body, statusCode, err := VCAPIFetch("GET", url, "")
	if err != nil {
		return fmt.Errorf("failed to read response body: %w, status code: %d", err, statusCode)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("bad response from VC API, status code: %d", statusCode)
	}

	var response model.VCResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if len(response.Data) == 0 {
		return fmt.Errorf("no items found in JSON response")
	}

	for _, resource := range response.Data {
		if resource.Team == nil || resource.Vendor == nil || resource.Actual == nil {
			continue
		}

		if *resource.Actual {
			teamID, err := getTeamIDByName(db, *resource.Team)
			if teamID == 99999 {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to get TeamID for team '%s': %w", *resource.Team, err)
			}

			vendorID, err := getVendorIDByName(db, *resource.Vendor)
			if err != nil {
				return fmt.Errorf("failed to get VendorID for vendor '%s': %w", *resource.Vendor, err)
			}

			_, err = db.Exec("UPDATE nc.teams SET actual_vendor_id = $1 WHERE id = $2", vendorID, teamID)
			if err != nil {
				return fmt.Errorf("failed to update actual VendorID for team ID '%d': %w", teamID, err)
			}
		}
	}

	return nil
}

// Получения статуса актуального вндора для команды
func CheckActualVendor(db *sqlx.DB, teamID int, vendorID int) (actual bool, err error) {
	err = db.Get(&actual, "SELECT EXISTS (SELECT 1 FROM nc.teams WHERE id = $1 AND (actual_vendor_id = $2 OR actual_vendor_id IS NULL))", teamID, vendorID)
	if err != nil {
		return false, fmt.Errorf("failed to check actual vendor for team ID %d and vendor ID %d: %w", teamID, vendorID, err)
	}

	return actual, nil
}
