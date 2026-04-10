package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProdPlanner       ProdPlannerConfig `yaml:"prodplanner"`
	Claude            ClaudeConfig      `yaml:"claude"`
	Polling           PollingConfig     `yaml:"polling"`
	Docker            DockerConfig      `yaml:"docker"`
	WebPort           int               `yaml:"web_port"`
	DBPath            string            `yaml:"db_path"`
	SkillsDir         string            `yaml:"skills_dir"`
	ContextsDir       string            `yaml:"contexts_dir"`
	WorkspaceBasePath string            `yaml:"workspace_base_path"`
	Projects          []ProjectConfig   `yaml:"projects"`
}

type ProdPlannerConfig struct {
	BaseURL      string `yaml:"base_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

type ClaudeConfig struct {
	Model    string   `yaml:"model"`
	MaxTurns int      `yaml:"max_turns"`
	Timeout  Duration `yaml:"timeout"`
}

type PollingConfig struct {
	Interval Duration `yaml:"interval"`
}

type DockerConfig struct {
	NetworkMode   string `yaml:"network_mode"`
	MaxConcurrent int    `yaml:"max_concurrent"`
}

type ProjectConfig struct {
	Name                 string   `yaml:"name"`
	Slug                 string   `yaml:"slug"`
	ProdPlannerProjectID int      `yaml:"prodplanner_project_id"`
	GithubRepo           string   `yaml:"github_repo"`
	DockerImage          string   `yaml:"docker_image"`
	ContextFile          string   `yaml:"context_file"`
	Skills               []string `yaml:"skills"`
	AutodevDeveloperID   int      `yaml:"autodev_developer_id"`
	DoneColumnID         int      `yaml:"done_column_id"`
}

// Duration wraps time.Duration to support YAML unmarshalling from strings like "60s", "10m".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("parsing duration %q: %w", s, err)
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Load reads a YAML config file, expands environment variables, and applies defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	// Expand ${VAR} references in the YAML before parsing
	expanded := os.ExpandEnv(string(data))

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	applyDefaults(cfg)

	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.WebPort == 0 {
		cfg.WebPort = 8080
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "./autodev.db"
	}
	if cfg.SkillsDir == "" {
		cfg.SkillsDir = "./skills"
	}
	if cfg.ContextsDir == "" {
		cfg.ContextsDir = "./contexts"
	}
	if cfg.WorkspaceBasePath == "" {
		cfg.WorkspaceBasePath = "./workspaces"
	}
	if cfg.Claude.Model == "" {
		cfg.Claude.Model = "sonnet"
	}
	if cfg.Claude.MaxTurns == 0 {
		cfg.Claude.MaxTurns = 30
	}
	if cfg.Claude.Timeout.Duration == 0 {
		cfg.Claude.Timeout.Duration = 10 * time.Minute
	}
	if cfg.Polling.Interval.Duration == 0 {
		cfg.Polling.Interval.Duration = 60 * time.Second
	}
	if cfg.Docker.NetworkMode == "" {
		cfg.Docker.NetworkMode = "host"
	}
	if cfg.Docker.MaxConcurrent == 0 {
		cfg.Docker.MaxConcurrent = 3
	}
}
