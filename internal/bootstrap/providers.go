package bootstrap

import (
	"context"
	"fmt"

	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/mailer"
	"github.com/yifen9/gamidoc-backend/internal/storage/objectstore"
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
