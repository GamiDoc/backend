package design

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gamidoc/backend/internal/docname"
	"github.com/gamidoc/backend/internal/http/response"
)

// DefaultDesignTitle names a design without a title nor a project name.
const DefaultDesignTitle = docname.DefaultTitle

// MaxTitleLength bounds the title typed in the overview.
const MaxTitleLength = 120

// DesignTitle is the name printed on the documents and used in their file names:
// the title set in the overview, else the project name, else DefaultDesignTitle.
func DesignTitle(status Status, projectName string) string {
	if title := strings.TrimSpace(status.Title); title != "" {
		return title
	}
	if name := strings.TrimSpace(projectName); name != "" {
		return name
	}
	return DefaultDesignTitle
}

// DocumentFileName is "<Title>_<document type>.pdf", e.g. "GreenStep_design-report.pdf".
func DocumentFileName(title string, format string) string {
	if format == ReportFormatGDD {
		return docname.FileName(title, docname.GameDesignDocument)
	}
	return docname.FileName(title, docname.DesignReport)
}

// setDocumentTitle puts the design title on the document (the AI may invent another name).
func setDocumentTitle(document any, title string) {
	switch doc := document.(type) {
	case *ReportDoc:
		doc.Title = title
	case *GDDDoc:
		doc.Title = title
	}
}

// saveTitle stores the title shown on the documents (empty = back to the default).
func (h *Handler) saveTitle(w http.ResponseWriter, r *http.Request, o owner) {
	var input struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid request body", nil)
		return
	}
	title := strings.TrimSpace(input.Title)
	if len([]rune(title)) > MaxTitleLength {
		response.WriteError(w, http.StatusBadRequest, "TITLE_TOO_LONG", "Title too long", nil)
		return
	}

	unlock := h.lock(o)
	defer unlock()
	status, err := o.states.Get(r.Context(), o.id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	status.Title = title
	if err := o.states.Save(r.Context(), o.id, status); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", nil)
		return
	}
	response.WriteJSON(w, http.StatusOK, status)
}

// projectName is the name of the owner's project ("" for a session or when not found).
func (h *Handler) projectName(ctx context.Context, o owner) string {
	if o.kind != "projects" || h.projects == nil {
		return ""
	}
	found, err := h.projects.FindByID(ctx, o.id)
	if err != nil {
		return ""
	}
	return found.Name
}
