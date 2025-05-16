package function

import (
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	createTeamsTableSQL = `CREATE TABLE IF NOT EXISTS nc.teams (
		id serial4 NOT NULL,
		"name" varchar NULL,
		webitel_res_ids jsonb NULL,
		actual_vendor_id int4 NULL,
		CONSTRAINT teams_pk PRIMARY KEY (id)
		);`

	createVendorsTableSQL = `CREATE TABLE IF NOT EXISTS nc.vendors (
		id serial4 NOT NULL,
		"name" varchar NULL,
		CONSTRAINT vendors_pk PRIMARY KEY (id)
		);`

	createPoolsTableSQL = `CREATE TABLE IF NOT EXISTS nc.pools (
		id serial4 NOT NULL,
		"name" varchar(255) NOT NULL,
		created_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
		subpool_block int4 NULL,
		vendor_id int4 NULL,
		num_count int4 NULL,
		subpool_count int4 NULL,
		active bool DEFAULT false NULL,
		rotation bool DEFAULT false NULL,
		team_id int4 NULL,
		finish bool DEFAULT false NULL,
		finish_at timestamptz NULL,
		sub_activate bool DEFAULT false NULL,
		CONSTRAINT pools_pkey PRIMARY KEY (id),
		CONSTRAINT pools_teams_fk FOREIGN KEY (team_id) REFERENCES nc.teams(id),
		CONSTRAINT pools_vendors_fk FOREIGN KEY (vendor_id) REFERENCES nc.vendors(id)
		);`

	createSubpoolsTableSQL = `CREATE TABLE IF NOT EXISTS nc.subpools (
		"index" int4 DEFAULT 0 NOT NULL,
		pool_id int4 NULL,
		id bigserial NOT NULL,
		status varchar NULL,
		activated_at timestamptz NULL,
		last_nid int8 DEFAULT 0 NULL,
		last_changed bool DEFAULT false NULL,
		CONSTRAINT subpools_pk PRIMARY KEY (id),
		CONSTRAINT subpools_pools_fk FOREIGN KEY (pool_id) REFERENCES nc.pools(id) ON DELETE CASCADE ON UPDATE CASCADE
		);`

	createNumbersTableSQL = `CREATE TABLE IF NOT EXISTS nc.numbers (
		id bigserial NOT NULL,
		value varchar(50) NOT NULL,
		spin int4 DEFAULT 0 NULL,
		pool_id int4 NULL,
		subpool_id int8 NULL,
		team_id int4 NULL,
		vendor_id int4 NULL,
		activated_at timestamptz NULL,
		used bool DEFAULT false NULL,
		active bool DEFAULT false NULL,
		"label" bool DEFAULT false NULL,
		CONSTRAINT numbers_pkey PRIMARY KEY (id),
		CONSTRAINT numbers_pools_fk FOREIGN KEY (pool_id) REFERENCES nc.pools(id) ON DELETE CASCADE ON UPDATE CASCADE,
		CONSTRAINT numbers_subpools_fk FOREIGN KEY (subpool_id) REFERENCES nc.subpools(id),
		CONSTRAINT numbers_teams_fk FOREIGN KEY (team_id) REFERENCES nc.teams(id),
		CONSTRAINT numbers_vendors_fk FOREIGN KEY (vendor_id) REFERENCES nc.vendors(id) ON DELETE CASCADE ON UPDATE CASCADE DEFERRABLE
		);`

	createLogsTableSQL = `CREATE TABLE IF NOT EXISTS nc.logs (
		id bigserial NOT NULL,
		number_id int8 NOT NULL,
		start_at timestamptz NULL,
		end_at timestamptz NULL,
		subpool_id int4 NULL,
		pool_id int4 NULL,
		team_id int4 NULL,
		vendor_id int4 NULL,
		"comment" varchar NULL,
		CONSTRAINT logs_pkey PRIMARY KEY (id),
		CONSTRAINT logs_numbers_fk FOREIGN KEY (number_id) REFERENCES nc.numbers(id),
		CONSTRAINT logs_pools_fk FOREIGN KEY (pool_id) REFERENCES nc.pools(id) ON DELETE CASCADE,
		CONSTRAINT logs_teams_fk FOREIGN KEY (team_id) REFERENCES nc.teams(id),
		CONSTRAINT logs_vendors_fk FOREIGN KEY (vendor_id) REFERENCES nc.vendors(id)
		);`

	createSchedulerTableSQL = `CREATE TABLE IF NOT EXISTS nc.scheduler (
		id int4 DEFAULT nextval('nc.sets_id_seq'::regclass) NOT NULL,
		start_time timetz NULL,
		stop_time timetz NULL,
		periodic_sec int4 NULL,
		team_id int4 NULL,
		"name" varchar NULL,
		running bool DEFAULT false NULL,
		CONSTRAINT scheduler_pk PRIMARY KEY (id),
		CONSTRAINT scheduler_teams_fk FOREIGN KEY (team_id) REFERENCES nc.teams(id) DEFERRABLE
		);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createTeamsTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createVendorsTableSQL)
	if err != nil {
		return err
	}
	_, err = db.Exec(createPoolsTableSQL)
	if err != nil {
		return err
	}
	_, err = db.Exec(createSubpoolsTableSQL)
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

	_, err = db.Exec(createSchedulerTableSQL)
	if err != nil {
		return err
	}

	// SQL-запросы для создания индексов
	indexSQLs := []string{
		"CREATE INDEX IF NOT EXISTS numbers_pool_id_idx ON nc.numbers USING btree (pool_id);",
		"CREATE INDEX IF NOT EXISTS numbers_value_idx ON nc.numbers USING btree (value);",
		"CREATE INDEX IF NOT EXISTS logs_number_id_idx ON nc.logs USING btree (number_id);",
		"CREATE INDEX IF NOT EXISTS logs_start_at_idx ON nc.logs USING btree (start_at);",
		"CREATE INDEX IF NOT EXISTS subpools_index_idx ON nc.subpools USING btree (index);",
		"CREATE INDEX IF NOT EXISTS subpools_last_nid_idx ON nc.subpools USING btree (last_nid);",
		"CREATE INDEX IF NOT EXISTS subpools_pool_id_idx ON nc.subpools USING btree (pool_id);",
		"CREATE INDEX IF NOT EXISTS subpools_status_idx ON nc.subpools USING btree (status);",
	}

	// Выполнение запросов на создание индексов
	for _, indexSQL := range indexSQLs {
		if _, err := db.Exec(indexSQL); err != nil {
			return err
		}
	}
	return err
}
