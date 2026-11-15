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
