package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/gamidoc/backend/internal/user"
)

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, input user.User) (user.User, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		INSERT INTO users (id, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, created_at, email_verified_at
		`,
		input.ID,
		input.Email,
		input.PasswordHash,
	)

	var created user.User
	err := row.Scan(&created.ID, &created.Email, &created.PasswordHash, &created.CreatedAt, &created.EmailVerifiedAt)
	if err != nil {
		return user.User{}, err
	}

	return created, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		SELECT id, email, password_hash, created_at, email_verified_at
		FROM users
		WHERE email = $1
		`,
		email,
	)

	var found user.User
	err := row.Scan(&found.ID, &found.Email, &found.PasswordHash, &found.CreatedAt, &found.EmailVerifiedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}

	return found, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (user.User, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		SELECT id, email, password_hash, created_at, email_verified_at
		FROM users
		WHERE id = $1
		`,
		id,
	)

	var found user.User
	err := row.Scan(&found.ID, &found.Email, &found.PasswordHash, &found.CreatedAt, &found.EmailVerifiedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}

	return found, nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id string) error {
	result, err := r.db.sql.ExecContext(ctx,
		`UPDATE users SET email_verified_at = COALESCE(email_verified_at, NOW()) WHERE id = $1`, id)
	if err != nil {
		return err
	}
	return requireRow(result)
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	result, err := r.db.sql.ExecContext(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, passwordHash)
	if err != nil {
		return err
	}
	return requireRow(result)
}

func requireRow(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return user.ErrUserNotFound
	}
	return nil
}

// Delete removes the user; projects, design states and reports go with them (ON DELETE
// CASCADE) and the activity log keeps the events without the user id.
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	tx, err := r.db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE activity_events SET user_id = NULL WHERE user_id = $1`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if err := requireRow(result); err != nil {
		return err
	}
	return tx.Commit()
}

// StoredFiles returns what the user's projects reference in the object storage: the
// URLs the server saved (evaluation PDFs, design documents) and, per project, the design
// data (JSON written by the user, with the URLs of the uploaded images).
func (r *UserRepository) StoredFiles(ctx context.Context, id string) ([]string, map[string]string, error) {
	rows, err := r.db.sql.QueryContext(ctx, `
		SELECT p.id::text, 'file', p.pdf_url FROM projects p WHERE p.user_id = $1 AND p.pdf_url IS NOT NULL
		UNION ALL
		SELECT p.id::text, 'file', dr.url FROM design_reports dr JOIN projects p ON p.id = dr.project_id WHERE p.user_id = $1
		UNION ALL
		SELECT p.id::text, 'design', ds.data FROM design_states ds JOIN projects p ON p.id = ds.project_id WHERE p.user_id = $1
	`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var files []string
	designs := map[string]string{}
	for rows.Next() {
		var projectID, kind, value string
		if err := rows.Scan(&projectID, &kind, &value); err != nil {
			return nil, nil, err
		}
		if kind == "design" {
			designs[projectID] = value
		} else {
			files = append(files, value)
		}
	}
	return files, designs, rows.Err()
}
