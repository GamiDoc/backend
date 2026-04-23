package auth

import (
	"encoding/json"
	"errors"
	"net/http"

	appmiddleware "github.com/gamidoc/backend/internal/http/middleware"
	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/storage/postgres"
	"github.com/gamidoc/backend/internal/token"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service      *Service
	tokenManager *token.Manager
}

func NewHandler(service *Service, tokenManager *token.Manager) *Handler {
	return &Handler{
		service:      service,
		tokenManager: tokenManager,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.With(appmiddleware.RequireAuth(h.tokenManager)).Get("/me", h.me)

	return r
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.Register(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			response.WriteError(w, http.StatusBadRequest, "INVALID_EMAIL", "Invalid email", map[string]any{"field": "email"})
		case errors.Is(err, ErrInvalidPassword):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PASSWORD", "Password must be at least 8 characters", map[string]any{"field": "password"})
		case errors.Is(err, ErrEmailAlreadyExists):
			response.WriteError(w, http.StatusBadRequest, "EMAIL_ALREADY_EXISTS", "Email already registered", map[string]any{"field": "email"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.Login(r.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials), errors.Is(err, postgres.ErrUserNotFound):
			response.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid credentials", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.GetAuthUserID(r.Context())
	if userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
		return
	}

	currentUser, err := h.service.Me(r.Context(), userID)
	if err != nil {
		if errors.Is(err, postgres.ErrUserNotFound) {
			response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, currentUser)
}
