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
		RETURNING id, email, password_hash, created_at
		`,
		input.ID,
		input.Email,
		input.PasswordHash,
	)

	var created user.User
	err := row.Scan(&created.ID, &created.Email, &created.PasswordHash, &created.CreatedAt)
	if err != nil {
		return user.User{}, err
	}

	return created, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	row := r.db.sql.QueryRowContext(
		ctx,
		`
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1
		`,
		email,
	)

	var found user.User
	err := row.Scan(&found.ID, &found.Email, &found.PasswordHash, &found.CreatedAt)
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
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE id = $1
		`,
		id,
	)

	var found user.User
	err := row.Scan(&found.ID, &found.Email, &found.PasswordHash, &found.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return user.User{}, user.ErrUserNotFound
		}
		return user.User{}, err
	}

	return found, nil
}
