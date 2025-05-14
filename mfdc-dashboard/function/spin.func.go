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

func GetSpins(db *sqlx.DB, dbWebitel *sqlx.DB, teamID int) ([]model.QueueSpin, error) {
	// Получаем данные о команде из БД
	teamData, err := GetTeam(db, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team data from DB for teamID %d: %w", teamID, err)
	}

	// Соединяемся с Redis
	redisClient := RedisClient()
	defer redisClient.Close() // Закрываем соединение при выходе из функции

	var calls []model.QueueSpin

	if teamData.WebitelQueues != nil {
		for _, team := range *teamData.WebitelQueues {
			// Преобразуем Webitel TeamID в string
			var queueIDStr string
			if team.QueueID != nil {
				queueIDStr = strconv.Itoa(*team.QueueID)
				queueIDStr = "spin_" + queueIDStr
			} else {
				continue // Пропускаем итерацию, если QueueID nil
			}

			// Пытаемся получить список кэшированных spin из Redis
			redisSpin, err := GetAllSpins(redisClient, queueIDStr)
			if err != nil {
				return nil, fmt.Errorf("failed to get spins from redis for queueID %s: %w", queueIDStr, err)
			}

			if len(*redisSpin) > 0 {
				calls = append(calls, *redisSpin...)
				continue
			}

			var spinCount sql.NullInt64 // Используем sql.NullInt64 для обработки NULL значений
			query := `SELECT MAX(seq) FROM call_center.cc_member_attempt_history 
        				WHERE
            				joined_at > CURRENT_DATE 
            				AND queue_id = $1`
			err = dbWebitel.Get(&spinCount, query, *team.QueueID)
			if err != nil {
				return nil, fmt.Errorf("failed to get queue ID %d: %w", *team.QueueID, err)
			}

			// Создаем структуру QueueSpin и заполняем ее данными
			callsData := model.QueueSpin{
				Spin:      new(int), // Создаем новый int для хранения spin
				QueueName: team.Name,
			}
			if spinCount.Valid {
				*callsData.Spin = int(spinCount.Int64) // Присваиваем значение только если оно не NULL
			} else {
				*callsData.Spin = 0 // Устанавливаем 0, если значение NULL
			}

			calls = append(calls, callsData)

			// Кэшируем spin в Redis
			callsStr := strconv.Itoa(*callsData.Spin)
			if err := AddToRedisHSETByQueue(redisClient, *callsData.QueueName, queueIDStr, 180*time.Second, "spin", callsStr, "queue_name", *callsData.QueueName); err != nil {
				ErrLog.Printf("Failed to add call data to Redis for queueName %s: %v", *callsData.QueueName, err)
			}

			SetExpireToTeamIDList(redisClient, queueIDStr, 180*time.Second)
		}
	}

	// Сортируем срез имен команд в алфавитном порядке
	sort.Sort(model.SpinByQueueName(calls))

	return calls, nil
}

// List spins godoc
// @Summary      List spins
// @Description  List spins
// @Tags         Spins
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartList
// @Router       /spins [get]
// @Security ApiKeyAuth
func DashSpins(db *sqlx.DB, dbWebitel *sqlx.DB, c *gin.Context) {
	teamID, err := GetTeamIDFromJWT(c)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get TeamID", "error": err.Error()})
		return
	}

	spins, err := GetSpins(db, dbWebitel, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get spins", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": spins})
}
