package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/bootstrap"
	"github.com/yifen9/gamidoc-backend/internal/storage/postgres"
	rediscache "github.com/yifen9/gamidoc-backend/internal/storage/redis"
)

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run environment diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			failed := false
			check := func(name string, fn func() error) {
				if err := fn(); err != nil {
					failed = true
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[fail] %s: %v\n", name, err)
					return
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[ok] %s\n", name)
			}

			check("config.core", cfg.ValidateCore)
			check("config.object_storage", cfg.ValidateObjectStorage)
			check("config.mailer", cfg.ValidateMailer)
			check("migrations.dir", func() error {
				info, err := os.Stat(cfg.MigrationsDir)
				if err != nil {
					return err
				}
				if !info.IsDir() {
					return errors.New("migrations dir must be a directory")
				}
				return nil
			})
			check("postgres", func() error {
				db, err := postgres.New(cfg.PostgresDSN())
				if err != nil {
					return err
				}
				defer func() {
					_ = db.Close()
				}()

				ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPReadTimeout)
				defer cancel()

				return db.Ready(ctx)
			})
			check("redis", func() error {
				client := rediscache.New(cfg.RedisAddr())
				defer func() {
					_ = client.Close()
				}()

				ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPReadTimeout)
				defer cancel()

				return client.Ready(ctx)
			})
			check("object_storage.init", func() error {
				_, err := bootstrap.NewObjectStore(cfg)
				return err
			})
			if cfg.ObjectStorageProviderNormalized() == "local" {
				check("object_storage.local_writable", func() error {
					if err := os.MkdirAll(cfg.ObjectStorageLocalRootDir, 0o755); err != nil {
						return err
					}
					path := filepath.Join(cfg.ObjectStorageLocalRootDir, fmt.Sprintf(".doctor-%d.tmp", time.Now().UnixNano()))
					if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
						return err
					}
					return os.Remove(path)
				})
			}
			check("mailer.init", func() error {
				_, err := bootstrap.NewMailer(cfg)
				return err
			})

			if failed {
				return errors.New("doctor failed")
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "doctor completed successfully")
			return nil
		},
	}
}
