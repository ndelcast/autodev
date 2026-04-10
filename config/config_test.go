package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	yaml := `
prodplanner:
  base_url: "https://example.com/api"
  client_id: "test-id"
  client_secret: "test-secret"

claude:
  model: "opus"
  max_turns: 50
  timeout: "15m"

polling:
  interval: "30s"

docker:
  network_mode: "bridge"
  max_concurrent: 5

web_port: 9090
db_path: "/tmp/test.db"
workspace_base_path: "/tmp/workspaces"
`
	path := writeTempFile(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// ProdPlanner
	if cfg.ProdPlanner.BaseURL != "https://example.com/api" {
		t.Errorf("BaseURL = %q, want %q", cfg.ProdPlanner.BaseURL, "https://example.com/api")
	}
	if cfg.ProdPlanner.ClientID != "test-id" {
		t.Errorf("ClientID = %q, want %q", cfg.ProdPlanner.ClientID, "test-id")
	}

	// Claude
	if cfg.Claude.Model != "opus" {
		t.Errorf("Model = %q, want %q", cfg.Claude.Model, "opus")
	}
	if cfg.Claude.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want %d", cfg.Claude.MaxTurns, 50)
	}
	if cfg.Claude.Timeout.Duration != 15*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Claude.Timeout.Duration, 15*time.Minute)
	}

	// Polling
	if cfg.Polling.Interval.Duration != 30*time.Second {
		t.Errorf("Interval = %v, want %v", cfg.Polling.Interval.Duration, 30*time.Second)
	}

	// Docker
	if cfg.Docker.NetworkMode != "bridge" {
		t.Errorf("NetworkMode = %q, want %q", cfg.Docker.NetworkMode, "bridge")
	}
	if cfg.Docker.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent = %d, want %d", cfg.Docker.MaxConcurrent, 5)
	}

	// Top-level
	if cfg.WebPort != 9090 {
		t.Errorf("WebPort = %d, want %d", cfg.WebPort, 9090)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
}

func TestLoadEnvExpansion(t *testing.T) {
	t.Setenv("TEST_CLIENT_ID", "env-id")
	t.Setenv("TEST_CLIENT_SECRET", "env-secret")

	yaml := `
prodplanner:
  base_url: "https://example.com/api"
  client_id: "${TEST_CLIENT_ID}"
  client_secret: "${TEST_CLIENT_SECRET}"
`
	path := writeTempFile(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.ProdPlanner.ClientID != "env-id" {
		t.Errorf("ClientID = %q, want %q", cfg.ProdPlanner.ClientID, "env-id")
	}
	if cfg.ProdPlanner.ClientSecret != "env-secret" {
		t.Errorf("ClientSecret = %q, want %q", cfg.ProdPlanner.ClientSecret, "env-secret")
	}
}

func TestLoadDefaults(t *testing.T) {
	yaml := `
prodplanner:
  base_url: "https://example.com/api"
`
	path := writeTempFile(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.WebPort != 8080 {
		t.Errorf("WebPort = %d, want default 8080", cfg.WebPort)
	}
	if cfg.DBPath != "./autodev.db" {
		t.Errorf("DBPath = %q, want default ./autodev.db", cfg.DBPath)
	}
	if cfg.Claude.Model != "sonnet" {
		t.Errorf("Model = %q, want default sonnet", cfg.Claude.Model)
	}
	if cfg.Claude.MaxTurns != 30 {
		t.Errorf("MaxTurns = %d, want default 30", cfg.Claude.MaxTurns)
	}
	if cfg.Claude.Timeout.Duration != 10*time.Minute {
		t.Errorf("Timeout = %v, want default 10m", cfg.Claude.Timeout.Duration)
	}
	if cfg.Polling.Interval.Duration != 60*time.Second {
		t.Errorf("Interval = %v, want default 60s", cfg.Polling.Interval.Duration)
	}
	if cfg.Docker.MaxConcurrent != 3 {
		t.Errorf("MaxConcurrent = %d, want default 3", cfg.Docker.MaxConcurrent)
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
