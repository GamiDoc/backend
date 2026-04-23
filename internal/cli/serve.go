package cli

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/gamidoc/backend/config"
	"github.com/gamidoc/backend/internal/app"
	"github.com/spf13/cobra"
)

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			application, err := app.New(cfg)
			if err != nil {
				return err
			}
			defer func() {
				_ = application.Close()
			}()

			server := &http.Server{
				Addr:              cfg.HTTPAddr,
				Handler:           application.Router(),
				ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
				ReadTimeout:       cfg.HTTPReadTimeout,
				WriteTimeout:      cfg.HTTPWriteTimeout,
				IdleTimeout:       cfg.HTTPIdleTimeout,
			}

			shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			serverErrCh := make(chan error, 1)

			application.Logger().Info("server_starting", "http_addr", cfg.HTTPAddr, "app_env", cfg.AppEnv)

			go func() {
				err := server.ListenAndServe()
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					serverErrCh <- err
					return
				}
				serverErrCh <- nil
			}()

			select {
			case err := <-serverErrCh:
				if err != nil {
					application.Logger().Error("server_stopped", "error", err.Error())
					return err
				}
				return nil
			case <-shutdownCtx.Done():
				application.Logger().Info("server_shutdown_started")

				ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
				defer cancel()

				if err := server.Shutdown(ctx); err != nil {
					application.Logger().Error("server_shutdown_failed", "error", err.Error())
					return err
				}

				if err := <-serverErrCh; err != nil {
					application.Logger().Error("server_stopped", "error", err.Error())
					return err
				}

				application.Logger().Info("server_shutdown_completed")
				return nil
			}
		},
	}
}
