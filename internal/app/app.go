package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/auth"
	"github.com/yifen9/gamidoc-backend/internal/bootstrap"
	apphttp "github.com/yifen9/gamidoc-backend/internal/http"
	"github.com/yifen9/gamidoc-backend/internal/pdf"
	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
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

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	pg, err := postgres.New(cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	redisClient := rediscache.New(cfg.RedisAddr())

	startupCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPReadTimeout)
	defer cancel()

	if err := pg.Ready(startupCtx); err != nil {
		_ = pg.Close()
		return nil, fmt.Errorf("postgres startup check failed: %w", err)
	}

	if err := redisClient.Ready(startupCtx); err != nil {
		_ = pg.Close()
		_ = redisClient.Close()
		return nil, fmt.Errorf("redis startup check failed: %w", err)
	}

	tokenManager := token.NewManager(cfg.JWTSecret, cfg.JWTExpiresIn)

	wizardService := wizard.NewService()

	rules, err := recommendation.LoadRulesFromFile(cfg.RecommendationRulesPath)
	if err != nil {
		_ = pg.Close()
		_ = redisClient.Close()
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

	store, err := bootstrap.NewObjectStore(cfg)
	if err != nil {
		_ = pg.Close()
		_ = redisClient.Close()
		return nil, err
	}

	m, err := bootstrap.NewMailer(cfg)
	if err != nil {
		_ = pg.Close()
		_ = redisClient.Close()
		return nil, err
	}

	pdfBuilder := pdf.NewBuilder()
	pdfGenerator := pdf.NewFPDFGenerator()
	pdfService := pdf.NewService(
		pdfBuilder,
		pdfGenerator,
		store,
		m,
		cfg.MailerFromEmail,
		cfg.MailerFromName,
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
		Logger:             application.logger,
		Postgres:           application.pg,
		Redis:              application.redis,
		TokenManager:       tokenManager,
		AuthHandler:        authHandler.Routes(),
		ProjectHandler:     projectHandler,
		SessionHandler:     sessionHandler,
		PDFHandler:         pdfHandler,
		PDFBaseURL:         cfg.ObjectStoragePublicBaseURL,
		MaxBodyBytes:       cfg.HTTPMaxBodyBytes,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	})

	summary := cfg.SafeSummary()
	application.logger.Info(
		"app_config",
		"app_env", summary["app_env"],
		"http_addr", summary["http_addr"],
		"http_max_body_bytes", summary["http_max_body_bytes"],
		"cors_allowed_origins", summary["cors_allowed_origins"],
		"migrations_dir", summary["migrations_dir"],
		"object_storage_provider", summary["object_storage_provider"],
		"mailer_provider", summary["mailer_provider"],
		"recommendation_rules", summary["recommendation_rules"],
	)

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
