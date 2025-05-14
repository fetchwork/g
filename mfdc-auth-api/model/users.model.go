package model

import (
	"time"

	"github.com/jackc/pgtype"
)

type User struct {
	UID             int        `json:"uid,omitempty"`               // уникальный идентификатор пользователя
	Firstname       *string    `json:"firstname,omitempty"`         // имя пользователя (может быть NULL)
	Lastname        *string    `json:"lastname,omitempty"`          // фамилия пользователя (может быть NULL)
	Email           string     `json:"email"`                       // электронная почта является логином (не может быть NULL)
	Password        string     `json:"password"`                    // пароль (не может быть NULL)
	Role            string     `json:"role"`                        // роль пользователя (не может быть NULL)
	PwdChangeAt     *time.Time `json:"pwd_change_at,omitempty"`     // дата и время последнего изменения пароля (может быть NULL)
	PwdResetExpires *time.Time `json:"pwd_reset_expires,omitempty"` // дата и время, когда срок действия сброса пароля истекает (может быть NULL)
	Enabled         *bool      `json:"enabled,omitempty"`           // статус включения пользователя (может быть NULL)
	TeamID          *int       `json:"team_id,omitempty"`
	TokenVersion    int        `json:"token_version,omitempty"`
	Sections        []string   `json:"sections"`
}

type UserInfoSwagger struct {
	email string
	role  string
}

type Reload struct {
	Reload string `json:"reload"`
}

type UserEdit struct {
	Firstname    *string    `json:"firstname,omitempty"`
	Lastname     *string    `json:"lastname,omitempty"`
	Email        *string    `json:"email,omitempty"`
	Password     *string    `json:"password,omitempty"`
	Role         *string    `json:"role,omitempty"`
	Enabled      *bool      `json:"enabled,omitempty"`
	TeamID       *int       `json:"team_id,omitempty"`
	TokenVersion *int       `json:"token_version"`
	PwdChangeAt  *time.Time `json:"pwd_change_at,omitempty"`
	Sections     *[]string  `json:"sections,omitempty"`
}

type UserEditCurrent struct {
	Firstname    *string           `db:"firstname"`
	Lastname     *string           `db:"lastname"`
	Email        *string           `db:"email"`
	Password     *string           `db:"password"`
	Role         *string           `db:"role"`
	Enabled      *bool             `db:"enabled"`
	TeamID       *int              `db:"team_id"`
	TokenVersion *int              `db:"token_version"`
	PwdChangeAt  *time.Time        `db:"pwd_change_at,omitempty"`
	Sections     *pgtype.TextArray `db:"sections"`
}

type UsersList struct {
	Uid             *int32           `db:"uid" json:"uid"`
	Firstname       *string          `db:"firstname" json:"firstname"`
	Lastname        *string          `db:"lastname" json:"lastname"`
	Email           *string          `db:"email" json:"email"`
	Role            *string          `db:"role" json:"role"`
	PwdChangeAt     *time.Time       `db:"pwd_change_at" json:"pwd_change_at"`
	PwdResetExpires *time.Time       `db:"pwd_reset_expires" json:"pwd_reset_expires"`
	Enabled         *pgtype.Bool     `db:"enabled" json:"enabled"`
	TeamID          *int             `db:"team_id" json:"team_id,omitempty"`
	Sections        pgtype.TextArray `db:"sections" json:"-"`
	SectionsList    []string         `json:"sections"` // Поле для JSON вывода
}

type UsersListSwaggerResponse struct {
	Status string `json:"status"`
	Data   struct {
		Uid             int32      `json:"uid"`
		Firstname       *string    `json:"firstname"`
		Lastname        *string    `json:"lastname"`
		Email           *string    `json:"email"`
		Password        *string    `json:"password"`
		Role            *string    `json:"role"`
		PwdChangeAt     *time.Time `json:"pwd_change_at"`
		PwdResetExpires *time.Time `json:"pwd_reset_expires"`
		Enabled         *bool      `json:"enabled,omitempty"`
		TeamID          *int       `json:"team_id,omitempty"`
		Sections        *[]string  `json:"sections"`
	} `json:"data"`
}

type UserDeleteSwagger struct {
	Status string `json:"status"`
}

type Teams struct {
	ID   *int    `db:"id" json:"id,omitempty"`
	Name *string `db:"name" json:"name,omitempty"`
}
