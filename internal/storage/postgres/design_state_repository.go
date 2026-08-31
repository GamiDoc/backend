package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/gamidoc/backend/internal/design"
)

type DesignStateRepository struct {
	db *DB
}

func NewDesignStateRepository(db *DB) *DesignStateRepository {
	return &DesignStateRepository{db: db}
}

func (r *DesignStateRepository) Get(ctx context.Context, id string) (design.Status, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		SELECT data
		FROM design_states
		WHERE project_id = $1
		`,
		id,
	)

	var data string
	if err := row.Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return design.NewInitialStatus(), nil
		}
		return design.Status{}, err
	}

	var status design.Status
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return design.Status{}, err
	}
	if status.Sections == nil {
		status.Sections = map[string]design.SectionState{}
	}

	return status, nil
}

func (r *DesignStateRepository) Save(ctx context.Context, id string, status design.Status) error {
	data, err := json.Marshal(status)
	if err != nil {
		return err
	}

	_, err = r.db.sql.ExecContext(
		ctx,
		`
		INSERT INTO design_states (project_id, data, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (project_id) DO UPDATE SET data = $2, updated_at = NOW()
		`,
		id,
		string(data),
	)
	return err
}
