package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/wizard"
)

type ProjectRepository struct {
	db *DB
}

func NewProjectRepository(db *DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, input project.Project) (project.Project, error) {
	wizardData, err := json.Marshal(input.Wizard)
	if err != nil {
		return project.Project{}, err
	}

	row := r.db.sql.QueryRowContext(
		ctx,
		`
		INSERT INTO projects (id, user_id, name, description, wizard_data)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		`,
		input.ID,
		input.UserID,
		input.Name,
		nullableString(input.Description),
		wizardData,
	)

	return scanProject(row)
}

func (r *ProjectRepository) ListByUserID(ctx context.Context, userID string) ([]project.Project, error) {
	rows, err := r.db.sql.QueryContext(
		ctx,
		`
		SELECT id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		FROM projects
		WHERE user_id = $1
		ORDER BY updated_at DESC
		`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []project.Project
	for rows.Next() {
		found, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, found)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ProjectRepository) FindByID(ctx context.Context, id string) (project.Project, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		SELECT id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		FROM projects
		WHERE id = $1
		`,
		id,
	)

	found, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project.Project{}, project.ErrProjectNotFound
		}
		return project.Project{}, err
	}

	return found, nil
}

func (r *ProjectRepository) UpdateWizard(ctx context.Context, projectID string, status wizard.Status) (project.Project, error) {
	wizardData, err := json.Marshal(status)
	if err != nil {
		return project.Project{}, err
	}

	row := r.db.sql.QueryRowContext(
		ctx,
		`
		UPDATE projects
		SET wizard_data = $2, pdf_url = NULL, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		`,
		projectID,
		wizardData,
	)

	found, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project.Project{}, project.ErrProjectNotFound
		}
		return project.Project{}, err
	}

	return found, nil
}

func (r *ProjectRepository) UpdateInfo(ctx context.Context, projectID string, name string, description string) (project.Project, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		UPDATE projects
		SET name = $2, description = $3, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		`,
		projectID,
		name,
		nullableString(description),
	)

	found, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project.Project{}, project.ErrProjectNotFound
		}
		return project.Project{}, err
	}

	return found, nil
}

func (r *ProjectRepository) Delete(ctx context.Context, projectID string) error {
	result, err := r.db.sql.ExecContext(
		ctx,
		`
		DELETE FROM projects
		WHERE id = $1
		`,
		projectID,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return project.ErrProjectNotFound
	}

	return nil
}

func (r *ProjectRepository) UpdatePDFURL(ctx context.Context, projectID string, pdfURL string) (project.Project, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		UPDATE projects
		SET pdf_url = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, user_id, name, description, wizard_data, pdf_url, created_at, updated_at
		`,
		projectID,
		pdfURL,
	)

	found, err := scanProject(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return project.Project{}, project.ErrProjectNotFound
		}
		return project.Project{}, err
	}

	return found, nil
}

type projectScanner interface {
	Scan(dest ...any) error
}

func scanProject(scanner projectScanner) (project.Project, error) {
	var found project.Project
	var description sql.NullString
	var pdfURL sql.NullString
	var wizardData []byte

	err := scanner.Scan(
		&found.ID,
		&found.UserID,
		&found.Name,
		&description,
		&wizardData,
		&pdfURL,
		&found.CreatedAt,
		&found.UpdatedAt,
	)
	if err != nil {
		return project.Project{}, err
	}

	if description.Valid {
		found.Description = description.String
	}

	if pdfURL.Valid {
		value := pdfURL.String
		found.PDFURL = &value
	} else {
		found.PDFURL = nil
	}

	if len(wizardData) == 0 {
		found.Wizard = project.NewInitialWizardStatus()
		return found, nil
	}

	if err := json.Unmarshal(wizardData, &found.Wizard); err != nil {
		return project.Project{}, err
	}

	if found.Wizard.Steps == nil {
		found.Wizard.Steps = map[string]json.RawMessage{}
	}

	return found, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
