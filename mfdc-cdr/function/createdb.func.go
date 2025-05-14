package function

import (
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	createCallsTableSQL = `CREATE TABLE IF NOT EXISTS cdr.calls (
			id bigserial NOT NULL,
			created_at timestamptz NOT NULL,
			from_type varchar NULL,
			from_number varchar NULL,
			to_type varchar NULL,
			to_number varchar NULL,
			duration int4 NULL,
			bill_sec int4 NULL,
			talk_sec int4 NULL,
			hold_sec int4 NULL,
			wait_sec int4 NULL,
			answered_at timestamptz NULL,
			bridged_at timestamptz NULL,
			destination varchar NULL,
			direction varchar NULL,
			call_id varchar NULL,
			has_children bool NULL,
			parent_id varchar NULL,
			user_name varchar NULL,
			agent varchar NULL,
			queue varchar NULL,
			team varchar NULL,
			hangup_at timestamptz NULL,
			hangup_by varchar NULL,
			sip_code int4 NULL,
			cause varchar NULL,
			transfer_from varchar NULL,
			transfer_to varchar NULL,
			record_file varchar NULL,
			tag_id int4 NULL,
			played varchar NULL,
			CONSTRAINT calls_call_id_unique UNIQUE (created_at, call_id),
			CONSTRAINT calls_pk PRIMARY KEY (id, created_at),
			CONSTRAINT calls_tags_fk FOREIGN KEY (tag_id) REFERENCES cdr.tags(id) ON DELETE SET NULL ON UPDATE CASCADE
		)
		PARTITION BY RANGE (created_at);`

	createTagsTableSQL = `CREATE TABLE IF NOT EXISTS cdr.tags (
			id bigserial NOT NULL,
			"name" varchar NULL,
			CONSTRAINT tags_pk PRIMARY KEY (id)
		);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createCallsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createTagsTableSQL)
	if err != nil {
		return err
	}

	return err
}
