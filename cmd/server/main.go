package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/api"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/config"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/server"
)

const shutdownTimeout = 30 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg.Address, api.NewRouter(cfg))
	errs := make(chan error, 2)
	go func() { errs <- serve(srv) }()
	slog.Info("server listening", "addr", cfg.Address)

	if cfg.PprofAddr != "" {
		debug := server.Pprof(cfg.PprofAddr)
		go func() { errs <- serve(debug) }()
		defer debug.Close()
		slog.Info("pprof listening", "addr", cfg.PprofAddr)
	}

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func serve(srv *http.Server) error {
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
