package function

import (
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	createTeamsTableSQL = `CREATE TABLE IF NOT EXISTS caf.teams (
			id serial4 NOT NULL,
			"name" varchar NULL,
			webitel_queues_ids _int4 NULL,
			strategy varchar NULL,
			active bool DEFAULT true NULL,
			stop_days int4 DEFAULT 0 NULL,
			analize_attempt_count int4 DEFAULT 0 NULL,
			email varchar NULL,
			filtration bool DEFAULT false NULL,
			bad_sip_codes _int4 NULL,
			CONSTRAINT strategy_check CHECK (((strategy)::text = ANY ((ARRAY['cause'::character varying, 'unsuccessful'::character varying])::text[]))),
			CONSTRAINT teams_pk PRIMARY KEY (id)
		);`

	createNumbersTableSQL = `CREATE TABLE IF NOT EXISTS caf.numbers (
			id bigserial NOT NULL,
			"number" varchar NULL,
			client_id varchar NULL,
			queue_id int4 NULL,
			team_id int4 NULL,
			first_load_at timestamptz NULL,
			last_load_at timestamptz NULL,
			load_counter int4 NULL,
			attempts_counter int4 NULL,
			success bool DEFAULT false NULL,
			"blocked" bool DEFAULT false NULL,
			blocked_at timestamptz NULL,
			stat_waiting bool DEFAULT true NULL,
			stop_expirid timestamptz NULL,
			today_success_call bool DEFAULT false NULL,
			repeated_check bool DEFAULT false NULL,
			block_rechecked bool NULL,
			member_id varchar NULL,
			first_success_call_at timestamptz NULL,
			second_success_call_at timestamptz NULL,
			CONSTRAINT numbers_pk PRIMARY KEY (id)
		);`

	createLogsTableSQL = `CREATE TABLE IF NOT EXISTS caf.logs (
			id bigserial NOT NULL,
			created_at timestamptz NULL,
			num_id int8 NULL,
			description varchar NULL,
			"number" varchar NULL,
			team_id int4 NULL,
			filtered bool DEFAULT false NULL,
			sent bool DEFAULT false NULL,
			CONSTRAINT logs_pk PRIMARY KEY (id),
			CONSTRAINT logs_numbers_fk FOREIGN KEY (num_id) REFERENCES caf.numbers(id) ON DELETE CASCADE
		);`

	createReasonsTableSQL = `CREATE TABLE IF NOT EXISTS caf.num_reasons (
			id bigserial NOT NULL,
			num_id int8 NULL,
			count int4 NULL,
			sip_code varchar NULL,
			sip_reason varchar NULL,
			CONSTRAINT num_reasons_pk PRIMARY KEY (id),
			CONSTRAINT num_reasons_numbers_fk FOREIGN KEY (num_id) REFERENCES caf.numbers(id) ON DELETE CASCADE
		);`

	createBlackListTableSQL = `CREATE TABLE IF NOT EXISTS caf.blacklist (
			id bigserial NOT NULL,
			"number" varchar NULL,
			created_at timestamptz NULL,
			team_id int4 NULL,
			description varchar NULL,
			CONSTRAINT blacklist_pk PRIMARY KEY (id)
		);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createTeamsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createNumbersTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createLogsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createReasonsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createBlackListTableSQL)
	if err != nil {
		return err
	}

	// SQL-запросы для создания индексов
	indexSQLs := []string{
		"CREATE INDEX IF NOT EXISTS numbers_blocked_idx ON caf.numbers USING btree (blocked);",
		"CREATE INDEX IF NOT EXISTS numbers_first_load_at_idx ON caf.numbers USING btree (first_load_at);",
		"CREATE INDEX IF NOT EXISTS numbers_number_idx ON caf.numbers USING btree (number);",
		"CREATE INDEX IF NOT EXISTS numbers_client_id_idx ON caf.numbers USING btree (client_id);",
		"CREATE INDEX IF NOT EXISTS numbers_repeated_ckeck_idx ON caf.numbers USING btree (repeated_check);",
		"CREATE INDEX IF NOT EXISTS numbers_stat_waiting_idx ON caf.numbers USING btree (stat_waiting);",
		"CREATE INDEX IF NOT EXISTS numbers_success_idx ON caf.numbers USING btree (success);",
		"CREATE INDEX IF NOT EXISTS numbers_today_success_call_idx ON caf.numbers USING btree (today_success_call);",
		"CREATE INDEX IF NOT EXISTS logs_created_at_idx ON caf.logs USING btree (created_at);",
		"CREATE INDEX IF NOT EXISTS logs_num_id_idx ON caf.logs USING btree (num_id);",
		"CREATE INDEX IF NOT EXISTS logs_number_idx ON caf.logs USING btree (number);",
		"CREATE INDEX IF NOT EXISTS logs_team_id_idx ON caf.logs USING btree (team_id);",
		"CREATE INDEX IF NOT EXISTS logs_sent_idx ON caf.logs USING btree (sent);",
		"CREATE INDEX IF NOT EXISTS num_reasons_num_id_idx ON caf.num_reasons USING btree (num_id);",
		"CREATE INDEX IF NOT EXISTS blacklist_created_at_idx ON caf.blacklist USING btree (created_at);",
		"CREATE INDEX IF NOT EXISTS blacklist_number_idx ON caf.blacklist USING btree (number);",
		"CREATE INDEX IF NOT EXISTS blacklist_team_id_idx ON caf.blacklist USING btree (team_id);",
	}

	// Выполнение запросов на создание индексов
	for _, indexSQL := range indexSQLs {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}
	return err
}
