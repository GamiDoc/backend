package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gamidoc/backend/internal/design"
)

type DesignReportRepository struct {
	db *DB
}

func NewDesignReportRepository(db *DB) *DesignReportRepository {
	return &DesignReportRepository{db: db}
}

func (r *DesignReportRepository) Create(ctx context.Context, report design.Report) (design.Report, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		INSERT INTO design_reports (id, project_id, version, url, created_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, version, url, created_at
		`,
		report.ID,
		report.ProjectID,
		report.Version,
		report.URL,
		report.CreatedAt,
	)

	var created design.Report
	if err := row.Scan(&created.ID, &created.ProjectID, &created.Version, &created.URL, &created.CreatedAt); err != nil {
		return design.Report{}, err
	}

	return created, nil
}

func (r *DesignReportRepository) Delete(ctx context.Context, projectID string, reportID string) (design.Report, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		DELETE FROM design_reports
		WHERE id = $1 AND project_id = $2
		RETURNING id, project_id, version, url, created_at
		`,
		reportID,
		projectID,
	)

	var deleted design.Report
	if err := row.Scan(&deleted.ID, &deleted.ProjectID, &deleted.Version, &deleted.URL, &deleted.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return design.Report{}, design.ErrReportNotFound
		}
		return design.Report{}, err
	}
	return deleted, nil
}

func (r *DesignReportRepository) UpdateURL(ctx context.Context, projectID string, reportID string, url string) (design.Report, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		UPDATE design_reports SET url = $3
		WHERE id = $1 AND project_id = $2
		RETURNING id, project_id, version, url, created_at
		`,
		reportID,
		projectID,
		url,
	)

	var updated design.Report
	if err := row.Scan(&updated.ID, &updated.ProjectID, &updated.Version, &updated.URL, &updated.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return design.Report{}, design.ErrReportNotFound
		}
		return design.Report{}, err
	}
	return updated, nil
}

func (r *DesignReportRepository) ListByProjectID(ctx context.Context, projectID string) ([]design.Report, error) {
	rows, err := r.db.sql.QueryContext(
		ctx,
		`
		SELECT id, project_id, version, url, created_at
		FROM design_reports
		WHERE project_id = $1
		ORDER BY created_at DESC
		`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []design.Report
	for rows.Next() {
		var found design.Report
		if err := rows.Scan(&found.ID, &found.ProjectID, &found.Version, &found.URL, &found.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, found)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
