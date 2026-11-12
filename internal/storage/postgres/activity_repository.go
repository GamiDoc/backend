package postgres

import (
	"context"
	"encoding/json"

	"github.com/gamidoc/backend/internal/activity"
)

type ActivityRepository struct {
	db *DB
}

func NewActivityRepository(db *DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

func (r *ActivityRepository) Save(ctx context.Context, event activity.Event) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}

	durationMS := int64(event.Duration / 1_000_000)

	_, err = r.db.sql.ExecContext(
		ctx,
		`
		INSERT INTO activity_events (
			id,
			event_type,
			user_id,
			session_id,
			project_id,
			method,
			path,
			status_code,
			duration_ms,
			metadata,
			created_at
		)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8, $9, $10, $11)
		`,
		event.ID,
		event.Type,
		event.UserID,
		event.SessionID,
		event.ProjectID,
		event.Method,
		event.Path,
		event.StatusCode,
		durationMS,
		string(metadata),
		event.CreatedAt,
	)
	return err
}
