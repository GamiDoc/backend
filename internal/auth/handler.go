package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appmiddleware "github.com/gamidoc/backend/internal/http/middleware"
	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	"github.com/go-chi/chi/v5"
)

const refreshCookieName = "refresh_token"

type Handler struct {
	service      *Service
	tokenManager *token.Manager
	blacklist    *token.Blacklist
	refreshTTL   time.Duration
	secureCookie bool
}

func NewHandler(service *Service, tokenManager *token.Manager, blacklist *token.Blacklist, refreshTTL time.Duration, secureCookie bool) *Handler {
	return &Handler{
		service:      service,
		tokenManager: tokenManager,
		blacklist:    blacklist,
		refreshTTL:   refreshTTL,
		secureCookie: secureCookie,
	}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Post("/logout", h.logout)
	r.Post("/verify-email", h.verifyEmail)
	r.Post("/resend-verification", h.resendVerification)
	r.Post("/forgot-password", h.forgotPassword)
	r.Post("/reset-password/check", h.checkResetToken)
	r.Post("/reset-password", h.resetPassword)
	r.With(appmiddleware.RequireAuth(h.tokenManager, h.blacklist)).Get("/me", h.me)

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
			response.WriteError(w, http.StatusBadRequest, "INVALID_PASSWORD", PasswordRulesText, map[string]any{"field": "password"})
		case errors.Is(err, ErrEmailAlreadyExists):
			response.WriteError(w, http.StatusBadRequest, "EMAIL_ALREADY_EXISTS", "Email already registered", map[string]any{"field": "email"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	if result.VerificationRequired {
		response.WriteJSON(w, http.StatusCreated, map[string]any{
			"verification_required": true,
			"email":                 result.User.Email,
		})
		return
	}

	h.setRefreshCookie(w, result.RefreshToken)
	response.WriteJSON(w, http.StatusCreated, map[string]any{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
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
		case errors.Is(err, ErrInvalidCredentials), errors.Is(err, user.ErrUserNotFound):
			response.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid credentials", nil)
		case errors.Is(err, ErrEmailNotVerified):
			response.WriteError(w, http.StatusForbidden, "EMAIL_NOT_VERIFIED", "Confirm your email address before logging in", map[string]any{"field": "email"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	h.setRefreshCookie(w, result.RefreshToken)
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing refresh token", nil)
		return
	}

	result, err := h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		h.clearRefreshCookie(w)
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid or expired refresh token", nil)
		return
	}

	h.setRefreshCookie(w, result.RefreshToken)
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	// Extract access token from Authorization header
	accessToken := ""
	if header := r.Header.Get("Authorization"); header != "" {
		parts := strings.SplitN(header, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			accessToken = parts[1]
		}
	}

	// Extract refresh token from cookie
	refreshToken := ""
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		refreshToken = cookie.Value
	}

	_ = h.service.Logout(r.Context(), accessToken, refreshToken)

	h.clearRefreshCookie(w)
	response.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.GetAuthUserID(r.Context())
	if userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
		return
	}

	currentUser, err := h.service.Me(r.Context(), userID)
	if err != nil {
		if errors.Is(err, user.ErrUserNotFound) {
			response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, currentUser)
}

func (h *Handler) setRefreshCookie(w http.ResponseWriter, tokenValue string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    tokenValue,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
	})
}

func (h *Handler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}

type tokenInput struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type emailInput struct {
	Email string `json:"email"`
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var input tokenInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Token) == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	result, err := h.service.VerifyEmail(r.Context(), input.Token)
	if err != nil {
		h.writeTokenError(w, err)
		return
	}
	h.writeAuth(w, result)
}

func (h *Handler) resendVerification(w http.ResponseWriter, r *http.Request) {
	var input emailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	if err := h.service.ResendVerification(r.Context(), input.Email); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var input emailInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	if err := h.service.ForgotPassword(r.Context(), input.Email); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) checkResetToken(w http.ResponseWriter, r *http.Request) {
	var input tokenInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Token) == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	if err := h.service.CheckResetToken(r.Context(), input.Token); err != nil {
		h.writeTokenError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var input tokenInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || strings.TrimSpace(input.Token) == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	result, err := h.service.ResetPassword(r.Context(), input.Token, input.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidPassword) {
			response.WriteError(w, http.StatusBadRequest, "INVALID_PASSWORD", PasswordRulesText, map[string]any{"field": "password"})
			return
		}
		h.writeTokenError(w, err)
		return
	}
	h.writeAuth(w, result)
}

func (h *Handler) writeTokenError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrInvalidToken) {
		response.WriteError(w, http.StatusBadRequest, "INVALID_TOKEN", "This link is invalid or has expired", nil)
		return
	}
	response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
}

func (h *Handler) writeAuth(w http.ResponseWriter, result AuthResult) {
	h.setRefreshCookie(w, result.RefreshToken)
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"access_token": result.AccessToken,
		"user":         result.User,
	})
}
