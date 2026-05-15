package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
)

type fakeUserRepository struct {
	usersByEmail   map[string]user.User
	usersByID      map[string]user.User
	createErr      error
	findByEmailErr error
	createCalls    int
}

func (r *fakeUserRepository) Create(ctx context.Context, input user.User) (user.User, error) {
	r.createCalls++
	if r.createErr != nil {
		return user.User{}, r.createErr
	}
	input.CreatedAt = time.Now()
	if r.usersByEmail == nil {
		r.usersByEmail = map[string]user.User{}
	}
	if r.usersByID == nil {
		r.usersByID = map[string]user.User{}
	}
	r.usersByEmail[input.Email] = input
	r.usersByID[input.ID] = input
	return input, nil
}

func (r *fakeUserRepository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	if r.findByEmailErr != nil {
		return user.User{}, r.findByEmailErr
	}
	u, ok := r.usersByEmail[email]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}

func (r *fakeUserRepository) FindByID(ctx context.Context, id string) (user.User, error) {
	u, ok := r.usersByID[id]
	if !ok {
		return user.User{}, user.ErrUserNotFound
	}
	return u, nil
}

func TestRegister(t *testing.T) {
	repo := &fakeUserRepository{
		usersByEmail: map[string]user.User{},
		usersByID:    map[string]user.User{},
	}
	tokens := token.NewManager("secret", time.Hour)
	service := NewService(repo, tokens)

	result, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Token == "" {
		t.Fatal("expected token to be set")
	}

	if result.User.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", result.User.Email)
	}
}

func TestRegisterReturnsFindByEmailError(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	repo := &fakeUserRepository{findByEmailErr: lookupErr}
	tokens := token.NewManager("secret", time.Hour)
	service := NewService(repo, tokens)

	_, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}

	if repo.createCalls != 0 {
		t.Fatalf("expected Create not to be called, got %d calls", repo.createCalls)
	}
}

func TestLogin(t *testing.T) {
	repo := &fakeUserRepository{
		usersByEmail: map[string]user.User{},
		usersByID:    map[string]user.User{},
	}
	tokens := token.NewManager("secret", time.Hour)
	service := NewService(repo, tokens)

	registered, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Token == "" {
		t.Fatal("expected token to be set")
	}

	if result.User.ID != registered.User.ID {
		t.Fatalf("expected user id %q, got %q", registered.User.ID, result.User.ID)
	}
}
