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

	RecommendationRulesPath string
}

func Load() Config {
	expiresIn := getEnv("JWT_EXPIRES_IN", "24h")
	parsedExpiresIn, err := time.ParseDuration(expiresIn)
	if err != nil {
		parsedExpiresIn = 24 * time.Hour
	}

	sessionTTL := getEnv("SESSION_TTL", "48h")
	parsedSessionTTL, err := time.ParseDuration(sessionTTL)
	if err != nil {
		parsedSessionTTL = 48 * time.Hour
	}

	return Config{
		AppEnv:                         getEnv("APP_ENV", "development"),
		HTTPAddr:                       getEnv("HTTP_ADDR", ":8080"),
		PostgresHost:                   getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:                   getEnv("POSTGRES_PORT", "5432"),
		PostgresDB:                     getEnv("POSTGRES_DB", "gamidoc"),
		PostgresUser:                   getEnv("POSTGRES_USER", "gamidoc"),
		PostgresPassword:               getEnv("POSTGRES_PASSWORD", "gamidoc"),
		RedisHost:                      getEnv("REDIS_HOST", "localhost"),
		RedisPort:                      getEnv("REDIS_PORT", "6379"),
		JWTSecret:                      getEnv("JWT_SECRET", "dev-secret"),
		JWTExpiresIn:                   parsedExpiresIn,
		SessionTTL:                     parsedSessionTTL,
		ObjectStorageProvider:          getEnv("OBJECT_STORAGE_PROVIDER", "local"),
		ObjectStoragePublicBaseURL:     getEnv("OBJECT_STORAGE_PUBLIC_BASE_URL", getEnv("PDF_BASE_URL", "/files/pdfs")),
		ObjectStorageLocalRootDir:      getEnv("OBJECT_STORAGE_LOCAL_ROOT_DIR", getEnv("PDF_STORAGE_DIR", ".localdata/pdfs")),
		ObjectStorageS3Bucket:          getEnv("OBJECT_STORAGE_S3_BUCKET", ""),
		ObjectStorageS3Region:          getEnv("OBJECT_STORAGE_S3_REGION", "auto"),
		ObjectStorageS3Endpoint:        getEnv("OBJECT_STORAGE_S3_ENDPOINT", ""),
		ObjectStorageS3AccessKeyID:     getEnv("OBJECT_STORAGE_S3_ACCESS_KEY_ID", ""),
		ObjectStorageS3SecretAccessKey: getEnv("OBJECT_STORAGE_S3_SECRET_ACCESS_KEY", ""),
		ObjectStorageS3UsePathStyle:    getEnvBool("OBJECT_STORAGE_S3_USE_PATH_STYLE", false),
		RecommendationRulesPath:        getEnv("RECOMMENDATION_RULES_PATH", "rule/recommendations.json"),
	}
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
