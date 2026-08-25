package config

import (
	"testing"
	"time"

	"github.com/11DingKing/lushan-study-ops-go/internal/domain"
)

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"HTTP_ADDR",
		"DATABASE_PATH",
		"SESSION_TTL",
		"WORKER_POLL_INTERVAL",
		"WORKER_MAX_ATTEMPTS",
		"BOOTSTRAP_EMAIL",
		"BOOTSTRAP_PASSWORD",
		"BOOTSTRAP_ROLE",
	} {
		t.Setenv(name, "")
	}
}

func TestLoadUsesDocumentedDefaultsWhenVariablesAreAbsent(t *testing.T) {
	for _, name := range []string{
		"HTTP_ADDR", "DATABASE_PATH", "SESSION_TTL", "WORKER_POLL_INTERVAL",
		"WORKER_MAX_ATTEMPTS", "BOOTSTRAP_EMAIL", "BOOTSTRAP_PASSWORD", "BOOTSTRAP_ROLE",
	} {
		t.Setenv(name, "")
	}
	// Setenv cannot represent absence, so establish the normal values explicitly.
	t.Setenv("HTTP_ADDR", ":8080")
	t.Setenv("DATABASE_PATH", ".data/lushan-study.db")
	t.Setenv("SESSION_TTL", "12h")
	t.Setenv("WORKER_POLL_INTERVAL", "1s")
	t.Setenv("WORKER_MAX_ATTEMPTS", "5")
	t.Setenv("BOOTSTRAP_EMAIL", "operator@example.test")
	t.Setenv("BOOTSTRAP_PASSWORD", "change-me-now")
	t.Setenv("BOOTSTRAP_ROLE", "operator")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.DatabasePath != ".data/lushan-study.db" {
		t.Fatalf("address/path = %q/%q", cfg.HTTPAddr, cfg.DatabasePath)
	}
	if cfg.SessionTTL != 12*time.Hour || cfg.WorkerPoll != time.Second || cfg.WorkerMaxAttempts != 5 {
		t.Fatalf("durations/attempts = %v/%v/%d", cfg.SessionTTL, cfg.WorkerPoll, cfg.WorkerMaxAttempts)
	}
	if cfg.BootstrapRole != domain.RoleOperator {
		t.Fatalf("bootstrap role = %q", cfg.BootstrapRole)
	}
}

func TestLoadParsesExplicitOperationalConfiguration(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("DATABASE_PATH", "/tmp/lushan.db")
	t.Setenv("SESSION_TTL", "45m")
	t.Setenv("WORKER_POLL_INTERVAL", "250ms")
	t.Setenv("WORKER_MAX_ATTEMPTS", "9")
	t.Setenv("BOOTSTRAP_EMAIL", "safety@example.test")
	t.Setenv("BOOTSTRAP_PASSWORD", "a-long-password")
	t.Setenv("BOOTSTRAP_ROLE", "safety")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.DatabasePath != "/tmp/lushan.db" {
		t.Fatalf("address/path = %q/%q", cfg.HTTPAddr, cfg.DatabasePath)
	}
	if cfg.SessionTTL != 45*time.Minute || cfg.WorkerPoll != 250*time.Millisecond {
		t.Fatalf("durations = %v/%v", cfg.SessionTTL, cfg.WorkerPoll)
	}
	if cfg.WorkerMaxAttempts != 9 || cfg.BootstrapEmail != "safety@example.test" || cfg.BootstrapRole != domain.RoleSafety {
		t.Fatalf("config = %+v", cfg)
	}
}

func TestLoadRejectsMalformedDurationsAndAttempts(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{name: "session duration", env: "SESSION_TTL", value: "tomorrow"},
		{name: "poll duration", env: "WORKER_POLL_INTERVAL", value: "soon"},
		{name: "attempt count", env: "WORKER_MAX_ATTEMPTS", value: "many"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.env, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%q", test.env, test.value)
			}
		})
	}
}

func TestLoadRejectsNonPositiveWorkerAndSessionValues(t *testing.T) {
	tests := []struct {
		env   string
		value string
	}{
		{env: "SESSION_TTL", value: "0s"},
		{env: "WORKER_POLL_INTERVAL", value: "-1s"},
		{env: "WORKER_MAX_ATTEMPTS", value: "0"},
		{env: "DATABASE_PATH", value: ""},
	}
	for _, test := range tests {
		t.Run(test.env, func(t *testing.T) {
			t.Setenv(test.env, test.value)
			if _, err := Load(); err == nil {
				t.Fatalf("Load() accepted %s=%q", test.env, test.value)
			}
		})
	}
}

func TestLoadRejectsUnknownBootstrapRole(t *testing.T) {
	t.Setenv("BOOTSTRAP_ROLE", "superuser")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted unknown bootstrap role")
	}
}
