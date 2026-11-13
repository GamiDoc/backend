package pdf

import (
	"encoding/json"
	"errors"
	"github.com/gamidoc/backend/internal/docname"
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

type generateRequest struct {
	NotifyEmail string          `json:"notifyEmail"`
	Template    *CustomTemplate `json:"template,omitempty"`
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

	var input generateRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.GenerateProjectPDFWithOptions(r.Context(), userID, projectID, GenerateOptions{
		NotifyEmail: input.NotifyEmail,
		Template:    input.Template,
	})
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
		case errors.Is(err, ErrInvalidPDFTemplate):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PDF_TEMPLATE", "Invalid PDF template", map[string]any{"field": "template"})
		case errors.Is(err, ErrPDFRendererUnavailable):
			response.WriteError(w, http.StatusServiceUnavailable, "PDF_RENDERER_UNAVAILABLE", "Custom PDF renderer is not configured", nil)
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

	var input generateRequest
	if err := decodeOptionalJSON(r, &input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.GenerateSessionPDFWithOptions(r.Context(), sessionID, GenerateOptions{
		NotifyEmail: input.NotifyEmail,
		Template:    input.Template,
	})
	if err != nil {
		switch {
		case errors.Is(err, session.ErrSessionNotFound):
			response.WriteError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found or expired", nil)
		case errors.Is(err, wizard.ErrIncompleteWizard):
			response.WriteError(w, http.StatusBadRequest, "WIZARD_INCOMPLETE", "Wizard is incomplete", nil)
		case errors.Is(err, ErrInvalidNotifyEmail):
			response.WriteError(w, http.StatusBadRequest, "INVALID_NOTIFY_EMAIL", "Invalid notify email", map[string]any{"field": "notifyEmail"})
		case errors.Is(err, ErrInvalidPDFTemplate):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PDF_TEMPLATE", "Invalid PDF template", map[string]any{"field": "template"})
		case errors.Is(err, ErrPDFRendererUnavailable):
			response.WriteError(w, http.StatusServiceUnavailable, "PDF_RENDERER_UNAVAILABLE", "Custom PDF renderer is not configured", nil)
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

func (h *Handler) ProjectDownload(w http.ResponseWriter, r *http.Request) {
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

	data, fileName, err := h.service.DownloadProjectPDF(r.Context(), userID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, project.ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, project.ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		case errors.Is(err, ErrPDFNotFound):
			response.WriteError(w, http.StatusNotFound, "PDF_NOT_FOUND", "PDF not yet generated", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) SessionDownload(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	if sessionID == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_SESSION_ID", "Invalid session id", nil)
		return
	}

	data, fileName, err := h.service.DownloadSessionPDF(r.Context(), sessionID)
	if err != nil {
		switch {
		case errors.Is(err, session.ErrSessionNotFound):
			response.WriteError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found or expired", nil)
		case errors.Is(err, ErrPDFNotFound):
			response.WriteError(w, http.StatusNotFound, "PDF_NOT_FOUND", "PDF not yet generated", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "*")
	key = strings.TrimLeft(key, "/")

	data, err := h.service.Download(r.Context(), key)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "PDF_NOT_FOUND", "PDF not found", nil)
		return
	}

	// Design images are stored next to the PDFs: serve them inline with their own type.
	if contentType, ok := imageContentType(key); ok {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+docname.FromKey(key, "document.pdf")+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func imageContentType(key string) (string, bool) {
	switch {
	case strings.HasSuffix(key, ".png"):
		return "image/png", true
	case strings.HasSuffix(key, ".jpg"):
		return "image/jpeg", true
	}
	return "", false
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
