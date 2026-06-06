package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gthbn/pastebin/internal/report"
)

// ReportRepo implements report.Repository using pgxpool.
type ReportRepo struct {
	pool *pgxpool.Pool
}

// NewReportRepo creates a new ReportRepo.
func NewReportRepo(pool *pgxpool.Pool) *ReportRepo {
	return &ReportRepo{pool: pool}
}

// Insert persists a new report.
func (r *ReportRepo) Insert(ctx context.Context, rep *report.Report) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reports (id, resource_type, slug, reason, details, reporter_ip, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, rep.ID, rep.ResourceType, rep.Slug, rep.Reason, rep.Details, rep.ReporterIP, rep.Status, rep.CreatedAt)
	return err
}

// List returns reports, optionally filtered by status (empty = all), most
// recent first.
func (r *ReportRepo) List(ctx context.Context, status string, limit, offset int) ([]*report.Report, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if status == "" {
		rows, err = r.pool.Query(ctx, `
			SELECT id, resource_type, slug, reason, details, reporter_ip, status, created_at, reviewed_at
			FROM reports
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, resource_type, slug, reason, details, reporter_ip, status, created_at, reviewed_at
			FROM reports
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, status, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*report.Report
	for rows.Next() {
		item := &report.Report{}
		if err := rows.Scan(
			&item.ID, &item.ResourceType, &item.Slug, &item.Reason, &item.Details,
			&item.ReporterIP, &item.Status, &item.CreatedAt, &item.ReviewedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateStatus sets a report's status and reviewed_at timestamp. Returns true if
// a row was updated.
func (r *ReportRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status report.Status, reviewedAt *time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE reports SET status = $2, reviewed_at = $3 WHERE id = $1
	`, id, status, reviewedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// DeleteByID removes a report by id. Returns true if a row was deleted.
func (r *ReportRepo) DeleteByID(ctx context.Context, id uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM reports WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// CountByStatus returns the number of reports with the given status (empty =
// all).
func (r *ReportRepo) CountByStatus(ctx context.Context, status string) (int, error) {
	var count int
	var err error
	if status == "" {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports`).Scan(&count)
	} else {
		err = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports WHERE status = $1`, status).Scan(&count)
	}
	return count, err
}
