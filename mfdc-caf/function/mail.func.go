package function

import (
	"bytes"
	"caf/model"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"github.com/jmoiron/sqlx"
)

// SendEmail отправляет электронное письмо через SMTP сервер с TLS авторизацией.
func SendEmail(to []string, subject string, body string, contentType string) error {
	// Параметры подключения к SMTP
	smtpServer := config.MAIL.ServerAddr
	port := config.MAIL.ServerPort
	from := config.MAIL.AuthUser
	username := config.MAIL.AuthUser
	password := config.MAIL.AuthPassword

	// Настройка TLS конфигурации
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false, // Пропускать ли проверку подлинности сертификата сервера
		ServerName:         smtpServer,
	}

	// Получаем адрес SMTP сервера
	addr := fmt.Sprintf("%s:%s", smtpServer, port)

	// Устанавливаем соединение с SMTP сервером
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to the SMTP server: %v", err)
	}
	defer conn.Close()

	// Создаем новый SMTP клиент
	client, err := smtp.NewClient(conn, smtpServer)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %v", err)
	}

	// Аутентификация
	auth := smtp.PlainAuth("", username, password, smtpServer)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	// Устанавливаем отправителя
	if err := client.Mail(username); err != nil {
		return fmt.Errorf("failed to set sender: %v", err)
	}

	// Устанавливаем получателей
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to add recipient %s: %v", recipient, err)
		}
	}

	// Получаем почтовый писатель
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get writer: %v", err)
	}
	defer writer.Close()

	// Формируем тело сообщения
	var message []byte

	if contentType == "html" {
		message = []byte(fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-version: 1.0;\r\nContent-Type: text/html; charset='UTF-8'\r\n\r\n%s",
			from,
			to[0],
			subject,
			body,
		))
	} else {
		message = []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to[0], subject, body))
	}

	// Записываем сообщение в почтовый писатель
	if _, err := writer.Write(message); err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	// Завершаем соединение
	client.Quit()

	return nil
}

// Функция для форматирования времени
func formatTime(t time.Time) string {
	return t.Format("02.01.2006 15:04:05")
}

func FilteredNotify(db *sqlx.DB) error {

	// Получаем срез команд
	var teamsDB []model.TeamDB
	err := db.Select(&teamsDB, "SELECT * FROM caf.teams WHERE active = $1 AND filtration = $2", true, false)
	if err != nil {
		return fmt.Errorf("failed to get teams: %w", err)
	}

	// Создаем срез команд
	teams := make([]model.Team, len(teamsDB))
	for idx, team := range teamsDB {
		teams[idx].ID = team.ID
		teams[idx].Name = team.Name
		teams[idx].EMail = team.EMail
	}

	loc, err := time.LoadLocation(config.API.TimeZone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %s", err)
	}

	currentTime := time.Now().In(loc)
	fromDate := currentTime.Add(-1 * time.Hour)
	toDate := currentTime

	dateFormat := "2006-01-02 15:04:05.000 -07"
	from := fromDate.Format(dateFormat)
	to := toDate.Format(dateFormat)

	// Перебираем срез команд и ищем соответствие
	for _, team := range teams {

		var logs []model.Logs
		err = db.Select(&logs, "SELECT * FROM caf.logs WHERE team_id = $1 AND filtered = $2 AND sent = $3 AND created_at BETWEEN $4 AND $5", team.ID, false, false, from, to)
		if err != nil {
			return fmt.Errorf("failed to get logs: %s", err)
		}

		for i := range logs { // Используем индекс для изменения исходного среза
			log := &logs[i] // Получаем указатель на текущий лог

			if log.CreatedAt != nil {
				CreatedAtTz := log.CreatedAt.In(loc) // Применяем временную зону
				log.CreatedAt = &CreatedAtTz         // Обновляем указатель на новое время
			}

			_, err := db.Exec("UPDATE caf.logs SET sent = $1 WHERE id = $2", true, log.ID)
			if err != nil {
				return fmt.Errorf("failed to update logs for sent status: %w", err)
			}
		}

		// Если в логах есть отфильтрованные номера на последный период
		if len(logs) != 0 {
			tmpl := `<!DOCTYPE html>
					<html lang="en">
					<head>
						<meta charset="UTF-8">
						<meta name="viewport" content="width=device-width, initial-scale=1.0">
						<title>Filtered contacts</title>
					</head>
					<body>
						<h1>Отфильтрованные контакты "{{.TeamName}}" за период<br>
				{{.FromDate}} - {{.ToDate}}
				</h1>
						<table border="1">
							<tr>
								<th>Дата</th>
								<th>Номер</th>
								<th>Описание</th>
							</tr>
							{{range .Logs}}
							<tr>
								<td>{{formatTime .CreatedAt}}</td>
								<td>{{.Number}}</td>
								<td>{{.Description}}</td>
							</tr>
							{{end}}
						</table>
					</body>
					</html>`

			// Создаем новый шаблон и регистрируем функцию форматирования
			t, err := template.New("email").Funcs(template.FuncMap{
				"formatTime": formatTime,
			}).Parse(tmpl)
			if err != nil {
				return fmt.Errorf("failed to parse template: %s", err)
			}

			teamName := "Unknown"
			if team.Name != nil {
				teamName = *team.Name
			}

			// Создаем данные для шаблона
			emailData := model.EmailData{
				Logs:     logs,
				TeamName: teamName,
				FromDate: formatTime(fromDate),
				ToDate:   formatTime(toDate),
			}

			// Создаем буфер для хранения сгенерированного HTML
			var buf bytes.Buffer
			if err := t.Execute(&buf, emailData); err != nil {
				return fmt.Errorf("failed to execute template: %s", err)
			}

			// Получаем сгенерированный HTML как строку
			htmlContent := buf.String()

			// Отправляем почту
			subject := "MFDC filter info"

			// Преобразование списка email в срез строк
			EMail := config.MAIL.AuthUser
			if team.EMail != nil {
				EMail = *team.EMail
			}
			toAddr := strings.Split(EMail, ", ")

			// Удаление пробелов вокруг каждого email (если они есть)
			for i := range toAddr {
				toAddr[i] = strings.TrimSpace(toAddr[i])
			}

			OutLog.Println("Sending mail notification to ", toAddr)

			err = SendEmail(toAddr, subject, htmlContent, "html")
			if err != nil {
				return fmt.Errorf("failed to send mail: %s", err)
			}
		}
	}

	return nil
}
