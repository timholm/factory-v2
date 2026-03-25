package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Unset env vars to test defaults
	os.Unsetenv("POSTGRES_URL")
	os.Unsetenv("ARCHIVE_URL")
	os.Unsetenv("GITHUB_USER")
	os.Unsetenv("CLAUDE_BINARY")
	os.Unsetenv("CYCLE_INTERVAL")
	os.Unsetenv("MAX_BUILDS")
	os.Unsetenv("WORKERS")

	cfg := Load()

	if cfg.PostgresURL != "postgres://factory:factory@localhost:5432/factory?sslmode=disable" {
		t.Errorf("unexpected PostgresURL: %s", cfg.PostgresURL)
	}
	if cfg.ArchiveURL != "http://localhost:8080" {
		t.Errorf("unexpected ArchiveURL: %s", cfg.ArchiveURL)
	}
	if cfg.GitHubUser != "timholm" {
		t.Errorf("unexpected GitHubUser: %s", cfg.GitHubUser)
	}
	if cfg.ClaudeBinary != "claude" {
		t.Errorf("unexpected ClaudeBinary: %s", cfg.ClaudeBinary)
	}
	if cfg.CycleInterval != 1*time.Hour {
		t.Errorf("unexpected CycleInterval: %s", cfg.CycleInterval)
	}
	if cfg.MaxBuilds != 5 {
		t.Errorf("unexpected MaxBuilds: %d", cfg.MaxBuilds)
	}
	if cfg.Workers != 2 {
		t.Errorf("unexpected Workers: %d", cfg.Workers)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("POSTGRES_URL", "postgres://test:test@db:5432/test")
	t.Setenv("ARCHIVE_URL", "http://archive:9090")
	t.Setenv("GITHUB_USER", "testuser")
	t.Setenv("CLAUDE_BINARY", "/usr/bin/claude")
	t.Setenv("CYCLE_INTERVAL", "30m")
	t.Setenv("MAX_BUILDS", "10")
	t.Setenv("WORKERS", "4")

	cfg := Load()

	if cfg.PostgresURL != "postgres://test:test@db:5432/test" {
		t.Errorf("unexpected PostgresURL: %s", cfg.PostgresURL)
	}
	if cfg.ArchiveURL != "http://archive:9090" {
		t.Errorf("unexpected ArchiveURL: %s", cfg.ArchiveURL)
	}
	if cfg.GitHubUser != "testuser" {
		t.Errorf("unexpected GitHubUser: %s", cfg.GitHubUser)
	}
	if cfg.ClaudeBinary != "/usr/bin/claude" {
		t.Errorf("unexpected ClaudeBinary: %s", cfg.ClaudeBinary)
	}
	if cfg.CycleInterval != 30*time.Minute {
		t.Errorf("unexpected CycleInterval: %s", cfg.CycleInterval)
	}
	if cfg.MaxBuilds != 10 {
		t.Errorf("unexpected MaxBuilds: %d", cfg.MaxBuilds)
	}
	if cfg.Workers != 4 {
		t.Errorf("unexpected Workers: %d", cfg.Workers)
	}
}

func TestEnvOr(t *testing.T) {
	os.Unsetenv("TEST_UNSET_VAR")
	if v := envOr("TEST_UNSET_VAR", "default"); v != "default" {
		t.Errorf("expected default, got %s", v)
	}

	t.Setenv("TEST_SET_VAR", "custom")
	if v := envOr("TEST_SET_VAR", "default"); v != "custom" {
		t.Errorf("expected custom, got %s", v)
	}
}

func TestEnvInt(t *testing.T) {
	os.Unsetenv("TEST_INT_UNSET")
	if v := envInt("TEST_INT_UNSET", 42); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}

	t.Setenv("TEST_INT_SET", "99")
	if v := envInt("TEST_INT_SET", 42); v != 99 {
		t.Errorf("expected 99, got %d", v)
	}

	t.Setenv("TEST_INT_BAD", "notanumber")
	if v := envInt("TEST_INT_BAD", 42); v != 42 {
		t.Errorf("expected 42 for invalid int, got %d", v)
	}
}

func TestEnvDuration(t *testing.T) {
	os.Unsetenv("TEST_DUR_UNSET")
	if v := envDuration("TEST_DUR_UNSET", 5*time.Second); v != 5*time.Second {
		t.Errorf("expected 5s, got %s", v)
	}

	t.Setenv("TEST_DUR_SET", "10m")
	if v := envDuration("TEST_DUR_SET", 5*time.Second); v != 10*time.Minute {
		t.Errorf("expected 10m, got %s", v)
	}
}
