package model

import (
	"encoding/json"

	"github.com/jackc/pgtype"
)

type Team struct {
	ID             *int             `db:"id" json:"caf_team_id,omitempty"`
	Name           *string          `db:"name" json:"name,omitempty"`
	TeamID         *int             `db:"team_id" json:"team_id,omitempty"`
	WebitelTeamIDS *[]int           `db:"webitel_team_ids" json:"webitel_team_ids,omitempty"`
	WebitelQueues  *[]WebitelQueues `db:"webitel_queues" json:"webitel_queues,omitempty"`
}

type TeamDB struct {
	ID             *int              `db:"id"`
	Name           *string           `db:"name"`
	TeamID         *int              `db:"team_id"`
	WebitelTeamIDS *pgtype.Int4Array `db:"webitel_team_ids"`
	WebitelQueues  *json.RawMessage  `db:"webitel_queues" json:"webitel_queues,omitempty"`
}

type WebitelQueues struct {
	Name    *string `db:"name" json:"name,omitempty"`
	QueueID *int    `db:"queue_id" json:"queue_id,omitempty"`
}
