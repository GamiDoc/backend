package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv   string
	HTTPAddr string

	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPShutdownTimeout   time.Duration
	HTTPMaxBodyBytes      int64

	CORSAllowedOrigins []string

	MigrationsDir string

	PostgresHost     string
	PostgresPort     string
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	RedisHost string
	RedisPort string

	JWTSecret    string
	JWTExpiresIn time.Duration

	SessionTTL time.Duration

	ObjectStorageProvider          string
	ObjectStoragePublicBaseURL     string
	ObjectStorageLocalRootDir      string
	ObjectStorageS3Bucket          string
	ObjectStorageS3Region          string
	ObjectStorageS3Endpoint        string
	ObjectStorageS3AccessKeyID     string
	ObjectStorageS3SecretAccessKey string
	ObjectStorageS3UsePathStyle    bool

	MailerProvider  string
	MailerFromEmail string
	MailerFromName  string
	ResendAPIKey    string
	ResendBaseURL   string

	RecommendationRulesPath string
}

func Load() Config {
	expiresIn := parseDurationWithFallback(getEnv("JWT_EXPIRES_IN", "24h"), 24*time.Hour)
	sessionTTL := parseDurationWithFallback(getEnv("SESSION_TTL", "48h"), 48*time.Hour)

	return Config{
		AppEnv:                         getEnv("APP_ENV", "development"),
		HTTPAddr:                       getEnv("HTTP_ADDR", ":8080"),
		HTTPReadHeaderTimeout:          parseDurationWithFallback(getEnv("HTTP_READ_HEADER_TIMEOUT", "5s"), 5*time.Second),
		HTTPReadTimeout:                parseDurationWithFallback(getEnv("HTTP_READ_TIMEOUT", "15s"), 15*time.Second),
		HTTPWriteTimeout:               parseDurationWithFallback(getEnv("HTTP_WRITE_TIMEOUT", "30s"), 30*time.Second),
		HTTPIdleTimeout:                parseDurationWithFallback(getEnv("HTTP_IDLE_TIMEOUT", "60s"), 60*time.Second),
		HTTPShutdownTimeout:            parseDurationWithFallback(getEnv("HTTP_SHUTDOWN_TIMEOUT", "10s"), 10*time.Second),
		HTTPMaxBodyBytes:               getEnvInt64("HTTP_MAX_BODY_BYTES", 1048576),
		CORSAllowedOrigins:             getEnvCSV("CORS_ALLOWED_ORIGINS", []string{}),
		MigrationsDir:                  getEnv("MIGRATIONS_DIR", "migrations"),
		PostgresHost:                   getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:                   getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:                     getEnv("POSTGRES_DB", "gamidoc"),
		PostgresUser:                   getEnv("POSTGRES_USER", "gamidoc"),
		PostgresPassword:               getEnv("POSTGRES_PASSWORD", "gamidoc"),
		RedisHost:                      getEnv("REDIS_HOST", "localhost"),
		RedisPort:                      getEnv("REDIS_PORT", "6379"),
		JWTSecret:                      getEnv("JWT_SECRET", "dev-secret"),
		JWTExpiresIn:                   expiresIn,
		SessionTTL:                     sessionTTL,
		ObjectStorageProvider:          getEnv("OBJECT_STORAGE_PROVIDER", "local"),
		ObjectStoragePublicBaseURL:     getEnv("OBJECT_STORAGE_PUBLIC_BASE_URL", getEnv("PDF_BASE_URL", "/files/pdfs")),
		ObjectStorageLocalRootDir:      getEnv("OBJECT_STORAGE_LOCAL_ROOT_DIR", getEnv("PDF_STORAGE_DIR", ".localdata/pdfs")),
		ObjectStorageS3Bucket:          getEnv("OBJECT_STORAGE_S3_BUCKET", ""),
		ObjectStorageS3Region:          getEnv("OBJECT_STORAGE_S3_REGION", "auto"),
		ObjectStorageS3Endpoint:        getEnv("OBJECT_STORAGE_S3_ENDPOINT", ""),
		ObjectStorageS3AccessKeyID:     getEnv("OBJECT_STORAGE_S3_ACCESS_KEY_ID", ""),
		ObjectStorageS3SecretAccessKey: getEnv("OBJECT_STORAGE_S3_SECRET_ACCESS_KEY", ""),
		ObjectStorageS3UsePathStyle:    getEnvBool("OBJECT_STORAGE_S3_USE_PATH_STYLE", false),
		MailerProvider:                 getEnv("MAILER_PROVIDER", "noop"),
		MailerFromEmail:                getEnv("MAILER_FROM_EMAIL", ""),
		MailerFromName:                 getEnv("MAILER_FROM_NAME", "GamiDoc"),
		ResendAPIKey:                   getEnv("RESEND_API_KEY", ""),
		ResendBaseURL:                  getEnv("RESEND_BASE_URL", "https://api.resend.com"),
		RecommendationRulesPath:        getEnv("RECOMMENDATION_RULES_PATH", "rule/recommendations.json"),
	}
}

func (c Config) Validate() error {
	if err := c.ValidateCore(); err != nil {
		return err
	}
	if err := c.ValidateObjectStorage(); err != nil {
		return err
	}
	if err := c.ValidateMailer(); err != nil {
		return err
	}
	return nil
}

func (c Config) ValidateCore() error {
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return errors.New("http addr is required")
	}
	if c.HTTPReadHeaderTimeout <= 0 {
		return errors.New("http read header timeout must be positive")
	}
	if c.HTTPReadTimeout <= 0 {
		return errors.New("http read timeout must be positive")
	}
	if c.HTTPWriteTimeout <= 0 {
		return errors.New("http write timeout must be positive")
	}
	if c.HTTPIdleTimeout <= 0 {
		return errors.New("http idle timeout must be positive")
	}
	if c.HTTPShutdownTimeout <= 0 {
		return errors.New("http shutdown timeout must be positive")
	}
	if c.HTTPMaxBodyBytes <= 0 {
		return errors.New("http max body bytes must be positive")
	}
	if strings.TrimSpace(c.MigrationsDir) == "" {
		return errors.New("migrations dir is required")
	}
	if strings.TrimSpace(c.RecommendationRulesPath) == "" {
		return errors.New("recommendation rules path is required")
	}
	info, err := os.Stat(c.RecommendationRulesPath)
	if err != nil {
		return fmt.Errorf("recommendation rules path is not readable: %w", err)
	}
	if info.IsDir() {
		return errors.New("recommendation rules path must be a file")
	}
	if strings.EqualFold(strings.TrimSpace(c.AppEnv), "production") && c.JWTSecret == "dev-secret" {
		return errors.New("jwt secret must not use the development default in production")
	}
	return nil
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresUser,
		c.PostgresPassword,
	)
}

func (c Config) PostgresURL() string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
	)
}

func (c Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func (c Config) ObjectStorageProviderNormalized() string {
	value := strings.ToLower(strings.TrimSpace(c.ObjectStorageProvider))
	switch value {
	case "", "local":
		return "local"
	case "r2", "cloudflare-r2":
		return "cloudflare-r2"
	case "s3", "s3-compatible", "aws-s3":
		return "s3-compatible"
	default:
		return value
	}
}

func (c Config) MailerProviderNormalized() string {
	value := strings.ToLower(strings.TrimSpace(c.MailerProvider))
	switch value {
	case "", "noop":
		return "noop"
	case "resend":
		return "resend"
	default:
		return value
	}
}

func (c Config) ValidateObjectStorage() error {
	if strings.TrimSpace(c.ObjectStoragePublicBaseURL) == "" {
		return errors.New("object storage public base url is required")
	}

	switch c.ObjectStorageProviderNormalized() {
	case "local":
		if strings.TrimSpace(c.ObjectStorageLocalRootDir) == "" {
			return errors.New("object storage local root dir is required")
		}
		return nil
	case "cloudflare-r2", "s3-compatible":
		if strings.TrimSpace(c.ObjectStorageS3Bucket) == "" {
			return errors.New("object storage s3 bucket is required")
		}
		if strings.TrimSpace(c.ObjectStorageS3Region) == "" {
			return errors.New("object storage s3 region is required")
		}
		if strings.TrimSpace(c.ObjectStorageS3Endpoint) == "" {
			return errors.New("object storage s3 endpoint is required")
		}
		if strings.TrimSpace(c.ObjectStorageS3AccessKeyID) == "" {
			return errors.New("object storage s3 access key id is required")
		}
		if strings.TrimSpace(c.ObjectStorageS3SecretAccessKey) == "" {
			return errors.New("object storage s3 secret access key is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported object storage provider: %s", c.ObjectStorageProvider)
	}
}

func (c Config) ValidateMailer() error {
	switch c.MailerProviderNormalized() {
	case "noop":
		return nil
	case "resend":
		if strings.TrimSpace(c.MailerFromEmail) == "" {
			return errors.New("mailer from email is required")
		}
		if strings.TrimSpace(c.ResendAPIKey) == "" {
			return errors.New("resend api key is required")
		}
		if strings.TrimSpace(c.ResendBaseURL) == "" {
			return errors.New("resend base url is required")
		}
		return nil
	default:
		return fmt.Errorf("unsupported mailer provider: %s", c.MailerProvider)
	}
}

func (c Config) SafeSummary() map[string]any {
	return map[string]any{
		"app_env":                 c.AppEnv,
		"http_addr":               c.HTTPAddr,
		"http_max_body_bytes":     c.HTTPMaxBodyBytes,
		"cors_allowed_origins":    c.CORSAllowedOrigins,
		"migrations_dir":          c.MigrationsDir,
		"object_storage_provider": c.ObjectStorageProviderNormalized(),
		"mailer_provider":         c.MailerProviderNormalized(),
		"recommendation_rules":    c.RecommendationRulesPath,
	}
}

func parseDurationWithFallback(value string, fallback time.Duration) time.Duration {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return fallback
	}

	return result
}
