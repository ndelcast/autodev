package store

import (
	"time"
)

type Project struct {
	ID                   int64
	ProdPlannerProjectID int
	Name                 string
	Slug                 string
	GithubRepo           string
	DockerImage          string
	ContextContent       string // Markdown context injected into prompts
	SkillsContent        string // Markdown skills injected into prompts
	AutodevDeveloperID   int
	DoneColumnID         int
	Status               string // "idle", "running"
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Generation struct {
	ID                  int64
	ProjectID           int64
	ProdPlannerTicketID int
	TicketNumber        string
	TicketTitle         string
	TicketDescription   string
	Status              string // "queued", "running", "completed", "failed"
	BranchName          string
	PRUrl               string
	PromptSent          string
	ClaudeOutput        string
	ErrorMessage        string
	DurationSeconds     int
	Attempt             int
	StartedAt           *time.Time
	CompletedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

