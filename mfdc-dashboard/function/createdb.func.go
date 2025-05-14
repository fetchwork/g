package function

import (
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	createTeamsTableSQL = `CREATE TABLE IF NOT EXISTS dashboard.teams (
				id serial4 NOT NULL,
				team_id int4 NULL,
				webitel_team_ids _int4 NULL,
				webitel_queue_ids _int4 NULL,
				"name" varchar NULL,
				CONSTRAINT teams_pk PRIMARY KEY (id)
			);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createTeamsTableSQL)
	if err != nil {
		return err
	}

	// SQL-запросы для создания индексов
	indexSQLs := []string{
		"CREATE INDEX IF NOT EXISTS teams_team_id_idx ON dashboard.teams USING btree (team_id);",
	}

	// Выполнение запросов на создание индексов
	for _, indexSQL := range indexSQLs {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}
	return err
}
