package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/app"
)

func main() {
	cfg := config.Load()

	application, err := app.New(cfg)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := application.Close(); err != nil {
			application.Logger().Error("app_close_failed", "error", err.Error())
		}
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
			panic(err)
		}
	case <-shutdownCtx.Done():
		application.Logger().Info("server_shutdown_started")

		ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTPShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			application.Logger().Error("server_shutdown_failed", "error", err.Error())
			panic(err)
		}

		if err := <-serverErrCh; err != nil {
			application.Logger().Error("server_stopped", "error", err.Error())
			panic(err)
		}

		application.Logger().Info("server_shutdown_completed")
	}
}
