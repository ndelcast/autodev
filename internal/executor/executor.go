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
	project, err := e.store.GetProject(gen.ProjectID)
	if err != nil {
		return e.failGeneration(gen, startedAt, "loading project: %v", err)
	}

	// 3. Fetch ticket from ProdPlanner
	ticket, err := e.pp.GetTicket(ctx, gen.ProdPlannerTicketID)
	if err != nil {
		return e.failGeneration(gen, startedAt, "fetching ticket: %v", err)
	}

	// 4. Build prompt
	promptText, err := e.prompt.Build(project, ticket)
	if err != nil {
		return e.failGeneration(gen, startedAt, "building prompt: %v", err)
	}

	// 5. Create temp directory for prompt and output
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("autodev-gen-%d-", gen.ID))
	if err != nil {
		return e.failGeneration(gen, startedAt, "creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	outputDir := filepath.Join(tmpDir, "output")
	os.MkdirAll(outputDir, 0755)

	// Write prompt
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

	// 6. Ensure workspace
	workspacePath, err := e.ensureWorkspace(project)
	if err != nil {
		return e.failGeneration(gen, startedAt, "preparing workspace: %v", err)
	}

	// 7. Build branch name
	branchName := buildBranchName(ticket.FormattedNumber, ticket.Title)
	gen.BranchName = branchName
	gen.PromptSent = promptText

	// 8. Run Docker container
	log.Info("launching container", "image", project.DockerImage, "branch", branchName)

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
	claudeResult, _ := os.ReadFile(filepath.Join(outputDir, "claude_result.json"))
	prURL, _ := os.ReadFile(filepath.Join(outputDir, "pr_url.txt"))
	gen.ClaudeOutput = string(claudeResult)

	if result.ExitCode != 0 {
		gen.ClaudeOutput = string(claudeResult)
		if result.Stderr != "" {
			gen.ErrorMessage = result.Stderr
		} else {
			errorTxt, _ := os.ReadFile(filepath.Join(outputDir, "error.txt"))
			gen.ErrorMessage = string(errorTxt)
		}
		return e.failGeneration(gen, startedAt, "container exited with code %d", result.ExitCode)
	}

	// 10. Success — update ProdPlanner
	gen.PRUrl = strings.TrimSpace(string(prURL))
	log.Info("generation completed", "pr_url", gen.PRUrl)

	if err := e.pp.MoveTicket(ctx, ticket.ID, project.DoneColumnID); err != nil {
		log.Error("failed to move ticket in ProdPlanner", "error", err)
		// Don't fail the generation for this
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
	workspacePath := filepath.Join(e.cfg.WorkspaceBasePath, project.Slug)

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		e.logger.Info("cloning repository", "repo", project.GithubRepo, "path", workspacePath)
		repoURL := fmt.Sprintf("https://github.com/%s.git", project.GithubRepo)
		cmd := exec.Command("git", "clone", repoURL, workspacePath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("cloning %s: %w", project.GithubRepo, err)
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
