package postgres

import (
	"context"

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
