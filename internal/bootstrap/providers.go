package bootstrap

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gamidoc/backend/config"
	"github.com/gamidoc/backend/internal/ai"
	"github.com/gamidoc/backend/internal/mailer"
	"github.com/gamidoc/backend/internal/storage/objectstore"
)

func NewObjectStore(cfg config.Config) (objectstore.ObjectStore, error) {
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

func NewAssistant(cfg config.Config) (ai.Assistant, error) {
	switch cfg.AIProviderNormalized() {
	case "noop":
		return ai.NewNoopAssistant(), nil
	case "openai-compatible":
		return ai.NewOpenAIAssistant(cfg.AIBaseURL, cfg.AIAPIKey, cfg.AIModel, &http.Client{Timeout: cfg.AITimeout}), nil
	default:
		return nil, fmt.Errorf("unsupported ai provider: %s", cfg.AIProvider)
	}
}

func NewMailer(cfg config.Config) (mailer.Mailer, error) {
	switch cfg.MailerProviderNormalized() {
	case "noop":
		return mailer.NewNoopMailer(), nil
	case "resend":
		return mailer.NewResendMailer(
			cfg.ResendAPIKey,
			cfg.ResendBaseURL,
			nil,
		), nil
	default:
		return nil, fmt.Errorf("unsupported mailer provider: %s", cfg.MailerProvider)
	}
}
