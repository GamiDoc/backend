package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	goredis "github.com/redis/go-redis/v9"
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

func (r *fakeUserRepository) MarkEmailVerified(ctx context.Context, id string) error {
	u, ok := r.usersByID[id]
	if !ok {
		return user.ErrUserNotFound
	}
	now := time.Now()
	u.EmailVerifiedAt = &now
	r.usersByID[id] = u
	r.usersByEmail[u.Email] = u
	return nil
}

func (r *fakeUserRepository) UpdatePassword(ctx context.Context, id string, passwordHash string) error {
	u, ok := r.usersByID[id]
	if !ok {
		return user.ErrUserNotFound
	}
	u.PasswordHash = passwordHash
	r.usersByID[id] = u
	r.usersByEmail[u.Email] = u
	return nil
}

func (r *fakeUserRepository) Delete(ctx context.Context, id string) error {
	u, ok := r.usersByID[id]
	if !ok {
		return user.ErrUserNotFound
	}
	delete(r.usersByID, id)
	delete(r.usersByEmail, u.Email)
	return nil
}

func newTestService(t *testing.T) (*Service, *token.Manager) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})

	repo := &fakeUserRepository{
		usersByEmail: map[string]user.User{},
		usersByID:    map[string]user.User{},
	}
	tokens := token.NewManager("secret", time.Hour)
	refreshStore := token.NewRefreshStore(rdb, 7*24*time.Hour)
	blacklist := token.NewBlacklist(rdb)

	return NewService(repo, tokens, refreshStore, blacklist), tokens
}

func TestRegister(t *testing.T) {
	service, _ := newTestService(t)

	result, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("expected access token to be set")
	}

	if result.RefreshToken == "" {
		t.Fatal("expected refresh token to be set")
	}

	if result.User.Email != "test@example.com" {
		t.Fatalf("expected email %q, got %q", "test@example.com", result.User.Email)
	}
}

func TestRegisterReturnsFindByEmailError(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	repo := &fakeUserRepository{findByEmailErr: lookupErr}
	tokens := token.NewManager("secret", time.Hour)
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	refreshStore := token.NewRefreshStore(rdb, 7*24*time.Hour)
	blacklist := token.NewBlacklist(rdb)
	service := NewService(repo, tokens, refreshStore, blacklist)

	_, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}

	if repo.createCalls != 0 {
		t.Fatalf("expected Create not to be called, got %d calls", repo.createCalls)
	}
}

func TestLogin(t *testing.T) {
	service, _ := newTestService(t)

	registered, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	result, err := service.Login(context.Background(), LoginInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.AccessToken == "" {
		t.Fatal("expected access token to be set")
	}

	if result.User.ID != registered.User.ID {
		t.Fatalf("expected user id %q, got %q", registered.User.ID, result.User.ID)
	}
}

func TestRefreshRotation(t *testing.T) {
	service, _ := newTestService(t)

	registered, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Refresh using the original refresh token
	refreshed, err := service.Refresh(context.Background(), registered.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if refreshed.AccessToken == "" {
		t.Fatal("expected new access token")
	}

	// Old refresh token should be revoked (rotation)
	_, err = service.Refresh(context.Background(), registered.RefreshToken)
	if err == nil {
		t.Fatal("expected error reusing old refresh token")
	}

	// New refresh token should work
	_, err = service.Refresh(context.Background(), refreshed.RefreshToken)
	if err != nil {
		t.Fatalf("expected new refresh token to work, got %v", err)
	}
}

func TestLogoutBlacklistsAccess(t *testing.T) {
	service, tokens := newTestService(t)

	registered, err := service.Register(context.Background(), RegisterInput{
		Email:    "test@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Logout should blacklist the access token's jti
	err = service.Logout(context.Background(), registered.AccessToken, registered.RefreshToken)
	if err != nil {
		t.Fatalf("logout: %v", err)
	}

	// Parse access token to get jti
	claims, err := tokens.Parse(registered.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	revoked, err := service.blacklist.IsBlacklisted(context.Background(), claims.ID)
	if err != nil {
		t.Fatalf("blacklist check: %v", err)
	}
	if !revoked {
		t.Fatal("expected jti to be blacklisted after logout")
	}

	// Refresh token should also be revoked
	_, err = service.Refresh(context.Background(), registered.RefreshToken)
	if err == nil {
		t.Fatal("expected refresh token to be revoked after logout")
	}
}
