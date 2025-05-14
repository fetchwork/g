package model

import "github.com/jackc/pgtype"

type Team struct {
	ID               *int    `db:"id" json:"caf_team_id,omitempty"`
	Name             *string `db:"name" json:"name,omitempty"`
	Active           *bool   `db:"active" json:"active,omitempty"`
	Filtration       *bool   `db:"filtration" json:"filtration,omitempty"`
	EMail            *string `db:"email" json:"email,omitempty"`
	StopDays         *int    `db:"stop_days" json:"stop_days,omitempty"`
	Strategy         *string `db:"strategy" json:"strategy,omitempty"`
	WebitelQueuesIDS *[]int  `db:"webitel_queues_ids" json:"webitel_queues_ids,omitempty"`
	BadSipCodes      *[]int  `db:"bad_sip_codes" json:"bad_sip_codes,omitempty"`
}

type TeamDB struct {
	ID               *int              `db:"id"`
	Name             *string           `db:"name"`
	Active           *bool             `db:"active"`
	Filtration       *bool             `db:"filtration" json:"filtration"`
	EMail            *string           `db:"email"`
	StopDays         *int              `db:"stop_days"`
	Strategy         *string           `db:"strategy"`
	WebitelQueuesIDS *pgtype.Int4Array `db:"webitel_queues_ids"`
	BadSipCodes      *pgtype.Int4Array `db:"bad_sip_codes"`
}

type SwaggerTeamsList struct {
	Status string `json:"status"`
	Data   []Team `json:"data"`
}
