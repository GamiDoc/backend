package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/auth"
	apphttp "github.com/yifen9/gamidoc-backend/internal/http"
	"github.com/yifen9/gamidoc-backend/internal/pdf"
	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/storage/objectstore"
	"github.com/yifen9/gamidoc-backend/internal/storage/postgres"
	rediscache "github.com/yifen9/gamidoc-backend/internal/storage/redis"
	"github.com/yifen9/gamidoc-backend/internal/token"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

type App struct {
	config config.Config
	logger *slog.Logger
	router http.Handler
	pg     *postgres.DB
	redis  *rediscache.Client
}

func New(cfg config.Config) (*App, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := cfg.ValidateObjectStorage(); err != nil {
		return nil, err
	}

	pg, err := postgres.New(cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	redisClient := rediscache.New(cfg.RedisAddr())
	tokenManager := token.NewManager(cfg.JWTSecret, cfg.JWTExpiresIn)

	wizardService := wizard.NewService()

	rules, err := recommendation.LoadRulesFromFile(cfg.RecommendationRulesPath)
	if err != nil {
		return nil, err
	}
	recommendationEngine := recommendation.NewEngine(rules)
	recommendationService := recommendation.NewService(recommendationEngine)

	userRepository := postgres.NewUserRepository(pg)
	authService := auth.NewService(userRepository, tokenManager)
	authHandler := auth.NewHandler(authService, tokenManager)

	projectRepository := postgres.NewProjectRepository(pg)
	sessionRepository := rediscache.NewSessionRepository(redisClient, cfg.SessionTTL)

	projectService := project.NewService(projectRepository, sessionRepository, wizardService, recommendationService)
	projectHandler := project.NewHandler(projectService)

	sessionService := session.NewService(sessionRepository, cfg.SessionTTL, wizardService, recommendationService)
	sessionHandler := session.NewHandler(sessionService, projectService)

	store, err := newObjectStore(cfg)
	if err != nil {
		return nil, err
	}

	pdfBuilder := pdf.NewBuilder()
	pdfGenerator := pdf.NewFPDFGenerator()
	pdfService := pdf.NewService(
		pdfBuilder,
		pdfGenerator,
		store,
		projectRepository,
		sessionRepository,
		projectService,
		sessionService,
	)
	pdfHandler := pdf.NewHandler(pdfService)

	application := &App{
		config: cfg,
		logger: logger,
		pg:     pg,
		redis:  redisClient,
	}

	application.router = apphttp.NewRouter(apphttp.Dependencies{
		Logger:         application.logger,
		Postgres:       application.pg,
		Redis:          application.redis,
		TokenManager:   tokenManager,
		AuthHandler:    authHandler.Routes(),
		ProjectHandler: projectHandler,
		SessionHandler: sessionHandler,
		PDFHandler:     pdfHandler,
		PDFBaseURL:     cfg.ObjectStoragePublicBaseURL,
	})

	return application, nil
}

func (a *App) Router() http.Handler {
	return a.router
}

func (a *App) Logger() *slog.Logger {
	return a.logger
}

func (a *App) Close() error {
	if err := a.pg.Close(); err != nil {
		return err
	}

	if err := a.redis.Close(); err != nil {
		return err
	}

	return nil
}

func newObjectStore(cfg config.Config) (objectstore.ObjectStore, error) {
	switch cfg.ObjectStorageProviderNormalized() {
	case "local":
		return objectstore.NewLocalStore(
			cfg.ObjectStorageLocalRootDir,
			cfg.ObjectStoragePublicBaseURL,
		), nil
	case "cloudflare-r2", "s3-compatible":
		return objectstore.NewS3Store(context.Background(), objectstore.S3StoreConfig{
			Bucket:          cfg.ObjectStorageS3Bucket,
			Region:          cfg.ObjectStorageS3Region,
			Endpoint:        cfg.ObjectStorageS3Endpoint,
			AccessKeyID:     cfg.ObjectStorageS3AccessKeyID,
			SecretAccessKey: cfg.ObjectStorageS3SecretAccessKey,
			UsePathStyle:    cfg.ObjectStorageS3UsePathStyle,
			BaseURL:         cfg.ObjectStoragePublicBaseURL,
		})
	default:
		return nil, fmt.Errorf("unsupported object storage provider: %s", cfg.ObjectStorageProvider)
	}
}
