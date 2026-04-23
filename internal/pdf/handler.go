package pdf

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	appmiddleware "github.com/gamidoc/backend/internal/http/middleware"
	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/session"
	"github.com/gamidoc/backend/internal/wizard"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ProjectGenerate(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.GetAuthUserID(r.Context())
	if userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
		return
	}

	projectID := chi.URLParam(r, "projectId")
	if projectID == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_ID", "Invalid project id", nil)
		return
	}

	var input struct {
		NotifyEmail string `json:"notifyEmail"`
	}
	if err := decodeOptionalJSON(r, &input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.GenerateProjectPDF(r.Context(), userID, projectID, input.NotifyEmail)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, project.ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		case errors.Is(err, wizard.ErrIncompleteWizard):
			response.WriteError(w, http.StatusBadRequest, "WIZARD_INCOMPLETE", "Wizard is incomplete", nil)
		case errors.Is(err, ErrInvalidNotifyEmail):
			response.WriteError(w, http.StatusBadRequest, "INVALID_NOTIFY_EMAIL", "Invalid notify email", map[string]any{"field": "notifyEmail"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "PDF_GENERATION_FAILED", "PDF generation failed", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"pdfUrl": result.URL,
		"email":  result.Email,
	})
}

func (h *Handler) SessionGenerate(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_SESSION_ID", "Invalid session id", nil)
		return
	}

	var input struct {
		NotifyEmail string `json:"notifyEmail"`
	}
	if err := decodeOptionalJSON(r, &input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.GenerateSessionPDF(r.Context(), sessionID, input.NotifyEmail)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrSessionNotFound):
			response.WriteError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found or expired", nil)
		case errors.Is(err, wizard.ErrIncompleteWizard):
			response.WriteError(w, http.StatusBadRequest, "WIZARD_INCOMPLETE", "Wizard is incomplete", nil)
		case errors.Is(err, ErrInvalidNotifyEmail):
			response.WriteError(w, http.StatusBadRequest, "INVALID_NOTIFY_EMAIL", "Invalid notify email", map[string]any{"field": "notifyEmail"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "PDF_GENERATION_FAILED", "PDF generation failed", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"pdfUrl": result.URL,
		"email":  result.Email,
	})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "*")
	key = strings.TrimLeft(key, "/")

	data, err := h.service.Download(r.Context(), key)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "PDF_NOT_FOUND", "PDF not found", nil)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="evaluation-plan.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func decodeOptionalJSON(r *http.Request, value any) error {
	if r.Body == nil {
		return nil
	}

	err := json.NewDecoder(r.Body).Decode(value)
	if err == nil {
		return nil
	}

	if errors.Is(err, io.EOF) {
		return nil
	}

	return err
}
