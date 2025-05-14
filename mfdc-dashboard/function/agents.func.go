package function

import (
	"dashboard/model"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

func GetAgents(db *sqlx.DB, teamID int) ([]model.Agents, error) {
	// Получаем данные о команде из БД
	teamData, err := GetTeam(db, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team data from DB for teamID %d: %v", teamID, err)
	}

	// Соединяемся с Redis
	redisClient := RedisClient()
	defer redisClient.Close() // Закрываем соединение при выходе из функции

	// Создаем мапу для хранения агентов по командам
	agentsByTeam := make(map[string][]model.AgentsData)

	// Перебираем срез TeamID для Webitel
	for _, team := range *teamData.WebitelTeamIDS {
		// Преобразуем Webitel TeamID в string
		teamIDStr := strconv.Itoa(team)
		teamIDStr = "agents_" + teamIDStr

		// Пытаемся получить список кэшированных агентов из Redis
		redisAgents, err := GetAllAgents(redisClient, teamIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to get agents from redis for teamID %s: %v", teamIDStr, err)
		}

		var teamName string // Переменная для хранения имени команды

		if len(redisAgents.Agents) > 0 {
			for _, redisAgent := range redisAgents.Agents {
				agentsByTeam[redisAgents.TeamName] = append(agentsByTeam[redisAgents.TeamName], redisAgent) // Добавляем агента из Redis
			}
			continue // Переходим к следующему TeamID
		}

		// Создаем новый запрос к API Webitel
		url := fmt.Sprintf("%s/call_center/agents?team_id=%d&page=%d&size=%d", config.API_Webitel.URL, team, 1, 1000)

		response, statusCode, err := APIFetch(config.API_Webitel.Header, config.API_Webitel.Key, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch agents from API for teamID %d: %v", teamID, err)
		}

		if statusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned non-200 status for teamID %d: %d", teamID, statusCode)
		}

		var agentsResponse model.Response
		if err := json.Unmarshal([]byte(response), &agentsResponse); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response from API for teamID %d: %v", teamID, err)
		}

		for _, agent := range agentsResponse.Items {
			if teamName == "" {
				teamName = agent.Team.Name // Устанавливаем имя команды на основе первого агента текущей Webitel TeamID
			}
			if agent.Status == "online" || agent.Status == "pause" { // Берём только агентов с online/pause статусом
				TimeInStatus, err := FormatTimeInStatus(agent.LastStatusChange)
				if err != nil {
					TimeInStatus = agent.LastStatusChange
				}

				TimeInState, err := FormatTimeInStatus(agent.Channel[0].JoinedAt)
				if err != nil {
					TimeInState = agent.Channel[0].JoinedAt
				}
				agentData := model.AgentsData{
					UserID:           agent.User.ID,
					UserName:         agent.Name,
					Status:           agent.Status,
					LastStatusChange: TimeInStatus,
					State:            agent.Channel[0].State,
					LastStateChange:  TimeInState,
					Extension:        agent.Extension,
				}
				agentsByTeam[teamName] = append(agentsByTeam[teamName], agentData) // Добавляем агента в соответствующую команду

				// Кэшируем агента в Redis
				if err := AddToRedisHSET(redisClient, agent.ID, teamIDStr, teamName, 30*time.Second,
					"user_id", agentData.UserID,
					"user_name", agentData.UserName,
					"status", agentData.Status,
					"last_status_change", agentData.LastStatusChange,
					"state", agentData.State,
					"last_state_change", agentData.LastStateChange,
					"extension", agentData.Extension,
				); err != nil {
					ErrLog.Printf("Failed to add agent to Redis for userID %s: %v", agentData.UserID, err)
				}
			} else {
				continue
			}
		}

		// Устанавливаем время жизни для коллекций с ID агентов
		SetExpireToTeamIDList(redisClient, teamIDStr, 30*time.Second)
	}

	// Создаем срез для хранения имен команд
	teamNames := make([]string, 0, len(agentsByTeam))

	// Заполняем срез именами команд
	for teamName := range agentsByTeam {
		teamNames = append(teamNames, teamName)
	}

	// Сортируем срез имен команд в алфавитном порядке
	sort.Strings(teamNames)

	// Обрабатываем команды в отсортированном порядке
	var result []model.Agents
	for _, teamName := range teamNames {
		if agents, exists := agentsByTeam[teamName]; exists {
			result = append(result, model.Agents{
				TeamName: teamName,
				Count:    len(agents),
				Agents:   agents,
			})
			// Сортируем по имени агента в алфавитном порядке
			sort.Sort(model.ByName(agents))
		}
	}

	return result, nil
}

// List agents online godoc
// @Summary      List agents online
// @Description  List agents online
// @Tags         Agents
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerStandartList
// @Router       /agents [get]
// @Security ApiKeyAuth
func DashAgents(db *sqlx.DB, c *gin.Context) {
	teamID, err := GetTeamIDFromJWT(c)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get TeamID", "error": err.Error()})
		return
	}

	agents, err := GetAgents(db, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to get agents", "error": err.Error()})
		return
	}

	// Считаем обшее количество агентов
	var total int
	for _, agent := range agents {
		total += len(agent.Agents)
	}

	result := model.AgentsResult{
		Total:  total,
		Status: "success",
		Data:   agents,
	}

	c.JSON(http.StatusOK, result)
}
