package function

import (
	"dashboard/model"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func GetCalls(db *sqlx.DB, dbWebitel *sqlx.DB, teamID int) ([]model.Calls, error) {
	// Получаем данные о команде из БД
	teamData, err := GetTeam(db, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team data from DB for teamID %d: %w", teamID, err)
	}

	// Соединяемся с Redis
	redisClient := RedisClient()
	defer redisClient.Close() // Закрываем соединение при выходе из функции

	var calls []model.Calls

	if teamData.WebitelQueues != nil {
		for _, team := range *teamData.WebitelQueues {
			// Преобразуем Webitel TeamID в string
			var queueIDStr string
			if team.QueueID != nil {
				queueIDStr = strconv.Itoa(*team.QueueID)
				queueIDStr = "calls_" + queueIDStr
			} else {
				continue // Пропускаем итерацию, если QueueID nil
			}

			// Пытаемся получить список кэшированных наборов из Redis
			redisCalls, err := GetAllCalls(redisClient, queueIDStr)
			if err != nil {
				return nil, fmt.Errorf("failed to get calls from redis for queueID %s: %w", queueIDStr, err)
			}

			if len(*redisCalls) > 0 {
				calls = append(calls, *redisCalls...)
				continue
			}

			var callsCount sql.NullInt64 // Используем sql.NullInt64 для обработки NULL значений
			query := `SELECT COUNT(*) AS calls FROM call_center.cc_member_attempt_history a
                      LEFT JOIN call_center.cc_queue q ON q.id = a.queue_id
                      WHERE a.domain_id = 1
                      AND a.joined_at BETWEEN (NOW() - interval '1 minute')::timestamptz AND NOW()::timestamptz 
                      AND a.queue_id = $1`
			err = dbWebitel.Get(&callsCount, query, *team.QueueID)
			if err != nil {
				return nil, fmt.Errorf("failed to get queue ID %d: %w", *team.QueueID, err)
			}

			// Создаем структуру Calls и заполняем ее данными
			callsData := model.Calls{
				Calls:     new(int), // Создаем новый int для хранения количества вызовов
				QueueName: team.Name,
			}
			if callsCount.Valid {
				*callsData.Calls = int(callsCount.Int64) // Присваиваем значение только если оно не NULL
			} else {
				*callsData.Calls = 0 // Устанавливаем 0, если значение NULL
			}

			calls = append(calls, callsData)

			// Кэшируем calls в Redis
			callsStr := strconv.Itoa(*callsData.Calls)
			if err := AddToRedisHSETByQueue(redisClient, *callsData.QueueName, queueIDStr, 30*time.Second, "calls", callsStr, "queue_name", *callsData.QueueName); err != nil {
				ErrLog.Printf("Failed to add call data to Redis for queueName %s: %v", *callsData.QueueName, err)
			}

			SetExpireToTeamIDList(redisClient, queueIDStr, 30*time.Second)
		}
	}

	// Сортируем срез имен команд в алфавитном порядке
	sort.Sort(model.ByQueueName(calls))

	return calls, nil
}

// List calls godoc
// @Summary      List calls
// @Description  List calls
// @Tags         Calls
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartList
// @Router       /calls [get]
// @Security ApiKeyAuth
func DashCalls(db *sqlx.DB, dbWebitel *sqlx.DB, c *gin.Context) {
	teamID, err := GetTeamIDFromJWT(c)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get TeamID", "error": err.Error()})
		return
	}

	calls, err := GetCalls(db, dbWebitel, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get calls", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": calls})
}
