package function

import (
	_ "github.com/jackc/pgx/stdlib"
	"github.com/jmoiron/sqlx"
)

const (
	createExportTasksTableSQL = `CREATE TABLE IF NOT EXISTS billing.export_tasks (
			id int4 DEFAULT nextval('billing.csv_export_id_seq'::regclass) NOT NULL,
			created_at timestamptz NULL,
			"name" varchar NULL,
			args jsonb NULL,
			"new" bool DEFAULT false NULL,
			done bool DEFAULT false NULL,
			saved bool DEFAULT false NULL,
			CONSTRAINT csv_export_pk PRIMARY KEY (id)
		);
		CREATE INDEX IF NOT EXISTS csv_export_created_at_idx ON billing.export_tasks USING btree (created_at);
		CREATE INDEX IF NOT EXISTS export_tasks_done_idx ON billing.export_tasks USING btree (done);
		CREATE INDEX IF NOT EXISTS export_tasks_name_idx ON billing.export_tasks USING btree (name);`

	createExportDataTableSQL = `CREATE TABLE IF NOT EXISTS billing.export_data (
			id bigserial NOT NULL,
			task_id int4 NULL,
			data_date date NULL,
			"content" text NULL,
			CONSTRAINT export_data_pk PRIMARY KEY (id)
		);
		CREATE INDEX IF NOT EXISTS export_data_task_id_idx ON billing.export_data USING btree (task_id);`
)

func CreateTables(db *sqlx.DB) error {

	// Выполнение SQL-запросов для создания таблиц
	_, err := db.Exec(createExportTasksTableSQL)
	if err != nil {
		return err
	}

	_, err = db.Exec(createExportDataTableSQL)
	if err != nil {
		return err
	}

	return nil
}
