package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

type Config struct {
	HTTPAddr          string
	DatabasePath      string
	SessionTTL        time.Duration
	WorkerPoll        time.Duration
	WorkerMaxAttempts int
	BootstrapEmail    string
	BootstrapPassword string
	BootstrapRole     domain.Role
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		DatabasePath:      env("DATABASE_PATH", ".data/lushan-study.db"),
		BootstrapEmail:    env("BOOTSTRAP_EMAIL", "operator@example.test"),
		BootstrapPassword: env("BOOTSTRAP_PASSWORD", "change-me-now"),
		BootstrapRole:     domain.Role(env("BOOTSTRAP_ROLE", "operator")),
	}
	var err error
	if cfg.SessionTTL, err = time.ParseDuration(env("SESSION_TTL", "12h")); err != nil {
		return Config{}, fmt.Errorf("SESSION_TTL: %w", err)
	}
	if cfg.WorkerPoll, err = time.ParseDuration(env("WORKER_POLL_INTERVAL", "1s")); err != nil {
		return Config{}, fmt.Errorf("WORKER_POLL_INTERVAL: %w", err)
	}
	if cfg.WorkerMaxAttempts, err = strconv.Atoi(env("WORKER_MAX_ATTEMPTS", "5")); err != nil {
		return Config{}, fmt.Errorf("WORKER_MAX_ATTEMPTS: %w", err)
	}
	if cfg.DatabasePath == "" || cfg.SessionTTL <= 0 || cfg.WorkerPoll <= 0 || cfg.WorkerMaxAttempts < 1 {
		return Config{}, fmt.Errorf("configuration values must be positive and database path is required")
	}
	if !cfg.BootstrapRole.Valid() {
		return Config{}, fmt.Errorf("BOOTSTRAP_ROLE is invalid")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
