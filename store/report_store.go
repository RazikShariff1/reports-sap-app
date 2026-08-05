package store

import (
	"context"
	"database/sql"
	"encoding/json"

	"gofr.dev/pkg/gofr/container"

	"reports-app/entity"
)

type ReportStore struct{}

func New() *ReportStore {
	return &ReportStore{}
}

func (*ReportStore) Create(ctx context.Context, c *container.Container, params entity.Params) (int64, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return 0, err
	}

	var id int64

	row := c.SQL.QueryRowContext(ctx,
		`INSERT INTO files (status, params) VALUES ('pending', $1::jsonb) RETURNING id`, string(b))
	if err := row.Scan(&id); err != nil {
		return 0, err
	}

	return id, nil
}

const reportColumns = `id, status, params, output_path, error_message, created_at, updated_at, completed_at`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanReport(s rowScanner) (*entity.Report, error) {
	var (
		r          entity.Report
		paramsRaw  string
		outputPath sql.NullString
		errMsg     sql.NullString
		completed  sql.NullTime
	)

	err := s.Scan(&r.ID, &r.Status, &paramsRaw, &outputPath, &errMsg,
		&r.CreatedAt, &r.UpdatedAt, &completed)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(paramsRaw), &r.Params); err != nil {
		return nil, err
	}

	if outputPath.Valid {
		r.OutputPath = &outputPath.String
	}

	if errMsg.Valid {
		r.ErrorMessage = &errMsg.String
	}

	if completed.Valid {
		r.CompletedAt = &completed.Time
	}

	return &r, nil
}

func (*ReportStore) GetByID(ctx context.Context, c *container.Container, id int64) (*entity.Report, error) {
	row := c.SQL.QueryRowContext(ctx, `SELECT `+reportColumns+` FROM files WHERE id = $1`, id)
	return scanReport(row)
}

// ListByAccountID returns every report requested by accountID, most recent first.
func (*ReportStore) ListByAccountID(ctx context.Context, c *container.Container, accountID int) ([]*entity.Report, error) {
	rows, err := c.SQL.QueryContext(ctx,
		`SELECT `+reportColumns+`
		 FROM files WHERE (params->>'account_id')::int = $1
		 ORDER BY created_at DESC`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]*entity.Report, 0)

	for rows.Next() {
		r, err := scanReport(rows)
		if err != nil {
			return nil, err
		}

		reports = append(reports, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return reports, nil
}

func (*ReportStore) MarkProcessing(ctx context.Context, c *container.Container, id int64) error {
	_, err := c.SQL.ExecContext(ctx,
		`UPDATE files SET status = 'processing', updated_at = now() WHERE id = $1`, id)
	return err
}

func (*ReportStore) MarkProcessed(ctx context.Context, c *container.Container, id int64, outputPath string) error {
	_, err := c.SQL.ExecContext(ctx,
		`UPDATE files
		 SET status = 'processed', output_path = $1, completed_at = now(), updated_at = now()
		 WHERE id = $2`, outputPath, id)
	return err
}

func (*ReportStore) MarkFailed(ctx context.Context, c *container.Container, id int64, errMsg string) error {
	_, err := c.SQL.ExecContext(ctx,
		`UPDATE files SET status = 'failed', error_message = $1, completed_at = now(), updated_at = now() WHERE id = $2`,
		errMsg, id)
	return err
}
