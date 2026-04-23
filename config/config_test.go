package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsEmptyHTTPAddr(t *testing.T) {
	rulesPath := filepath.Join("..", "rule", "recommendations.json")

	cfg := Config{
		HTTPAddr:                   "",
		HTTPReadHeaderTimeout:      1,
		HTTPReadTimeout:            1,
		HTTPWriteTimeout:           1,
		HTTPIdleTimeout:            1,
		HTTPShutdownTimeout:        1,
		HTTPMaxBodyBytes:           1,
		MigrationsDir:              "migrations",
		RecommendationRulesPath:    rulesPath,
		ObjectStorageProvider:      "local",
		ObjectStoragePublicBaseURL: "/files/pdfs",
		ObjectStorageLocalRootDir:  t.TempDir(),
		MailerProvider:             "noop",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsProductionDefaultJWTSecret(t *testing.T) {
	rulesPath := filepath.Join("..", "rule", "recommendations.json")

	cfg := Config{
		AppEnv:                     "production",
		HTTPAddr:                   ":8080",
		HTTPReadHeaderTimeout:      1,
		HTTPReadTimeout:            1,
		HTTPWriteTimeout:           1,
		HTTPIdleTimeout:            1,
		HTTPShutdownTimeout:        1,
		HTTPMaxBodyBytes:           1,
		MigrationsDir:              "migrations",
		JWTSecret:                  "dev-secret",
		RecommendationRulesPath:    rulesPath,
		ObjectStorageProvider:      "local",
		ObjectStoragePublicBaseURL: "/files/pdfs",
		ObjectStorageLocalRootDir:  t.TempDir(),
		MailerProvider:             "noop",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateRejectsMissingRulesFile(t *testing.T) {
	cfg := Config{
		AppEnv:                     "development",
		HTTPAddr:                   ":8080",
		HTTPReadHeaderTimeout:      1,
		HTTPReadTimeout:            1,
		HTTPWriteTimeout:           1,
		HTTPIdleTimeout:            1,
		HTTPShutdownTimeout:        1,
		HTTPMaxBodyBytes:           1,
		MigrationsDir:              "migrations",
		JWTSecret:                  "not-default",
		RecommendationRulesPath:    filepath.Join(t.TempDir(), "missing.json"),
		ObjectStorageProvider:      "local",
		ObjectStoragePublicBaseURL: "/files/pdfs",
		ObjectStorageLocalRootDir:  t.TempDir(),
		MailerProvider:             "noop",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidatePassesWithValidConfig(t *testing.T) {
	rulesPath := filepath.Join("..", "rule", "recommendations.json")

	cfg := Config{
		AppEnv:                     "development",
		HTTPAddr:                   ":8080",
		HTTPReadHeaderTimeout:      1,
		HTTPReadTimeout:            1,
		HTTPWriteTimeout:           1,
		HTTPIdleTimeout:            1,
		HTTPShutdownTimeout:        1,
		HTTPMaxBodyBytes:           1024,
		MigrationsDir:              "migrations",
		JWTSecret:                  "secret",
		RecommendationRulesPath:    rulesPath,
		ObjectStorageProvider:      "local",
		ObjectStoragePublicBaseURL: "/files/pdfs",
		ObjectStorageLocalRootDir:  t.TempDir(),
		MailerProvider:             "noop",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestLoadParsesCSVOrigins(t *testing.T) {
	rulesPath := filepath.Join("..", "rule", "recommendations.json")

	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("JWT_SECRET", "secret")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "5s")
	t.Setenv("HTTP_READ_TIMEOUT", "5s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "5s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "5s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("HTTP_MAX_BODY_BYTES", "2048")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://example.com")
	t.Setenv("MIGRATIONS_DIR", "migrations")
	t.Setenv("RECOMMENDATION_RULES_PATH", rulesPath)
	t.Setenv("OBJECT_STORAGE_PROVIDER", "local")
	t.Setenv("OBJECT_STORAGE_PUBLIC_BASE_URL", "/files/pdfs")
	t.Setenv("OBJECT_STORAGE_LOCAL_ROOT_DIR", t.TempDir())
	t.Setenv("MAILER_PROVIDER", "noop")

	cfg := Load()

	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected 2 cors origins, got %d", len(cfg.CORSAllowedOrigins))
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
