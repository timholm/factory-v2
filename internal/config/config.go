package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all factory configuration.
type Config struct {
	PostgresURL   string
	ArchiveURL    string
	GitHubToken   string
	GitHubUser    string
	GitDir        string
	ClaudeBinary  string
	CycleInterval time.Duration
	MaxBuilds     int
	Workers       int
}

// Load reads config from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		PostgresURL:   envOr("POSTGRES_URL", "postgres://factory:factory@localhost:5432/factory?sslmode=disable"),
		ArchiveURL:    envOr("ARCHIVE_URL", "http://localhost:8080"),
		GitHubToken:   os.Getenv("GITHUB_TOKEN"),
		GitHubUser:    envOr("GITHUB_USER", "timholm"),
		GitDir:        envOr("GIT_DIR", os.Getenv("HOME")+"/factory-git"),
		ClaudeBinary:  envOr("CLAUDE_BINARY", "claude"),
		CycleInterval: envDuration("CYCLE_INTERVAL", 1*time.Hour),
		MaxBuilds:     envInt("MAX_BUILDS", 5),
		Workers:       envInt("WORKERS", 2),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
