package function

import (
	"caf/model"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgtype"
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
	var teamsDB []model.TeamDB
	err := db.Select(&teamsDB, "SELECT * FROM caf.teams ORDER By name")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get teams", "error": err.Error()})
		return
	}

	// Создаем срез команд
	teams := make([]model.Team, len(teamsDB))
	for idx, team := range teamsDB {
		teams[idx].ID = team.ID
		teams[idx].Name = team.Name
		teams[idx].Active = team.Active
		teams[idx].EMail = team.EMail
		teams[idx].StopDays = team.StopDays
		teams[idx].Strategy = team.Strategy
		teams[idx].Filtration = team.Filtration
		if team.WebitelQueuesIDS != nil {
			// Инициализируем teams[idx].WebitelQueuesIDS, если он nil
			if teams[idx].WebitelQueuesIDS == nil {
				teams[idx].WebitelQueuesIDS = new([]int)
			}
			*teams[idx].WebitelQueuesIDS = PgIntArr2IntArr(*team.WebitelQueuesIDS)
		}
		if team.BadSipCodes != nil {
			if teams[idx].BadSipCodes == nil {
				teams[idx].BadSipCodes = new([]int)
			}
			*teams[idx].BadSipCodes = PgIntArr2IntArr(*team.BadSipCodes)
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "data": teams})
}

// Team add godoc
// @Summary      Add team
// @Description  Add new team
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Param route body model.Team true "Team"
// @Router       /teams/add [post]
// @Security ApiKeyAuth
// @Description JSON object containing resource IDs
func AddTeam(db *sqlx.DB, c *gin.Context) {

	var request model.Team

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	if request.Name == nil || request.WebitelQueuesIDS == nil || request.EMail == nil || request.Strategy == nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "All columns are required"})
		return
	}

	var teamID int
	addQuery := `INSERT INTO caf.teams (name, active, filtration, email, stop_days, strategy, webitel_queues_ids, bad_sip_codes) 
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`

	var webitelQueuesIds pgtype.Int4Array
	var badSipCodes pgtype.Int4Array
	if request.WebitelQueuesIDS != nil {
		webitelQueuesIds = IntArr2PgIntArr(*request.WebitelQueuesIDS)
	} else {
		webitelQueuesIds = pgtype.Int4Array{Status: pgtype.Null}
	}
	if request.BadSipCodes != nil {
		badSipCodes = IntArr2PgIntArr(*request.BadSipCodes)
	} else {
		badSipCodes = pgtype.Int4Array{Status: pgtype.Null}
	}

	err := db.QueryRow(addQuery, request.Name, request.Active, request.Filtration, request.EMail, request.StopDays, request.Strategy, webitelQueuesIds, badSipCodes).Scan(&teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to insert new team", "error": err.Error()})
		return
	}

	request.ID = &teamID

	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully added", "data": request})
}

// Team edit godoc
// @Summary      Edt team
// @Description  Edit team params
// @Tags         Teams
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Team ID"
// @Param ip body model.Team true "Data without ID"
// @Success      200  {array}   model.SwaggerDefaultResponse
// @Router       /teams/edit/{id} [patch]
// @Security ApiKeyAuth
// @Description JSON object containing resource IDs
func TeamEdit(db *sqlx.DB, c *gin.Context) {
	// Получение ID из URL
	id := c.Param("id")
	CheckIDAsInt(id, c)

	var request model.Team

	// Чтение данных из тела запроса
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Invalid request data", "error": err.Error()})
		return
	}

	// Чтобы методом PATCH иметь возможно частично менять значения, прочитаем текущие
	var teamDB model.TeamDB
	err := db.Get(&teamDB, "SELECT * FROM caf.teams WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failed", "message": "Failed to get team", "error": err.Error()})
	}

	// Проверяем и устанавливаем значения
	if request.Name == nil {
		request.Name = teamDB.Name
	}
	if request.Active == nil {
		request.Active = teamDB.Active
	}
	if request.EMail == nil {
		request.EMail = teamDB.EMail
	}
	if request.StopDays == nil {
		request.StopDays = teamDB.StopDays
	}
	if request.Strategy == nil {
		request.Strategy = teamDB.Strategy
	}
	if request.Filtration == nil {
		request.Filtration = teamDB.Filtration
	}

	var WebitelQueuesIDS pgtype.Int4Array
	if request.WebitelQueuesIDS == nil {
		if teamDB.WebitelQueuesIDS != nil {
			WebitelQueuesIDS = *teamDB.WebitelQueuesIDS
		} else {
			WebitelQueuesIDS = pgtype.Int4Array{Status: pgtype.Null}
		}
	} else {
		WebitelQueuesIDS = IntArr2PgIntArr(*request.WebitelQueuesIDS)
	}

	var BadSipCodes pgtype.Int4Array
	if request.BadSipCodes == nil {
		if teamDB.BadSipCodes != nil {
			BadSipCodes = *teamDB.BadSipCodes
		} else {
			WebitelQueuesIDS = pgtype.Int4Array{Status: pgtype.Null}
		}
	} else {
		BadSipCodes = IntArr2PgIntArr(*request.BadSipCodes)
	}

	updateQuery := `UPDATE caf.teams SET
				name = $1, 
				active = $2,
				email = $3,
				stop_days = $4,
				strategy = $5,
				filtration = $6,
				webitel_queues_ids = $7,
				bad_sip_codes = $8
				WHERE id = $9`

	_, err = db.Exec(updateQuery, request.Name, request.Active, request.EMail, request.StopDays, request.Strategy, request.Filtration, WebitelQueuesIDS, BadSipCodes, id)
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
	err := db.Get(&teamExists, "SELECT EXISTS (SELECT 1 FROM caf.teams WHERE id = $1)", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to check team on exists", "error": err.Error()})
		return
	}

	if !teamExists {
		c.JSON(http.StatusNotFound, gin.H{"status": "failed", "message": "Team not found for deletion"})
		return
	}

	_, err = db.Exec("DELETE FROM caf.teams WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "failed", "message": "Failed to delete team", "error": err.Error()})
		return
	}

	// Отправляем JSON-ответ
	c.IndentedJSON(http.StatusOK, gin.H{"status": "success", "message": "Team successfully deleted"})
}
