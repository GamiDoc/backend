package http

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gamidoc/backend/internal/auth"
	"github.com/gamidoc/backend/internal/mailer"
	"github.com/gamidoc/backend/internal/pdf"
	"github.com/gamidoc/backend/internal/project"
	"github.com/gamidoc/backend/internal/recommendation"
	"github.com/gamidoc/backend/internal/session"
	"github.com/gamidoc/backend/internal/storage/objectstore"
	"github.com/gamidoc/backend/internal/token"
	"github.com/gamidoc/backend/internal/user"
	"github.com/gamidoc/backend/internal/wizard"
)

type fakePostgres struct {
	readyErr error
}

func (f *fakePostgres) Ready(ctx context.Context) error {
	return f.readyErr
}

type fakeRedis struct {
	readyErr error
}

func (f *fakeRedis) Ready(ctx context.Context) error {
	return f.readyErr
}

type fakeUserRepository struct {
	usersByEmail map[string]user.User
	usersByID    map[string]user.User
}

func (r *fakeUserRepository) Create(ctx context.Context, input user.User) (user.User, error) {
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

type fakeProjectRepository struct {
	items []project.Project
	byID  map[string]project.Project
}

func (r *fakeProjectRepository) Create(ctx context.Context, input project.Project) (project.Project, error) {
	input.CreatedAt = time.Now()
	input.UpdatedAt = input.CreatedAt
	if r.byID == nil {
		r.byID = map[string]project.Project{}
	}
	r.items = append(r.items, input)
	r.byID[input.ID] = input
	return input, nil
}

func (r *fakeProjectRepository) ListByUserID(ctx context.Context, userID string) ([]project.Project, error) {
	var result []project.Project
	for _, item := range r.items {
		if item.UserID == userID {
			result = append(result, item)
		}
	}
	return result, nil
}

func (r *fakeProjectRepository) FindByID(ctx context.Context, id string) (project.Project, error) {
	item, ok := r.byID[id]
	if !ok {
		return project.Project{}, project.ErrProjectNotFound
	}
	return item, nil
}

func (r *fakeProjectRepository) UpdateWizard(ctx context.Context, projectID string, status wizard.Status) (project.Project, error) {
	item, ok := r.byID[projectID]
	if !ok {
		return project.Project{}, project.ErrProjectNotFound
	}
	item.Wizard = status
	item.PDFURL = nil
	item.UpdatedAt = time.Now()
	r.byID[projectID] = item
	for i := range r.items {
		if r.items[i].ID == projectID {
			r.items[i] = item
		}
	}
	return item, nil
}

func (r *fakeProjectRepository) UpdateInfo(ctx context.Context, projectID string, name string, description string) (project.Project, error) {
	item, ok := r.byID[projectID]
	if !ok {
		return project.Project{}, project.ErrProjectNotFound
	}
	item.Name = name
	item.Description = description
	item.UpdatedAt = time.Now()
	r.byID[projectID] = item
	for i := range r.items {
		if r.items[i].ID == projectID {
			r.items[i] = item
		}
	}
	return item, nil
}

func (r *fakeProjectRepository) Delete(ctx context.Context, projectID string) error {
	item, ok := r.byID[projectID]
	if !ok {
		return project.ErrProjectNotFound
	}
	delete(r.byID, projectID)

	var filtered []project.Project
	for _, candidate := range r.items {
		if candidate.ID != item.ID {
			filtered = append(filtered, candidate)
		}
	}
	r.items = filtered
	return nil
}

func (r *fakeProjectRepository) UpdatePDFURL(ctx context.Context, projectID string, pdfURL string) (project.Project, error) {
	item, ok := r.byID[projectID]
	if !ok {
		return project.Project{}, project.ErrProjectNotFound
	}
	item.PDFURL = &pdfURL
	item.UpdatedAt = time.Now()
	r.byID[projectID] = item
	for i := range r.items {
		if r.items[i].ID == projectID {
			r.items[i] = item
		}
	}
	return item, nil
}

type fakeSessionRepository struct {
	byID map[string]session.Session
}

func (r *fakeSessionRepository) Create(ctx context.Context, input session.Session) (session.Session, error) {
	if r.byID == nil {
		r.byID = map[string]session.Session{}
	}
	r.byID[input.ID] = input
	return input, nil
}

func (r *fakeSessionRepository) FindByID(ctx context.Context, id string) (session.Session, error) {
	item, ok := r.byID[id]
	if !ok {
		return session.Session{}, session.ErrSessionNotFound
	}
	return item, nil
}

func (r *fakeSessionRepository) FindWizardByID(ctx context.Context, id string) (wizard.Status, error) {
	item, ok := r.byID[id]
	if !ok {
		return wizard.Status{}, session.ErrSessionNotFound
	}
	return item.Wizard, nil
}

func (r *fakeSessionRepository) UpdateWizard(ctx context.Context, id string, status wizard.Status) (session.Session, error) {
	item, ok := r.byID[id]
	if !ok {
		return session.Session{}, session.ErrSessionNotFound
	}
	item.Wizard = status
	item.PDFURL = nil
	r.byID[id] = item
	return item, nil
}

func (r *fakeSessionRepository) UpdatePDFURL(ctx context.Context, id string, pdfURL string) (session.Session, error) {
	item, ok := r.byID[id]
	if !ok {
		return session.Session{}, session.ErrSessionNotFound
	}
	item.PDFURL = &pdfURL
	r.byID[id] = item
	return item, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func testRecommendationService() *recommendation.Service {
	engine := recommendation.NewEngine([]recommendation.Rule{
		{
			ForStep:                 2,
			RequiredEvaluationGoals: []string{"Usability & Playability"},
			Recommendations: []recommendation.Recommendation{
				{ID: "think-aloud", Name: "Think-aloud testing", Priority: "Recommended"},
				{ID: "surveys", Name: "Surveys & Questionnaires", Priority: "Recommended"},
			},
		},
		{
			ForStep:                   2,
			RequiredEvaluationGoals:   []string{"Usability & Playability"},
			RequiredProjectTypes:      []string{"Concept test"},
			RequiredParticipants:      []string{"Limited set of participants"},
			RequiredDevelopmentStages: []string{"Concept idea"},
			Recommendations: []recommendation.Recommendation{
				{ID: "expert-review", Name: "Expert review", Priority: "Engagement"},
			},
		},
		{
			ForStep:         3,
			RequiredMethods: []string{"surveys"},
			Recommendations: []recommendation.Recommendation{
				{ID: "useq-like", Name: "USEQ-Like", Priority: "Recommended"},
				{ID: "sus", Name: "SUS", Priority: "Engagement"},
			},
		},
		{
			ForStep:         3,
			RequiredMethods: []string{"think-aloud"},
			Recommendations: []recommendation.Recommendation{
				{ID: "observation-grid", Name: "Observation Grid", Priority: "Recommended"},
			},
		},
	})
	return recommendation.NewService(engine)
}

func testTokenManager() *token.Manager {
	return token.NewManager("secret", time.Hour)
}

func authToken() string {
	manager := testTokenManager()
	value, _ := manager.Generate("user-1", "test@example.com")
	return value
}

func tTempDir() string {
	dir, _ := os.MkdirTemp("", "gamidoc-http-test-*")
	return dir
}

func testAuthHandler() http.Handler {
	repo := &fakeUserRepository{
		usersByEmail: map[string]user.User{},
		usersByID:    map[string]user.User{},
	}
	manager := testTokenManager()
	service := auth.NewService(repo, manager)
	handler := auth.NewHandler(service, manager)
	return handler.Routes()
}

func testProjectHandler() *project.Handler {
	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	service := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		testRecommendationService(),
	)

	return project.NewHandler(service)
}

func testSessionHandler() *session.Handler {
	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		testRecommendationService(),
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		testRecommendationService(),
	)

	return session.NewHandler(sessionService, projectService)
}

func testPDFHandler() *pdf.Handler {
	projectRepo := &fakeProjectRepository{
		items: []project.Project{},
		byID:  map[string]project.Project{},
	}

	sessionRepo := &fakeSessionRepository{
		byID: map[string]session.Session{},
	}

	recommendationService := testRecommendationService()

	projectService := project.NewService(
		projectRepo,
		sessionRepo,
		wizard.NewService(),
		recommendationService,
	)

	sessionService := session.NewService(
		sessionRepo,
		48*time.Hour,
		wizard.NewService(),
		recommendationService,
	)

	store := objectstore.NewLocalStore(tTempDir(), "/files/pdfs")
	builder := pdf.NewBuilder()
	generator := pdf.NewFPDFGenerator()

	service := pdf.NewService(
		builder,
		generator,
		store,
		mailer.NewNoopMailer(),
		"noreply@example.com",
		"GamiDoc",
		projectRepo,
		sessionRepo,
		projectService,
		sessionService,
	)

	return pdf.NewHandler(service)
}
