package function

import (
	"context"
	"dashboard/model"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// Создаем контекст для работы с Redis
var redis_ctx = context.Background()

// Функция для инициализации клиента Redis
func RedisClient() *redis.Client {
	// Конвертируем порт из int в string
	redisPort := strconv.Itoa(config.Redis.Port)
	// Создаём коннект
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Redis.Host + ":" + redisPort,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

	return rdb
}

// Функция для добавления данных в Redis с указанием времени жизни
func AddToRedisSET(rdb *redis.Client, key string, value string, expiration time.Duration) error {
	err := rdb.Set(redis_ctx, key, value, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

// Функция для чтения данных из Redis
func GetFromRedisSET(rdb *redis.Client, key string) (string, error) {
	val, err := rdb.Get(redis_ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

// Функция для удаления данных из Redis
func DeleteFromRedis(rdb *redis.Client, key string) error {
	err := rdb.Del(redis_ctx, key).Err()
	if err != nil {
		return err
	}
	return nil
}

// Функция для чтения всех значений по ключу HSET
func GetFromRedisALL(rdb *redis.Client, key string) (map[string]string, error) {
	// Проверяем, существует ли ключ
	exists, err := rdb.Exists(redis_ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Если ключ не существует, возвращаем ошибку или пустую карту
	if exists == 0 {
		return map[string]string{}, fmt.Errorf("key %s does not exist", key)
	}

	// Получаем все поля и значения по ключу
	result, err := rdb.HGetAll(redis_ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func AddToRedisHSET(rdb *redis.Client, key string, collectionKey string, teamName string, expiry time.Duration, fieldsAndValues ...interface{}) error {
	// Проверяем, что количество аргументов четное
	if len(fieldsAndValues)%2 != 0 {
		return fmt.Errorf("неверное количество аргументов: должно быть четное число (пара 'поле-значение')")
	}

	// Устанавливаем поля и значения в хеш
	if err := rdb.HSet(redis_ctx, key, fieldsAndValues).Err(); err != nil {
		return fmt.Errorf("ошибка при установке значений в хеш: %w", err)
	}

	if err := rdb.Expire(redis_ctx, key, expiry).Err(); err != nil {
		return fmt.Errorf("ошибка при установке времени жизни для агента: %w", err)
	}

	var itemJSON []byte
	var err error

	// Создаем объект Item с ключом и именем команды
	item := model.RedisListAgents{Key: key, TeamName: teamName}
	itemJSON, err = json.Marshal(item)
	if err != nil {
		return fmt.Errorf("ошибка при сериализации данных: %w", err)
	}

	// Добавляем сериализованный объект в набор
	if err := rdb.SAdd(redis_ctx, collectionKey, itemJSON).Err(); err != nil {
		return fmt.Errorf("ошибка при добавлении ключа в коллекцию: %w", err)
	}

	return nil
}

func AddToRedisHSETByQueue(rdb *redis.Client, queueName string, queueID string, expiry time.Duration, fieldsAndValues ...interface{}) error {
	// Проверяем, что количество аргументов четное
	if len(fieldsAndValues)%2 != 0 {
		return fmt.Errorf("неверное количество аргументов: должно быть четное число (пара 'поле-значение')")
	}

	// Устанавливаем поля и значения в хеш
	if err := rdb.HSet(redis_ctx, queueName, fieldsAndValues).Err(); err != nil {
		return fmt.Errorf("ошибка при установке значений в хеш: %w", err)
	}

	if err := rdb.Expire(redis_ctx, queueName, expiry).Err(); err != nil {
		return fmt.Errorf("ошибка при установке времени жизни для агента: %w", err)
	}

	// Добавляем объект в набор
	if err := rdb.SAdd(redis_ctx, queueID, queueName).Err(); err != nil {
		return fmt.Errorf("ошибка при добавлении ключа в коллекцию: %w", err)
	}

	return nil
}

func SetExpireToTeamIDList(rdb *redis.Client, teamID string, expiry time.Duration) error {

	if err := rdb.Expire(redis_ctx, teamID, expiry).Err(); err != nil {
		return fmt.Errorf("ошибка при установке времени жизни для коллекции: %w", err)
	}

	return nil
}

// Функция для получения всех агентов из коллекции
func GetAllAgents(rdb *redis.Client, collectionKey string) (*model.Agents, error) {
	// Проверяем, существует ли ключ
	exists, err := rdb.Exists(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}

	// Если ключ не существует, возвращаем пустую структуру
	if exists == 0 {
		return &model.Agents{}, nil
	}

	var agentsData []model.AgentsData

	// Получаем все ключи из коллекции
	keys, err := rdb.SMembers(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}
	// Десериализуем JSON-строку в структуру Item
	var item model.RedisListAgents

	// Извлекаем данные о каждом человеке
	for _, keyJSON := range keys {
		if err := json.Unmarshal([]byte(keyJSON), &item); err != nil {
			return nil, fmt.Errorf("ошибка при десериализации данных: %w", err)
		}

		personData, err := rdb.HGetAll(redis_ctx, item.Key).Result()
		if err != nil {
			return nil, err
		}

		// Создаем структуру AgentsData и заполняем ее данными
		person := model.AgentsData{
			UserID:           personData["user_id"],
			UserName:         personData["user_name"],
			Status:           personData["status"],
			LastStatusChange: personData["last_status_change"],
			State:            personData["state"],
			LastStateChange:  personData["last_state_change"],
			Extension:        personData["extension"],
		}

		agentsData = append(agentsData, person)
	}

	// Создаем и возвращаем структуру Agents с именем команды и данными агентов
	return &model.Agents{
		TeamName: item.TeamName, // Здесь предполагается, что у вас есть доступ к имени команды из последнего обработанного элемента
		Agents:   agentsData,
	}, nil
}

func GetAllCalls(rdb *redis.Client, collectionKey string) (*[]model.Calls, error) {
	// Проверяем, существует ли ключ
	exists, err := rdb.Exists(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}

	// Если ключ не существует, возвращаем пустую структуру
	if exists == 0 {
		return &[]model.Calls{}, nil
	}

	var callsData []model.Calls

	// Получаем все ключи из коллекции
	keys, err := rdb.SMembers(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}

	// Извлекаем данные по каждой очереди
	for _, queueName := range keys {
		queueData, err := rdb.HGetAll(redis_ctx, queueName).Result()
		if err != nil {
			return nil, err
		}

		// Проверяем наличие ключей в queueData
		callsValueStr, callsExists := queueData["calls"]
		queueNameValue, nameExists := queueData["queue_name"]

		if !callsExists || !nameExists {
			continue // Пропускаем итерацию, если ключи отсутствуют
		}

		// Преобразуем строку в int
		callsValue, err := strconv.Atoi(callsValueStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка преобразования calls: %w", err)
		}

		// Создаем структуру Calls и заполняем ее данными
		calls := model.Calls{
			Calls:     &callsValue,
			QueueName: &queueNameValue,
		}

		callsData = append(callsData, calls)
	}

	// Возвращаем срез структуры Calls
	return &callsData, nil
}

func GetAllSpins(rdb *redis.Client, collectionKey string) (*[]model.QueueSpin, error) {
	// Проверяем, существует ли ключ
	exists, err := rdb.Exists(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}

	// Если ключ не существует, возвращаем пустую структуру
	if exists == 0 {
		return &[]model.QueueSpin{}, nil
	}

	var callsData []model.QueueSpin

	// Получаем все ключи из коллекции
	keys, err := rdb.SMembers(redis_ctx, collectionKey).Result()
	if err != nil {
		return nil, err
	}

	// Извлекаем данные по каждой очереди
	for _, queueName := range keys {
		queueData, err := rdb.HGetAll(redis_ctx, queueName).Result()
		if err != nil {
			return nil, err
		}

		// Проверяем наличие ключей в queueData
		spinsValueStr, spinsExists := queueData["spin"]
		queueNameValue, nameExists := queueData["queue_name"]

		if !spinsExists || !nameExists {
			continue // Пропускаем итерацию, если ключи отсутствуют
		}

		// Преобразуем строку в int
		spinsValue, err := strconv.Atoi(spinsValueStr)
		if err != nil {
			return nil, fmt.Errorf("ошибка преобразования spins: %w", err)
		}

		// Создаем структуру Calls и заполняем ее данными
		calls := model.QueueSpin{
			Spin:      &spinsValue,
			QueueName: &queueNameValue,
		}

		callsData = append(callsData, calls)
	}

	// Возвращаем срез структуры Calls
	return &callsData, nil
}
