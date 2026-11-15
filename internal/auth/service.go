package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidEmail = errors.New("invalid email")
var ErrInvalidPassword = errors.New("invalid password")
var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrEmailNotVerified = errors.New("email not verified")

type Service struct {
	users        user.Repository
	tokens       *token.Manager
	refreshStore *token.RefreshStore
	blacklist    *token.Blacklist
	// email is nil when accounts need no email verification (tests, older wiring).
	email *emailFlows
	// files and store: optional, to delete the stored files with the account.
	files AccountFiles
	store FileStore
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResult struct {
	AccessToken  string    `json:"access_token"`
	User         user.User `json:"user"`
	RefreshToken string    `json:"-"` // sent via httpOnly cookie, not in JSON body
	// VerificationRequired: the account was created but the user must open the link of
	// the verification email before logging in (no tokens are issued).
	VerificationRequired bool `json:"-"`
}

func NewService(users user.Repository, tokens *token.Manager, refreshStore *token.RefreshStore, blacklist *token.Blacklist) *Service {
	return &Service{
		users:        users,
		tokens:       tokens,
		refreshStore: refreshStore,
		blacklist:    blacklist,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password

	if _, err := mail.ParseAddress(email); err != nil {
		return AuthResult{}, ErrInvalidEmail
	}

	if err := ValidatePassword(password); err != nil {
		return AuthResult{}, err
	}

	_, err := s.users.FindByEmail(ctx, email)
	if err == nil {
		return AuthResult{}, ErrEmailAlreadyExists
	}
	if !errors.Is(err, user.ErrUserNotFound) {
		return AuthResult{}, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return AuthResult{}, err
	}

	createdUser, err := s.users.Create(ctx, user.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		return AuthResult{}, err
	}

	if s.email != nil {
		// The account works once the email is confirmed; a failed send can be retried
		// with "resend the email".
		s.email.sendVerification(ctx, createdUser)
		return AuthResult{User: createdUser, VerificationRequired: true}, nil
	}
	return s.issueTokens(ctx, createdUser)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password

	foundUser, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, user.ErrUserNotFound) {
			return AuthResult{}, err
		}
		return AuthResult{}, ErrInvalidCredentials
	}

	err = bcrypt.CompareHashAndPassword([]byte(foundUser.PasswordHash), []byte(password))
	if err != nil {
		return AuthResult{}, ErrInvalidCredentials
	}
	if s.email != nil && !foundUser.Verified() {
		return AuthResult{}, ErrEmailNotVerified
	}

	return s.issueTokens(ctx, foundUser)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (AuthResult, error) {
	userID, err := s.refreshStore.Validate(ctx, refreshToken)
	if err != nil {
		return AuthResult{}, err
	}

	// Revoke old refresh token (rotation)
	_ = s.refreshStore.Revoke(ctx, refreshToken)

	foundUser, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return AuthResult{}, err
	}

	return s.issueTokens(ctx, foundUser)
}

func (s *Service) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	// Blacklist the current access token jti
	claims, err := s.tokens.Parse(accessToken)
	if err == nil && claims.ID != "" {
		_ = s.blacklist.Add(ctx, claims.ID, claims.ExpiresAt.Time)
	}

	// Revoke the refresh token
	if refreshToken != "" {
		_ = s.refreshStore.Revoke(ctx, refreshToken)
	}

	return nil
}

func (s *Service) Me(ctx context.Context, userID string) (user.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *Service) issueTokens(ctx context.Context, u user.User) (AuthResult, error) {
	accessToken, err := s.tokens.Generate(u.ID)
	if err != nil {
		return AuthResult{}, err
	}

	refreshToken, err := s.refreshStore.GenerateAndStore(ctx, u.ID)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		AccessToken:  accessToken,
		User:         u,
		RefreshToken: refreshToken,
	}, nil
}
