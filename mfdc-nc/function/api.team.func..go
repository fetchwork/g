package function

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"nc/model"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// List teams godoc
// @Summary      List teams
// @Description  List teams
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerTeamsList
// @Router       /teams/list [get]
// @Security ApiKeyAuth
func TeamsList(db *sqlx.DB, c *gin.Context) {
	var teams []model.Teams
	err := db.Select(&teams, "SELECT * FROM nc.teams ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get teams", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": teams})
}

// Rotate team number godoc
// @Summary      Rotate team number
// @Description  Rotate team number
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Team ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /teams/{id}/rotate [get]
// @Security ApiKeyAuth
func TeamNumberRotate(db *sqlx.DB, c *gin.Context) {
	// Получение ID Team из URL
	team_id := c.Param("id")
	id, err := strconv.Atoi(team_id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Team ID must be a number", "error": err.Error()})
		return
	}

	// Вызываем функцию ротации
	err = DailyRotation(db, id, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to manual rotate", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "New number has been manually rotated for team ID " + team_id})
}

// List active teams numbers godoc
// @Summary      Active teams numbers
// @Description  Active teams numbers
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerActiveTeamNumber
// @Router       /teams/activenums [get]
// @Security ApiKeyAuth
func ActiveTeamsNumbers(db *sqlx.DB, c *gin.Context) {
	var teams []model.ActiveTeamNumber
	query := `SELECT t.id, t.name, v.name AS vendor_name, n.value, n.activated_at, n.spin, sch.periodic_sec FROM nc.teams AS t 
			LEFT JOIN nc.numbers AS n ON t.id=n.team_id
			LEFT JOIN nc.vendors AS v ON n.vendor_id=v.id 
			LEFT JOIN nc.scheduler AS sch ON t.id=sch.team_id 
			WHERE n.active=$1
			ORDER By t.name`
	err := db.Select(&teams, query, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get teams", "error": err.Error()})
		return
	}

	// Перебираем срез с командами и добавляем дату следующей ротации
	for i := range teams {
		team := &teams[i] // Получаем указатель на текущую команду

		/*
			if team.Name != nil && team.VendorName != nil {
				*team.Name = *team.Name + "." + *team.VendorName
			} else if team.Name == nil && team.VendorName != nil {
				team.Name = new(string)       // Создаем новое значение типа string
				*team.Name = *team.VendorName // Присваиваем значение VendorName
			}
		*/

		if team.PeriodicSec != nil && team.ActivatedAt != nil {
			periodic := *team.PeriodicSec + 10

			// Создаем новое значение для ExpiredAt
			newExpirationTime := team.ActivatedAt.Add(time.Duration(periodic) * time.Second)
			team.ExpiredAt = &newExpirationTime

			// Устанавливаем значение для ExpiredAtUnixTime
			unixTime := team.ExpiredAt.Unix()
			team.ExpiredAtUnixTime = &unixTime
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": teams})
}

// List teams numbers per day godoc
// @Summary      List teams numbers per day
// @Description  List teams numbers per day
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerTeamDayNumbers
// @Param data body model.TeamsDayRequest true "Data"
// @Router       /teams/daynums [post]
// @Security ApiKeyAuth
func TeamsDayNumbers(db *sqlx.DB, c *gin.Context) {
	var teamRequest model.TeamsDayRequest
	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&teamRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data"})
		return
	}

	var teams []model.Teams
	err := db.Select(&teams, "SELECT * FROM nc.teams ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get teams", "error": err.Error()})
		return
	}

	teamDayNumbers := make([]model.TeamDayNumbers, len(teams))

	for tidx, team := range teams {
		teamDayNumbers[tidx].ID = team.ID
		teamDayNumbers[tidx].Name = team.Name

		var pools []model.DayPools
		err := db.Select(&pools, "SELECT id, name, created_at, subpool_block, num_count, active, vendor_id, rotation, finish, finish_at FROM nc.pools WHERE team_id=$1 ORDER By name", team.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get pools", "error": err.Error()})
			return
		}

		for pidx, pool := range pools {
			// Начинаем строить запрос
			subpoolsQuery := "SELECT id, activated_at, spin FROM nc.subpools WHERE 1=1"
			var args []interface{}
			paramIndex := 1 // Индекс для параметров

			if teamRequest.From_date != nil && teamRequest.To_date != nil {
				subpoolsQuery += fmt.Sprintf(" AND activated_at BETWEEN $%d AND $%d", paramIndex, paramIndex+1)
				args = append(args, *teamRequest.From_date, *teamRequest.To_date)
				paramIndex += 2
			} else {
				subpoolsQuery += fmt.Sprintf(" AND status=$%d", paramIndex)
				args = append(args, "active")
				paramIndex += 1
			}

			subpoolsQuery += fmt.Sprintf(" AND pool_id = $%d", paramIndex)
			args = append(args, pool.ID)
			paramIndex += 1

			// Сортируем по крайнему активированному
			subpoolsQuery += " ORDER By activated_at DESC"

			var subpool model.DaySubPool
			err := db.Get(&subpool, subpoolsQuery, args...)
			if err != nil {
				if err == sql.ErrNoRows {
					//c.JSON(http.StatusOK, gin.H{"status": "failed", "message": "Subpool not found at requested date"})
					//return
					continue
				}
				c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get active subpool", "error": err.Error()})
				return
			}
			if subpool.ID != nil {
				pools[pidx].SubPool = &subpool
			}

			var numbers []model.DayNumbers
			err = db.Select(&numbers, "SELECT id, value, activated_at, active, used, enabled, spin FROM nc.numbers WHERE subpool_id = $1 ORDER By id", subpool.ID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get numbers for active subpool", "error": err.Error()})
				return
			}

			// Ищем Spin активного номера в сабпуле
			var activeNumberSpin int // Переменная для хранения Spin активного номера
			foundActive := false     // Флаг для проверки, найден ли активный номер

			// Ищем Spin активного номера
			for _, number := range numbers {
				if number.Active {
					activeNumberSpin = number.Spin // Сохраняем Spin активного номера
					foundActive = true             // Устанавливаем флаг
					break                          // Прерываем цикл, если нашли активный номер
				}
			}

			var logs []model.DayLogs
			for nidx, number := range numbers {
				if foundActive && number.Spin >= activeNumberSpin {
					numbers[nidx].Marked = true
				}

				logs = nil // Очистка массива перед использованием
				err = db.Select(&logs, "SELECT start_at, end_at FROM nc.logs WHERE number_id = $1 ORDER BY start_at", number.ID)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get logs for number", "error": err.Error()})
					return
				}
				numbers[nidx].Logs = logs
			}

			pools[pidx].SubPool.Numbers = numbers
		}

		teamDayNumbers[tidx].Pools = pools
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": teamDayNumbers})
}

// Team add godoc
// @Summary      Add team
// @Description  Add new team
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param route body model.SwaggerTeams true "Team"
// @Router       /teams/add [post]
// @Security ApiKeyAuth
// @Description JSON object containing resource IDs
func AddTeam(db *sqlx.DB, c *gin.Context) {

	var request model.Teams

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Name == nil || request.WebitelResourceIDS == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All columns are required"})
		return
	}

	var teamID int
	err := db.QueryRow("INSERT INTO nc.teams (name, webitel_res_ids) VALUES ($1, $2) RETURNING id", *request.Name, *request.WebitelResourceIDS).Scan(&teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new team", "error": err.Error()})
		return
	}

	request.ID = &teamID

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully added", "data": request})
}

// Team schedule godoc
// @Summary      Edt team
// @Description  Edit team params
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Team ID"
// @Param ip body model.SwaggerTeams true "Data without ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /teams/edit/{id} [patch]
// @Security ApiKeyAuth
// @Description JSON object containing resource IDs
func TeamEdit(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var request model.Teams

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	// Чтобы методом PATCH иметь возможно частично менять значения, прочитаем текущие
	var data model.Teams
	err := db.Get(&data, "SELECT * FROM nc.teams WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get team", "error": err.Error()})
	}

	// Проверяем и устанавливаем значения
	if request.Name == nil {
		request.Name = data.Name
	}

	var WebitelResourceIDS json.RawMessage
	if request.WebitelResourceIDS == nil {
		WebitelResourceIDS = *data.WebitelResourceIDS
	} else {
		WebitelResourceIDS = *request.WebitelResourceIDS
	}

	_, err = db.Exec("UPDATE nc.teams SET name = $1, webitel_res_ids = $2 WHERE id = $3", request.Name, WebitelResourceIDS, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to update team", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully edited"})
}

// Delete team godoc
// @Summary      Delete team
// @Description  Drop team
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Team ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /teams/delete/{id} [delete]
// @Security ApiKeyAuth
func TeamDelete(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var teamExists bool
	err := db.Get(&teamExists, "SELECT EXISTS (SELECT 1 FROM nc.teams WHERE id = $1)", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check team on exists", "error": err.Error()})
		return
	}

	if !teamExists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Team not found for deletion"})
		return
	}

	_, err = db.Exec("DELETE FROM nc.teams WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete team", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully deleted"})
}
