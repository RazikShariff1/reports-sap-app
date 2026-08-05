package migrations

import "gofr.dev/pkg/gofr/migration"

const createReportsTable = `
CREATE TABLE IF NOT EXISTS files (
    id             SERIAL PRIMARY KEY,
    status         VARCHAR(20) NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','processing','processed','failed')),
    params         JSONB NOT NULL,
    output_path    TEXT,
    error_message  TEXT,
    created_at     TIMESTAMP NOT NULL DEFAULT now(),
    updated_at     TIMESTAMP NOT NULL DEFAULT now(),
    completed_at   TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
`

func createReportsTableMigration() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(createReportsTable)
			return err
		},
	}
}
