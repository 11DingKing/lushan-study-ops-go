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

	"github.com/11DingKing/lushan-study-ops-go/internal/clock"
	"github.com/11DingKing/lushan-study-ops-go/internal/config"
	"github.com/11DingKing/lushan-study-ops-go/internal/httpapi"
	"github.com/11DingKing/lushan-study-ops-go/internal/service"
	"github.com/11DingKing/lushan-study-ops-go/internal/storage"
	"github.com/11DingKing/lushan-study-ops-go/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	store, err := storage.Open(rootCtx, cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	svc := service.New(store, clock.Real{}, cfg.SessionTTL, cfg.WorkerMaxAttempts)
	if err := svc.EnsureBootstrapUser(rootCtx, cfg.BootstrapEmail, cfg.BootstrapPassword, cfg.BootstrapRole); err != nil {
		return err
	}
	background := worker.New(store, clock.Real{}, cfg.WorkerPoll, logger)
	loggingHandler := worker.LoggingHandler{Logger: logger}
	background.Register("confirmation.notice", loggingHandler)
	background.Register("settlement.process", loggingHandler)
	workerDone := make(chan error, 1)
	go func() { workerDone <- background.Run(rootCtx) }()

	httpServer := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpapi.New(svc, logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	serverDone := make(chan error, 1)
	go func() {
		logger.Info("http server started", "address", cfg.HTTPAddr)
		serverDone <- httpServer.ListenAndServe()
	}()

	select {
	case <-rootCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		if err := <-workerDone; err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case err := <-serverDone:
		stop()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-workerDone:
		stop()
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}
