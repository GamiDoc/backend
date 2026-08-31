package design

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gamidoc/backend/internal/ai"
	appmiddleware "github.com/gamidoc/backend/internal/http/middleware"
	"github.com/gamidoc/backend/internal/http/response"
	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/session"
	"github.com/go-chi/chi/v5"
)

type SessionGuard interface {
	FindByID(ctx context.Context, id string) (session.Session, error)
}

type ProjectGuard interface {
	FindByID(ctx context.Context, id string) (project.Project, error)
}

type Handler struct {
	service       *Service
	assistant     ai.Assistant
	reports       *ReportService
	sessions      SessionGuard
	sessionStates StateStore
	projects      ProjectGuard
	projectStates StateStore
}

func NewHandler(
	service *Service,
	assistant ai.Assistant,
	reports *ReportService,
	sessions SessionGuard,
	sessionStates StateStore,
	projects ProjectGuard,
	projectStates StateStore,
) *Handler {
	return &Handler{
		service:       service,
		assistant:     assistant,
		reports:       reports,
		sessions:      sessions,
		sessionStates: sessionStates,
		projects:      projects,
		projectStates: projectStates,
	}
}

type owner struct {
	kind   string
	id     string
	states StateStore
}

func (h *Handler) SessionRoutes() chi.Router {
	r := chi.NewRouter()
	h.mountShared(r, h.forSession)
	return r
}

func (h *Handler) ProjectRoutes() chi.Router {
	r := chi.NewRouter()
	h.mountShared(r, h.forProject)
	r.Get("/reports", h.forProject(h.listReports))
	r.Post("/import-session", h.forProject(h.importSession))
	return r
}

func (h *Handler) mountShared(r chi.Router, guard func(func(http.ResponseWriter, *http.Request, owner)) http.HandlerFunc) {
	r.Get("/", guard(h.getState))
	r.Put("/spark", guard(h.saveSpark))
	r.Get("/branch", guard(h.branch))
	r.Post("/path", guard(h.choosePath))
	r.Put("/section/{sectionNumber}", guard(h.saveSection))
	r.Get("/dashboard", guard(h.dashboard))
	r.Post("/generate-pdf", guard(h.generatePDF))
	r.Get("/faq/{sectionNumber}", guard(h.faq))
	r.Post("/ai/rewrite", guard(h.rewrite))
	r.Post("/ai/chat", guard(h.chat))
}

func (h *Handler) forSession(next func(http.ResponseWriter, *http.Request, owner)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := chi.URLParam(r, "sessionId")
		if sessionID == "" {
			response.WriteError(w, http.StatusBadRequest, "INVALID_SESSION_ID", "Invalid session id", nil)
			return
		}

		if _, err := h.sessions.FindByID(r.Context(), sessionID); err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				response.WriteError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found", nil)
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
			return
		}

		next(w, r, owner{kind: "sessions", id: sessionID, states: h.sessionStates})
	}
}

func (h *Handler) forProject(next func(http.ResponseWriter, *http.Request, owner)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		found, err := h.projects.FindByID(r.Context(), projectID)
		if err != nil {
			if errors.Is(err, project.ErrProjectNotFound) {
				response.WriteError(w, http.StatusNotFound, "PROJECT_NOT_FOUND", "Project not found", nil)
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
			return
		}

		if found.UserID != userID {
			response.WriteError(w, http.StatusForbidden, "FORBIDDEN", "Project does not belong to user", nil)
			return
		}

		next(w, r, owner{kind: "projects", id: projectID, states: h.projectStates})
	}
}

func decodeOptional(r *http.Request, target any) error {
	err := json.NewDecoder(r.Body).Decode(target)
	if err != nil && errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (h *Handler) getState(w http.ResponseWriter, r *http.Request, o owner) {
	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, status)
}

func (h *Handler) saveSpark(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		Spark string `json:"spark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	var prefill map[int]json.RawMessage
	var prefillErr error
	if input.Spark != "" {
		prefill, prefillErr = h.assistant.Prefill(r.Context(), input.Spark, sectionsMeta())
	}

	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	status = h.service.SaveSpark(status, input.Spark)

	applied := false
	if prefillErr == nil && len(prefill) > 0 {
		converted := make(map[string]json.RawMessage, len(prefill))
		for number, content := range prefill {
			converted[SectionKey(number)] = content
		}
		status = h.service.ApplyPrefill(status, converted)
		applied = true
	}

	if err := o.states.Save(r.Context(), o.id, status); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"designStatus":   status,
		"prefillApplied": applied,
		"prefillFailed":  prefillErr != nil,
	})
}

func (h *Handler) branch(w http.ResponseWriter, r *http.Request, o owner) {
	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	if status.Spark == "" {
		response.WriteJSON(w, http.StatusOK, map[string]any{
			"recommendedPath": PathExperienceFirst,
			"basis":           "default",
		})
		return
	}

	recommended, err := h.assistant.RecommendBranch(r.Context(), status.Spark)
	if err != nil {
		response.WriteError(w, http.StatusBadGateway, "AI_PROVIDER_ERROR", "AI provider error", nil)
		return
	}
	if recommended != PathMechanicsFirst {
		recommended = PathExperienceFirst
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"recommendedPath": recommended,
		"basis":           "spark",
	})
}

func (h *Handler) choosePath(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	updated, err := h.service.ChoosePath(status, input.Path)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidPath):
			response.WriteError(w, http.StatusBadRequest, "INVALID_PATH", "Invalid path", nil)
		case errors.Is(err, ErrPathAlreadyChosen):
			response.WriteError(w, http.StatusConflict, "PATH_ALREADY_CHOSEN", "Path already chosen", nil)
		case errors.Is(err, ErrSectionLocked):
			response.WriteError(w, http.StatusBadRequest, "SECTION_LOCKED", "Complete the Context section first", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	if err := o.states.Save(r.Context(), o.id, updated); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) saveSection(w http.ResponseWriter, r *http.Request, o owner) {
	sectionValue := chi.URLParam(r, "sectionNumber")
	sectionNumber, err := strconv.Atoi(sectionValue)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_SECTION_NUMBER", "Invalid section number", nil)
		return
	}

	var input struct {
		Content  json.RawMessage `json:"content"`
		Complete *bool           `json:"complete"`
		Skip     bool            `json:"skip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	updated, err := h.service.SaveSection(status, sectionNumber, input.Content, input.Complete, input.Skip)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidSectionNumber):
			response.WriteError(w, http.StatusBadRequest, "INVALID_SECTION_NUMBER", "Invalid section number", nil)
		case errors.Is(err, ErrInvalidSectionData):
			response.WriteError(w, http.StatusBadRequest, "INVALID_SECTION_DATA", "Invalid section data", nil)
		case errors.Is(err, ErrSectionLocked):
			response.WriteError(w, http.StatusBadRequest, "SECTION_LOCKED", "Sections must be traversed in order on the first pass", nil)
		case errors.Is(err, ErrPathNotChosen):
			response.WriteError(w, http.StatusBadRequest, "PATH_NOT_CHOSEN", "Choose a path after the Context section", nil)
		default:
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		}
		return
	}

	if err := o.states.Save(r.Context(), o.id, updated); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"sectionNumber": sectionNumber,
		"section":       updated.Section(sectionNumber),
		"designStatus":  updated,
	})
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, o owner) {
	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	dashboard, err := h.service.Dashboard(status)
	if err != nil {
		if errors.Is(err, ErrDashboardLocked) {
			response.WriteError(w, http.StatusForbidden, "DASHBOARD_LOCKED", "Complete a full first pass with at least one filled section first", nil)
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, dashboard)
}

func (h *Handler) generatePDF(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct{}
	if err := decodeOptional(r, &input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	if _, err := h.service.Dashboard(status); err != nil {
		response.WriteError(w, http.StatusForbidden, "DASHBOARD_LOCKED", "Complete a full first pass with at least one filled section first", nil)
		return
	}

	generated, err := h.reports.Generate(r.Context(), o.kind, o.id, status)
	if err != nil {
		if errors.Is(err, ErrAssistant) {
			response.WriteError(w, http.StatusBadGateway, "AI_PROVIDER_ERROR", "AI provider error", nil)
			return
		}
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	if o.kind == "sessions" {
		status.Reports = append(status.Reports, generated.Standard, generated.Enhanced)
		if err := o.states.Save(r.Context(), o.id, status); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
			return
		}
	}

	response.WriteJSON(w, http.StatusOK, generated)
}

func (h *Handler) listReports(w http.ResponseWriter, r *http.Request, o owner) {
	reports, err := h.reports.List(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"reports": reports,
		"total":   len(reports),
	})
}

func (h *Handler) importSession(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.SessionID == "" {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	incoming, err := h.sessionStates.Get(r.Context(), input.SessionID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	if !incoming.HasContent() && incoming.Spark == "" {
		if _, err := h.sessions.FindByID(r.Context(), input.SessionID); err != nil {
			if errors.Is(err, session.ErrSessionNotFound) {
				response.WriteError(w, http.StatusNotFound, "SESSION_NOT_FOUND", "Session not found", nil)
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
			return
		}
		response.WriteError(w, http.StatusConflict, "SESSION_DESIGN_EMPTY", "Session has no design content to import", nil)
		return
	}

	existing, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	if existing.HasContent() {
		response.WriteError(w, http.StatusConflict, "DESIGN_NOT_EMPTY", "Project already has design content", nil)
		return
	}

	if len(incoming.Reports) > 0 {
		if err := h.reports.Persist(r.Context(), o.id, incoming.Reports); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
			return
		}
		incoming.Reports = nil
	}

	if err := o.states.Save(r.Context(), o.id, incoming); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, incoming)
}

func (h *Handler) faq(w http.ResponseWriter, r *http.Request, o owner) {
	sectionValue := chi.URLParam(r, "sectionNumber")
	sectionNumber, err := strconv.Atoi(sectionValue)
	if err != nil || sectionNumber < 1 || sectionNumber > SectionCount {
		response.WriteError(w, http.StatusBadRequest, "INVALID_SECTION_NUMBER", "Invalid section number", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"sectionNumber": sectionNumber,
		"faq":           SectionFAQ(sectionNumber),
	})
}

func (h *Handler) rewrite(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}

	rewritten, err := h.assistant.Rewrite(r.Context(), input.Text)
	if err != nil {
		if errors.Is(err, ai.ErrEmptyText) {
			response.WriteError(w, http.StatusBadRequest, "EMPTY_TEXT", "Text is required", nil)
			return
		}
		response.WriteError(w, http.StatusBadGateway, "AI_PROVIDER_ERROR", "AI provider error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"text":         rewritten,
		"previousText": input.Text,
	})
}

func (h *Handler) chat(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		SectionNumber int    `json:"sectionNumber"`
		Message       string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	if input.SectionNumber < 1 || input.SectionNumber > SectionCount {
		response.WriteError(w, http.StatusBadRequest, "INVALID_SECTION_NUMBER", "Invalid section number", nil)
		return
	}

	reply, err := h.assistant.Chat(r.Context(), ai.Section{
		Number: input.SectionNumber,
		Name:   SectionName(input.SectionNumber),
	}, input.Message)
	if err != nil {
		response.WriteError(w, http.StatusBadGateway, "AI_PROVIDER_ERROR", "AI provider error", nil)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"sectionNumber": input.SectionNumber,
		"reply":         reply,
		"faq":           SectionFAQ(input.SectionNumber),
	})
}

func sectionsMeta() []ai.Section {
	var sections []ai.Section
	for number := 1; number <= SectionCount; number++ {
		sections = append(sections, ai.Section{
			Number: number,
			Name:   SectionName(number),
		})
	}
	return sections
}
