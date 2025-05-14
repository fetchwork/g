package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"

	_ "github.com/jackc/pgx/stdlib"
	"github.com/rabbitmq/amqp091-go"
)

// Config структура для хранения конфигурации
type Config struct {
	RabbitMQ struct {
		URL         string `json:"url"`
		Exchange    string `json:"exchange"`
		Queue       string `json:"queue"`
		RoutingKey  string `json:"routing_key"`
		ConsumerKey string `json:"consumer_key"`
	} `json:"rabbitmq"`
	PostgreSQL struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DBName   string `json:"dbname" `
	} `json:"postgresql"`
}

type CDRParams struct { // Описание структуры сообщений
	Method     string `json:"method"`
	FromTag    string `json:"from_tag"`
	ToTag      string `json:"to_tag"`
	CallID     string `json:"callid"`
	SIPCode    string `json:"sip_code"`
	SIPReason  string `json:"sip_reason"`
	Time       int64  `json:"time"`
	SrcIP      string `json:"src_ip"`
	DstIP      string `json:"dst_ip"`
	CallerID   string `json:"callerid"`
	Callee     string `json:"callee"`
	Team       string `json:"bill"`
	Duration   int    `json:"duration,omitempty"`
	MSDuration int64  `json:"ms_duration,omitempty"`
	SetupTime  int64  `json:"setuptime,omitempty"`
	Created    int64  `json:"created"`
}

type CDRMessage struct {
	JSONRPC string    `json:"jsonrpc"`
	Method  string    `json:"method"`
	Params  CDRParams `json:"params"`
}

type ProvidersAddress struct {
	Id  int
	Pid int
	IP  string
}

type Routes struct {
	Rid         int
	Description string
	Prefix      string
	Cost        float64
	Step        int
	Pid         int
}

var (
	OutLog *log.Logger
	ErrLog *log.Logger
)

// Глобальные переменные для хранения конфигурации
var (
	config             Config
	configMutex        sync.RWMutex      // Мьютекс для безопасного доступа к конфигурации
	configReloadSignal = "config.reload" // Имя файла для сигнала перезагрузки
)

// Функция для проверки и загрузки конфигурации
func LoadConfig() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	// Проверка наличия файла billing.json
	if _, err := os.Stat("billing.json"); os.IsNotExist(err) {
		return errors.New("configuration file billing.json not found")
	}

	// Считывание конфигурации из файла
	data, err := os.ReadFile("billing.json")
	if err != nil {
		return err
	}

	// Парсинг JSON конфига
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	return nil
}

// Функция для получения текущей конфигурации
func GetConfig() Config {
	configMutex.RLock()
	defer configMutex.RUnlock()
	return config
}

// PGConnect подключается к PostgreSQL и возвращает указатель на базу данных и ошибку
func PGConnect() (*sqlx.DB, error) {
	config := GetConfig()

	// Подключение к PostgreSQL
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.PostgreSQL.Host,
		config.PostgreSQL.Port,
		config.PostgreSQL.User,
		config.PostgreSQL.Password,
		config.PostgreSQL.DBName)

	db, err := sqlx.Connect("pgx", psqlInfo)
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
		return nil, err // Возвращаем nil и ошибку
	}

	// Проверка соединения с базой данных
	if err := db.Ping(); err != nil {
		ErrLog.Printf("Failed to ping PostgreSQL: %s", err)
		return nil, err // Возвращаем nil и ошибку
	}

	return db, nil // Возвращаем указатель на базу данных и nil в качестве ошибки
}

func init() {
	// Настройка логгера для stdout
	OutLog = log.New(os.Stdout, "", log.LstdFlags)

	// Настройка логгера для stderr
	ErrLog = log.New(os.Stderr, "", log.LstdFlags)
}

func main() {

	// Загружаем конфигурацию при запуске
	if err := LoadConfig(); err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	// Подключение к PostgreSQL
	db, err := PGConnect()
	if err != nil {
		ErrLog.Printf("Failed to connect to PostgreSQL: %s", err)
	}
	defer db.Close()

	// Создаём структуру БД если нет
	err = CreateTables(db)
	if err != nil {
		ErrLog.Printf("Error creating structure tables: %v", err)
	}
	// Создание секции текущего месяца для таблицы calls
	err = CreateSectionCalls(db, "")
	if err != nil {
		ErrLog.Printf("Error creating current section for table calls: %v", err)
	}

	// Подключение к RabbitMQ
	var conn *amqp091.Connection
	var err_rmq error

	for {
		conn, err_rmq = amqp091.Dial(config.RabbitMQ.URL)
		if err_rmq != nil {
			ErrLog.Printf("Failed to connect to RabbitMQ: %s. Retrying in 5 seconds...", err_rmq)
			time.Sleep(5 * time.Second) // Ждем 5 секунд перед повторной попыткой
			continue
		}

		OutLog.Printf("Connected to RabbitMQ")
		break // Выход из цикла, если подключение успешно
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		ErrLog.Printf("Failed to open a channel: %s", err)
	}
	defer ch.Close()

	// Проверка наличия exchange
	err = ch.ExchangeDeclare(
		config.RabbitMQ.Exchange, // имя exchange
		"topic",                  // тип exchange
		true,                     // durable
		false,                    // auto-delete
		false,                    // internal
		false,                    // no-wait
		nil,                      // дополнительные аргументы
	)
	if err != nil {
		ErrLog.Printf("Failed to declare RabbitMQ exchange: %s", err)
	}

	// Проверка наличия очереди
	queue, err := ch.QueueDeclare(
		config.RabbitMQ.Queue, // имя очереди
		true,                  // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		nil,                   // дополнительные аргументы
	)
	if err != nil {
		ErrLog.Printf("Failed to declare RabbitMQ queue: %s", err)
	}

	// Создание биндинга между exchange и очередью
	err = ch.QueueBind(
		queue.Name,                 // имя очереди
		config.RabbitMQ.RoutingKey, // routing key
		config.RabbitMQ.Exchange,   // имя exchange
		false,                      // no-wait
		nil,                        // дополнительные аргументы
	)
	if err != nil {
		ErrLog.Printf("Failed to bind RabbitMQ queue: %s", err)
	}

	// Установка prefetch count
	err = ch.Qos(
		5,     // prefetch count
		0,     // prefetch size (0 не ограничивать)
		false, // глобальный prefetch
	)
	if err != nil {
		log.Printf("Failed to set QoS: %s", err)
	}

	// Подписка на очередь
	msgs, err := ch.Consume(
		config.RabbitMQ.Queue,       // имя очереди
		config.RabbitMQ.ConsumerKey, // consumer tag
		false,                       // auto-ack
		false,                       // exclusive
		false,                       // no-local
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		ErrLog.Printf("Failed to register a RabbitMQ consumer: %s", err)
	}

	var wg sync.WaitGroup
	// Создаем канал для сигналов
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	numWorkers := runtime.NumCPU()
	semaphore := make(chan struct{}, numWorkers)

	/*
		go func() {
			<-signalChan // Ждем получения сигнала
			log.Println("Received shutdown signal, shutting down...")
			cancel() // Отменяем контекст
		}()
	*/

	// Запускаем мониторинг изменения конфига в отдельной горутине
	wg.Add(1)
	go func() {
		defer wg.Done()
		MonitorConfigReload(ctx)
	}()

	// Запуск горутины для проверки первого числа месяца
	wg.Add(1)
	go func() {
		defer wg.Done()
		checkAndCreatePartition(ctx, db)
	}()

	OutLog.Println("Starting message processing loop...")

	//OutLog.Println(numWorkers, "CPU cores in system")
	// Запуск обработки сообщений
	go func() {
		defer wg.Done() // Убедитесь, что это defer вызывается в самом начале горутины
		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					OutLog.Println("Message channel closed!")
					return // Выходим из цикла, если канал закрыт
				}

				wg.Add(1)
				semaphore <- struct{}{} // Ограничиваем количество одновременно работающих горутин

				go func(d amqp091.Delivery) {
					defer wg.Done()
					defer func() { <-semaphore }() // Освобождаем место в семафоре

					// Шлём ACK что сообщение приняли
					if err := d.Ack(false); err != nil {
						ErrLog.Printf("Failed to ack message: %s", err)
					}

					var cdrMessage CDRMessage
					if err := json.Unmarshal(msg.Body, &cdrMessage); err != nil {
						ErrLog.Printf("Failed to parse JSON message: %s", err)
						return
					}

					// Преобразование unixtime в timestamp
					created := time.Unix(cdrMessage.Params.Created, 0)

					var rate float64
					var rid int
					var bill float64
					var method *int
					provider := ProvidersAddress{}

					if cdrMessage.Params.DstIP != "" && cdrMessage.Params.SrcIP != "" {
						pQuery := `SELECT paddr.pid, p.method FROM billing.providers_address AS paddr 
									LEFT JOIN billing.providers AS p ON paddr.pid=p.pid
									WHERE paddr.ip = $1`
						provider_row := db.QueryRow(pQuery, cdrMessage.Params.DstIP)
						err = provider_row.Scan(&provider.Pid, &method)

						if err != nil {
							ErrLog.Printf("Failed to get provider ID by %s: %s", cdrMessage.Params.DstIP, err)
							return
						} else {
							prefixes_slice := []string{} // Объявляем срез для префиксов
							routes_slice := []Routes{}
							err := db.Select(&routes_slice, "SELECT * FROM billing.routes WHERE pid=$1 ORDER BY LENGTH(prefix) DESC", provider.Pid)
							if err != nil {
								ErrLog.Printf("Failed to fetch routes: %s", err)
							}

							// Извлечение префиксов из маршрутов и добавление в срез с префиксами
							for _, route := range routes_slice {
								prefixes_slice = append(prefixes_slice, route.Prefix)
							}
							// Проходим по всем префиксам в срезе
							var dstPrefix string
							for _, prefix := range prefixes_slice {
								if strings.HasPrefix(cdrMessage.Params.Callee, prefix) { // Проверяем, начинается ли callee с префикса в текущей итерации
									dstPrefix = prefix // Если да, сохраняем префикс
									break              // Выходим из цикла, так как нашли первый подходящий префикс
								}
							}

							if dstPrefix != "" {
								for _, route := range routes_slice {
									if route.Prefix == dstPrefix {

										rate = route.Cost
										rid = route.Rid
										if method != nil {
											if *method == 2 {
												// 2 - метод
												// Минуты = (Секунды / 60) с округлением вниз до двух знаков
												// Стоимость = Минуты × Тариф (с округлением вверх до 2 знаков)
												// Если третий знак после запятой "0", округление не применяется

												//Количество шагов = Время разговора / Шаг тарификации
												steps := int(math.Ceil(float64(cdrMessage.Params.Duration) / float64(route.Step)))

												// Переводим шагов в минуты с округлением вниз до двух знаков
												minutes := math.Floor(float64(steps)/60*100) / 100

												// Рассчитываем стоимость
												billMiddle := minutes * route.Cost

												// Округляем стоимость вверх до двух знаков с учетом исключения
												bill = math.Ceil(billMiddle*100) / 100
											} else { // 1 - обычный метод
												// Определяем количество шагов (шаг тарификации) округляем до большего
												//Количество шагов = Время разговора / Шаг тарификации
												steps := int(math.Ceil(float64(cdrMessage.Params.Duration) / float64(route.Step)))

												// Переводим количество шагов в минуты
												totalMinutes := float64(steps * route.Step)

												// Рассчитываем стоимость
												bill = (route.Cost / 60.0) * totalMinutes

												// Округляем до двух знаков после запятой
												// Округления по математическим правилам (среднее округление)
												bill = math.Round(bill*100) / 100
											}
										} else {
											ErrLog.Printf("Calc method is null for number %s in provider ID = %d", cdrMessage.Params.Callee, provider.Pid)
											return
										}
									}
								}
							} else {
								ErrLog.Printf("Prefix not found for number %s in provider ID = %d", cdrMessage.Params.Callee, provider.Pid) // Если префикс не найден
							}
						}
					} else {
						if strings.HasPrefix(cdrMessage.Params.SIPCode, "4") { // пропускаем вставку если нет IP и ответы 400е
							return
						} else {
							ErrLog.Printf("IP is empty date=%v message=%v", created, cdrMessage)
						}
					}

					// Добавляем вычисление даты завершения звонка
					var endAt time.Time
					if cdrMessage.Params.Duration != 0 {
						endAtUnixTime := cdrMessage.Params.Created + int64(cdrMessage.Params.Duration)
						// Преобразование unixtime в timestamp
						endAt = time.Unix(endAtUnixTime, 0)
					} else {
						endAt = created
					}

					// Обработка в зависимости от метода opensips ACC-модуля: E_ACC_CDR или E_ACC_MISSED_EVENT
					switch cdrMessage.Method {

					case "E_ACC_CDR":
						// Сохранение данных в PostgreSQL для E_ACC_CDR
						_, err = db.NamedExec(`INSERT INTO billing.calls (callid, sip_code, sip_reason, callerid, callee, duration, created, end_at, pid, rate, rid, bill, team) VALUES (:callid, :sip_code, :sip_reason, :callerid, :callee, :duration, :created, :end_at, :pid, :rate, :rid, :bill, :team)`,
							map[string]interface{}{
								"callid":     cdrMessage.Params.CallID,
								"sip_code":   cdrMessage.Params.SIPCode,
								"sip_reason": cdrMessage.Params.SIPReason,
								"callerid":   cdrMessage.Params.CallerID,
								"callee":     cdrMessage.Params.Callee,
								"duration":   cdrMessage.Params.Duration,
								"created":    created,
								"end_at":     endAt,
								"pid":        provider.Pid,
								"rate":       rate,
								"rid":        rid,
								"bill":       bill,
								"team":       cdrMessage.Params.Team,
							})

					case "E_ACC_MISSED_EVENT":
						// Сохранение данных в PostgreSQL для E_ACC_MISSED_EVENT
						_, err = db.NamedExec(`INSERT INTO billing.calls (callid, sip_code, sip_reason, callerid, callee, created, end_at, pid, rate, rid, team) VALUES (:callid, :sip_code, :sip_reason, :callerid, :callee, :created, :end_at, :pid, :rate, :rid, :team)`,
							map[string]interface{}{
								"callid":     cdrMessage.Params.CallID,
								"sip_code":   cdrMessage.Params.SIPCode,
								"sip_reason": cdrMessage.Params.SIPReason,
								"callerid":   cdrMessage.Params.CallerID,
								"callee":     cdrMessage.Params.Callee,
								"created":    created,
								"end_at":     endAt,
								"pid":        provider.Pid,
								"rate":       rate,
								"rid":        rid,
								"team":       cdrMessage.Params.Team,
							})

					default: // Если вдруг прилетело что то незнакомое в методе, то пропускаем текущую итерацию
						ErrLog.Printf("Received unknown method from RabbitMQ: %s", cdrMessage.Method)
						return
					}

					if err != nil {
						ErrLog.Printf("Failed to insert into database: %s; body: %s", err, string(msg.Body))
						return
					}

					//OutLog.Printf("Insert message: %v", cdrMessage)
				}(msg)
			case <-ctx.Done():
				OutLog.Println("Stopping receive data...")
				return // Выходим из цикла, если получен сигнал завершения
			default:
				continue // Если нет сообщений, продолжаем ждать
			}
		}
	}()
	<-signalChan // Ждем получения сигнала
	OutLog.Println("Received shutdown signal, shutting down...")
	cancel() // Отменяем контекст

	wg.Wait() // Ждем завершения всех горутин перед выходом
	OutLog.Println("Billing service is halted")
}

func MonitorConfigReload(ctx context.Context) {
	for {
		select {
		case <-time.After(5 * time.Second): // Пауза между проверками
			// Проверяем наличие файла сигнала
			if _, err := os.Stat(configReloadSignal); err == nil {
				err := LoadConfig() // вызываем LoadConfig() для обновления
				if err != nil {
					return
				}
				OutLog.Println("Configuration reloaded")
				// Удаляем файл после обработки сигнала
				os.Remove(configReloadSignal)
			}
		case <-ctx.Done(): // Если контекст завершен
			OutLog.Println("Stopping config reload monitoring...")
			return // Завершаем выполнение функции
		}
	}
}

const (
	createCallsTableSQL = `CREATE TABLE IF NOT EXISTS billing.calls (
			cid bigserial NOT NULL,
			pid int4 NOT NULL,
			callid varchar NULL,
			created timestamptz NOT NULL,
			callerid varchar NULL,
			callee varchar NULL,
			duration int4 DEFAULT 0 NOT NULL,
			rate numeric DEFAULT 0 NOT NULL,
			bill numeric DEFAULT 0 NOT NULL,
			rid int4 NULL,
			sip_code varchar NULL,
			sip_reason varchar NULL,
			team varchar NULL,
			end_at timestamptz NULL,
			CONSTRAINT calls_pk PRIMARY KEY (cid, created)
		)
		PARTITION BY RANGE (created);`

	createProvidersTableSQL = `CREATE TABLE IF NOT EXISTS billing.providers (
        pid serial4 NOT NULL,
        "name" varchar NULL,
        "description" varchar NULL,
		"method" int4 NULL,
        CONSTRAINT providers_pk PRIMARY KEY (pid)
    );`

	createProvidersIndexesSQL = `CREATE UNIQUE INDEX IF NOT EXISTS providers_name_idx ON billing.providers USING btree (name);`

	createProvidersAddressTableSQL = `CREATE TABLE IF NOT EXISTS billing.providers_address (
        id serial4 NOT NULL,
        pid int4 NULL,
        ip inet NOT NULL,
        CONSTRAINT pa_pk PRIMARY KEY (id)
    );`

	createRoutesTableSQL = `CREATE TABLE IF NOT EXISTS billing.routes (
        rid serial4 NOT NULL,
        description varchar NOT NULL,
        prefix varchar NOT NULL,
        "cost" numeric NOT NULL,
        step int4 NULL,
        pid int4 NOT NULL,
        CONSTRAINT routes_pk PRIMARY KEY (rid)
    );`

	createRoutesIndexesSQL = `CREATE INDEX IF NOT EXISTS routes_pid_idx ON billing.routes USING btree (pid);
    CREATE INDEX IF NOT EXISTS routes_prefix_idx ON billing.routes USING btree (prefix);`

	createSumTableSQL = `CREATE TABLE IF NOT EXISTS billing.sum (
		sid bigserial NOT NULL,
		pid int4 NULL,
		provider_name varchar NULL,
		created timestamptz NULL,
		talk_minutes numeric NULL,
		bill_summ numeric NULL
	);`

	createSumTableIndexesSQL = `CREATE INDEX IF NOT EXISTS sum_created_idx ON billing.sum USING btree (created);
	CREATE INDEX IF NOT EXISTS sum_pid_idx ON billing.sum USING btree (pid);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createSumTableSQL)
	if err != nil {
		return err
	}
	_, err = db.Exec(createSumTableIndexesSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createCallsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createProvidersTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createProvidersIndexesSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createProvidersAddressTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createRoutesTableSQL)
	if err != nil {
		return err
	}

	// Создание индексов для таблицы routes
	_, err = db.Exec(createRoutesIndexesSQL)
	return err
}

func checkAndCreatePartition(ctx context.Context, db *sqlx.DB) {
	for {
		select {
		case <-ctx.Done():
			// Если контекст отменен, выходим из функции
			OutLog.Println("Stopping create partition table...")
			return
		default:
			now := time.Now()

			// Проверка, является ли сегодня 1 число месяца
			if now.Day() == 1 {
				// Создаём секционную таблицу для calls
				err := CreateSectionCalls(db, "future")
				if err != nil {
					OutLog.Printf("Error creating tables: %v", err)
				} else {
					OutLog.Println("Create a section of the partition has been started")
				}

				// Ждем до следующего первого числа месяца
				nextMonth := now.Month() + 1
				nextYear := now.Year()
				if nextMonth > 12 {
					nextMonth = 1
					nextYear++
				}
				nextFirst := time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, time.UTC)

				// Вычисляем время ожидания до следующего первого числа месяца
				time.Sleep(time.Until(nextFirst))
			} else {
				// Ждем сутки перед следующей проверкой
				time.Sleep(24 * time.Hour)
			}
		}
	}
}

func CreateSectionCalls(db *sqlx.DB, create string) error {
	now := time.Now()
	var partitionTableName string
	var startDate string
	var endDate string

	// Если в функции аргумент future то формируем имя таблицы на следующий месяц
	if create == "future" {
		// Получаем следующий месяц и год
		nextMonth := now.Month() + 1
		nextYear := now.Year()

		// Обработка перехода на следующий год
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}

		// Формируем имя секционной таблицы
		partitionTableName = fmt.Sprintf("calls_%02d_%d", nextMonth, nextYear)

		// Форматируем даты
		startDate = fmt.Sprintf("%d-%02d-01 00:00:00.001", nextYear, nextMonth)

		// Переход к следующему месяцу для endDate
		endMonth := nextMonth + 1
		if endMonth > 12 {
			endMonth = 1
			nextYear++
		}

		endDate = fmt.Sprintf("%d-%02d-01 00:00:00.000", nextYear, endMonth)
	} else {
		// Получаем текущий месяц и год
		month := now.Month()
		year := now.Year()

		// Формируем имя секционной таблицы
		partitionTableName = fmt.Sprintf("calls_%02d_%d", month, year)

		// Форматируем даты
		startDate = fmt.Sprintf("%d-%02d-01 00:00:00.001", year, month)

		// Обработка перехода на следующий месяц
		nextMonth := int(month) + 1
		nextYear := year
		if nextMonth > 12 {
			nextMonth = 1
			nextYear++
		}

		endDate = fmt.Sprintf("%d-%02d-01 00:00:00.000", nextYear, nextMonth)
	}

	// Создание SQL-запроса
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS billing.%s PARTITION OF billing.calls FOR VALUES FROM ('%s') TO ('%s')",
		partitionTableName, startDate, endDate)

	// Выполнение запроса
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// SQL-запросы для создания индексов
	indexSQLs := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_callee_idx ON billing.%s USING btree (callee);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_callerid_idx ON billing.%s USING btree (callerid);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_created_idx ON billing.%s USING btree (created);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_end_at_idx ON billing.%s USING btree (end_at);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_idx ON billing.%s USING btree (cid, created);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_pid_idx ON billing.%s USING btree (pid);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_rid_idx ON billing.%s USING btree (rid);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_sip_code_idx ON billing.%s USING btree (sip_code);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_sip_code_created_idx ON billing.%s USING btree (created, sip_code);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_team_idx ON billing.%s USING btree (team);", partitionTableName, partitionTableName),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s_duration_idx ON billing.%s USING btree (duration);", partitionTableName, partitionTableName),
	}

	// Выполнение запросов на создание индексов
	for _, indexSQL := range indexSQLs {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}

	return nil
}
