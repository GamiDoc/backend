package project

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	appmiddleware "github.com/yifen9/gamidoc-backend/internal/http/middleware"
	"github.com/yifen9/gamidoc-backend/internal/http/response"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{projectId}", h.get)
	r.Patch("/{projectId}", h.update)
	r.Delete("/{projectId}", h.delete)
	r.Put("/{projectId}/wizard/step/{stepNumber}", h.saveStep)
	r.Post("/{projectId}/wizard/recommendations", h.recommend)

	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.GetAuthUserID(r.Context())
	if userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
		return
	}

	var input CreateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	created, err := h.service.Create(r.Context(), userID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidProjectName):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_NAME", "Project name is required", map[string]any{"field": "name"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	userID := appmiddleware.GetAuthUserID(r.Context())
	if userID == "" {
		response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Unauthorized", nil)
		return
	}

	projects, err := h.service.List(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"projects": projects,
		"total":    len(projects),
	})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
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

	found, err := h.service.Get(r.Context(), userID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, found)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
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

	var input UpdateInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	updated, err := h.service.Update(r.Context(), userID, projectID, input)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		case errors.Is(err, ErrInvalidProjectName):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PROJECT_NAME", "Project name is required", map[string]any{"field": "name"})
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
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

	err := h.service.Delete(r.Context(), userID, projectID)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) saveStep(w http.ResponseWriter, r *http.Request) {
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

	stepValue := chi.URLParam(r, "stepNumber")
	stepNumber, err := strconv.Atoi(stepValue)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_STEP_NUMBER", "Invalid step number", nil)
		return
	}

	var input SaveStepInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	updated, err := h.service.SaveStep(r.Context(), userID, projectID, stepNumber, input.StepData)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		case errors.Is(err, wizard.ErrInvalidStepNumber):
			response.WriteError(w, http.StatusBadRequest, "INVALID_STEP_NUMBER", "Invalid step number", nil)
		case errors.Is(err, wizard.ErrInvalidStepData):
			response.WriteError(w, http.StatusBadRequest, "INVALID_STEP_DATA", "Invalid step data", nil)
		case errors.Is(err, wizard.ErrStepPrerequisiteNotMet):
			response.WriteError(w, http.StatusBadRequest, "STEP_PREREQUISITE_NOT_MET", "Previous steps must be completed first", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"projectId":    updated.ID,
		"stepNumber":   stepNumber,
		"stepData":     updated.Wizard.Steps[strconv.Itoa(stepNumber)],
		"updatedAt":    updated.UpdatedAt,
		"wizardStatus": updated.Wizard,
	})
}

func (h *Handler) recommend(w http.ResponseWriter, r *http.Request) {
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
		ForStep int `json:"forStep"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	result, err := h.service.Recommend(r.Context(), userID, projectID, input.ForStep)
	if err != nil {
		switch {
		case errors.Is(err, ErrProjectNotFound):
			response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
		case errors.Is(err, ErrForbiddenProject):
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
		case errors.Is(err, recommendation.ErrInvalidRecommendationStep):
			response.WriteError(w, http.StatusBadRequest, "INVALID_RECOMMENDATION_STEP", "Invalid recommendation step", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}
