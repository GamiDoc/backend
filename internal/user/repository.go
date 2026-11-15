package user

import (
	"context"
	"errors"
)

var ErrUserNotFound = errors.New("user not found")

type Repository interface {
	Create(ctx context.Context, user User) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
	MarkEmailVerified(ctx context.Context, id string) error
	UpdatePassword(ctx context.Context, id string, passwordHash string) error
	// Delete removes the user with their projects (and everything in them).
	Delete(ctx context.Context, id string) error
}
