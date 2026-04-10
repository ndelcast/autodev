package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/outlined/autodev/config"
	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

type Executor struct {
	store   *store.Store
	pp      *prodplanner.Client
	docker  *DockerRunner
	prompt  *PromptBuilder
	cfg     *config.Config
	logger  *slog.Logger
}

func New(store *store.Store, pp *prodplanner.Client, docker *DockerRunner, prompt *PromptBuilder, cfg *config.Config) *Executor {
	return &Executor{
		store:  store,
		pp:     pp,
		docker: docker,
		prompt: prompt,
		cfg:    cfg,
		logger: slog.Default().With("component", "executor"),
	}
}

// Process executes a full generation cycle for a ticket.
func (e *Executor) Process(ctx context.Context, gen *store.Generation) error {
	startedAt := time.Now()
	log := e.logger.With("generation_id", gen.ID, "ticket", gen.TicketNumber)

	// 1. Mark running
	gen.Status = "running"
	gen.StartedAt = &startedAt
	if err := e.store.UpdateGeneration(gen); err != nil {
		return fmt.Errorf("marking generation running: %w", err)
	}
	log.Info("generation started")

	// 2. Load project
	log.Info("loading project", "project_id", gen.ProjectID)
	project, err := e.store.GetProject(gen.ProjectID)
	if err != nil {
		return e.failGeneration(gen, startedAt, "loading project: %v", err)
	}
	log.Info("project loaded", "project", project.Name, "image", project.DockerImage)

	// 3. Fetch ticket from ProdPlanner
	log.Info("fetching ticket from ProdPlanner", "ticket_id", gen.ProdPlannerTicketID)
	ticket, err := e.pp.GetTicket(ctx, gen.ProdPlannerTicketID)
	if err != nil {
		return e.failGeneration(gen, startedAt, "fetching ticket: %v", err)
	}
	log.Info("ticket fetched", "title", ticket.Title, "type", ticket.Type, "size", ticket.Size)

	// 4. Ensure workspace
	log.Info("ensuring workspace exists", "repo", project.GithubRepo)
	workspacePath, err := e.ensureWorkspace(project)
	if err != nil {
		return e.failGeneration(gen, startedAt, "preparing workspace: %v", err)
	}
	log.Info("workspace ready", "path", workspacePath)

	// 5. Write CLAUDE.md into workspace (context + skills — loaded automatically by Claude Code)
	claudeMD := e.prompt.BuildClaudeMD(project)
	claudeMDPath := filepath.Join(workspacePath, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(claudeMD), 0644); err != nil {
		return e.failGeneration(gen, startedAt, "writing CLAUDE.md: %v", err)
	}
	log.Info("CLAUDE.md written", "path", claudeMDPath, "length", len(claudeMD))

	// 6. Build lean prompt (ticket only — context is in CLAUDE.md)
	promptText := e.prompt.BuildPrompt(ticket)
	log.Info("prompt built", "length", len(promptText))

	// 7. Create temp directory for prompt and output
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("autodev-gen-%d-", gen.ID))
	if err != nil {
		return e.failGeneration(gen, startedAt, "creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Write prompt file
	promptFile := filepath.Join(tmpDir, "prompt.md")
	if err := os.WriteFile(promptFile, []byte(promptText), 0644); err != nil {
		return e.failGeneration(gen, startedAt, "writing prompt: %v", err)
	}

	// Write PR body
	prBody := e.prompt.BuildPRBody(ticket)
	prBodyFile := filepath.Join(outputDir, "pr_body.md")
	if err := os.WriteFile(prBodyFile, []byte(prBody), 0644); err != nil {
		return e.failGeneration(gen, startedAt, "writing PR body: %v", err)
	}

	// 8. Build branch name
	branchName := buildBranchName(ticket.FormattedNumber, ticket.Title)
	gen.BranchName = branchName
	gen.PromptSent = promptText
	log.Info("branch name generated", "branch", branchName)

	// 8. Run Docker container
	log.Info("launching container",
		"image", project.DockerImage,
		"branch", branchName,
		"timeout", e.cfg.Claude.Timeout.Duration,
		"model", e.cfg.Claude.Model,
		"max_turns", e.cfg.Claude.MaxTurns,
	)

	result, err := e.docker.RunContainer(ctx, ContainerOpts{
		Image:         project.DockerImage,
		ContainerName: "autodev-" + strings.ToLower(ticket.FormattedNumber),
		Cmd: []string{
			"/usr/local/bin/autodev-exec",
			branchName,
			ticket.FormattedNumber,
			ticket.Title,
			"/prompt/prompt.md",
		},
		Env: []string{
			"ANTHROPIC_API_KEY=" + os.Getenv("ANTHROPIC_API_KEY"),
			"GITHUB_TOKEN=" + os.Getenv("GITHUB_TOKEN"),
			"CLAUDE_MODEL=" + e.cfg.Claude.Model,
			"CLAUDE_MAX_TURNS=" + fmt.Sprintf("%d", e.cfg.Claude.MaxTurns),
		},
		Binds: []string{
			workspacePath + ":/workspace",
			promptFile + ":/prompt/prompt.md:ro",
			outputDir + ":/output",
		},
		NetworkMode: e.cfg.Docker.NetworkMode,
		Timeout:     e.cfg.Claude.Timeout.Duration,
	})
	if err != nil {
		return e.failGeneration(gen, startedAt, "running container: %v", err)
	}

	// 9. Collect results
	log.Info("container finished", "exit_code", result.ExitCode)
	claudeResult, _ := os.ReadFile(filepath.Join(outputDir, "claude_result.json"))
	prURL, _ := os.ReadFile(filepath.Join(outputDir, "pr_url.txt"))
	gen.ClaudeOutput = string(claudeResult)

	if result.ExitCode != 0 {
		gen.ClaudeOutput = string(claudeResult)

		// Collect all error sources
		var errParts []string
		if result.Stderr != "" {
			errParts = append(errParts, "Container stderr:\n"+result.Stderr)
		}
		if result.Stdout != "" {
			errParts = append(errParts, "Container stdout:\n"+result.Stdout)
		}
		errorTxt, _ := os.ReadFile(filepath.Join(outputDir, "error.txt"))
		if len(errorTxt) > 0 {
			errParts = append(errParts, "error.txt:\n"+string(errorTxt))
		}
		stderrLog, _ := os.ReadFile(filepath.Join(outputDir, "claude_stderr.log"))
		if len(stderrLog) > 0 {
			errParts = append(errParts, "claude_stderr.log:\n"+string(stderrLog))
		}

		if len(errParts) > 0 {
			gen.ErrorMessage = strings.Join(errParts, "\n\n")
		} else {
			gen.ErrorMessage = fmt.Sprintf("container exited with code %d (no error output captured)", result.ExitCode)
		}

		return e.failGenerationKeepError(gen, startedAt, result.ExitCode)
	}

	// 10. Success — update ProdPlanner
	gen.PRUrl = strings.TrimSpace(string(prURL))
	log.Info("generation completed successfully",
		"pr_url", gen.PRUrl,
		"duration", int(time.Since(startedAt).Seconds()),
	)

	log.Info("moving ticket in ProdPlanner", "column_id", project.DoneColumnID)
	if err := e.pp.MoveTicket(ctx, ticket.ID, project.DoneColumnID); err != nil {
		log.Error("failed to move ticket in ProdPlanner", "error", err)
	}

	// 11. Save success
	gen.Status = "completed"
	gen.DurationSeconds = int(time.Since(startedAt).Seconds())
	now := time.Now()
	gen.CompletedAt = &now
	if err := e.store.UpdateGeneration(gen); err != nil {
		return fmt.Errorf("saving completed generation: %w", err)
	}

	return nil
}

// failGenerationKeepError saves the generation as failed without overwriting ErrorMessage.
func (e *Executor) failGenerationKeepError(gen *store.Generation, startedAt time.Time, exitCode int64) error {
	e.logger.Error("generation failed",
		"generation_id", gen.ID,
		"exit_code", exitCode,
		"error_preview", truncateStr(gen.ErrorMessage, 200),
	)

	gen.Status = "failed"
	gen.DurationSeconds = int(time.Since(startedAt).Seconds())
	now := time.Now()
	gen.CompletedAt = &now
	e.store.UpdateGeneration(gen)

	return fmt.Errorf("generation %d failed: container exited with code %d", gen.ID, exitCode)
}

func (e *Executor) failGeneration(gen *store.Generation, startedAt time.Time, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	e.logger.Error("generation failed", "generation_id", gen.ID, "error", msg)

	gen.Status = "failed"
	gen.ErrorMessage = msg
	gen.DurationSeconds = int(time.Since(startedAt).Seconds())
	now := time.Now()
	gen.CompletedAt = &now
	e.store.UpdateGeneration(gen)

	return fmt.Errorf("generation %d failed: %s", gen.ID, msg)
}

func (e *Executor) ensureWorkspace(project *store.Project) (string, error) {
	basePath, _ := filepath.Abs(e.cfg.WorkspaceBasePath)
	workspacePath := filepath.Join(basePath, fmt.Sprintf("proj_%d", project.ID))

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		e.logger.Info("cloning repository", "repo", project.GithubRepo, "path", workspacePath)

		// Use GITHUB_TOKEN for private repos
		token := os.Getenv("GITHUB_TOKEN")
		var repoURL string
		if token != "" {
			repoURL = fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", token, project.GithubRepo)
		} else {
			repoURL = fmt.Sprintf("https://github.com/%s.git", project.GithubRepo)
		}

		var stderr strings.Builder
		cmd := exec.Command("git", "clone", repoURL, workspacePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("cloning %s: %s", project.GithubRepo, stderr.String())
		}
	}

	return workspacePath, nil
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func buildBranchName(ticketNumber, title string) string {
	slug := strings.ToLower(title)
	slug = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			return r
		}
		return -1
	}, slug)
	slug = nonAlphaNum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if len(slug) > 40 {
		slug = slug[:40]
		slug = strings.TrimRight(slug, "-")
	}

	return fmt.Sprintf("feat/%s/%s", strings.ToLower(ticketNumber), slug)
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
