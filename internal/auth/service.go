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

type Service struct {
	users  user.Repository
	tokens *token.Manager
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
	Token string    `json:"token"`
	User  user.User `json:"user"`
}

func NewService(users user.Repository, tokens *token.Manager) *Service {
	return &Service{
		users:  users,
		tokens: tokens,
	}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResult, error) {
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := input.Password

	if _, err := mail.ParseAddress(email); err != nil {
		return AuthResult{}, ErrInvalidEmail
	}

	if len(password) < 8 {
		return AuthResult{}, ErrInvalidPassword
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

	tokenValue, err := s.tokens.Generate(createdUser.ID, createdUser.Email)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token: tokenValue,
		User:  createdUser,
	}, nil
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

	tokenValue, err := s.tokens.Generate(foundUser.ID, foundUser.Email)
	if err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		Token: tokenValue,
		User:  foundUser,
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (user.User, error) {
	return s.users.FindByID(ctx, userID)
}
