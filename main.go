package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/outlined/autodev/config"
	"github.com/outlined/autodev/internal/executor"
	"github.com/outlined/autodev/internal/poller"
	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/scheduler"
	"github.com/outlined/autodev/internal/store"
	"github.com/outlined/autodev/internal/web"
)

func main() {
	// Setup structured logging
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Load config
	cfgPath := "config.yaml"
	if v := os.Getenv("AUTODEV_CONFIG"); v != "" {
		cfgPath = v
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("loading config", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	switch os.Args[1] {
	case "serve":
		cmdServe(ctx, cfg)
	case "run":
		cmdRun(ctx, cfg)
	case "retry":
		cmdRetry(ctx, cfg)
	case "projects":
		cmdProjects(cfg)
	case "generations":
		cmdGenerations(cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: autodev <command> [flags]

Commands:
  serve         Start the daemon (poller + scheduler + dashboard)
  run           Process a single ticket
  retry         Retry a failed generation
  projects      List configured projects
  generations   List generations for a project
`)
}

func initStore(cfg *config.Config) *store.Store {
	s, err := store.New(cfg.DBPath)
	if err != nil {
		slog.Error("opening database", "error", err)
		os.Exit(1)
	}
	if err := s.SeedProjects(cfg.Projects); err != nil {
		slog.Error("seeding projects", "error", err)
		os.Exit(1)
	}
	return s
}

func initExecutor(s *store.Store, pp *prodplanner.Client, cfg *config.Config) *executor.Executor {
	docker, err := executor.NewDockerRunner()
	if err != nil {
		slog.Error("creating docker client", "error", err)
		os.Exit(1)
	}
	prompt := executor.NewPromptBuilder(cfg.SkillsDir, cfg.ContextsDir)
	return executor.New(s, pp, docker, prompt, cfg)
}

func cmdServe(ctx context.Context, cfg *config.Config) {
	s := initStore(cfg)
	defer s.Close()

	pp := prodplanner.NewClient(cfg.ProdPlanner)
	exec := initExecutor(s, pp, cfg)

	// Scheduler
	sched := scheduler.New(exec, s, cfg.Docker.MaxConcurrent)

	// Re-enqueue any generations left in "queued" state from a previous run
	queued, err := s.ListQueuedGenerations()
	if err != nil {
		slog.Error("loading queued generations", "error", err)
	} else {
		for i := range queued {
			slog.Info("re-enqueuing generation", "id", queued[i].ID, "ticket", queued[i].TicketNumber)
			sched.Enqueue(&queued[i])
		}
	}

	// Poller
	poll := poller.New(s, pp, sched, cfg.Polling.Interval.Duration)
	go poll.Start(ctx)

	// Web dashboard
	dashboard, err := web.New(s, poll, cfg.WebPort)
	if err != nil {
		slog.Error("creating dashboard", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := dashboard.Start(); err != nil {
			slog.Error("dashboard server", "error", err)
		}
	}()

	slog.Info("autodev serve started",
		"polling_interval", cfg.Polling.Interval.Duration,
		"max_concurrent", cfg.Docker.MaxConcurrent,
		"dashboard", fmt.Sprintf("http://localhost:%d", cfg.WebPort),
	)

	// Block until shutdown signal
	<-ctx.Done()
	slog.Info("shutting down...")
	sched.Shutdown()
}

func cmdRun(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	projectSlug := fs.String("project", "", "Project slug")
	ticketID := fs.Int("ticket", 0, "ProdPlanner ticket ID")
	fs.Parse(os.Args[2:])

	if *projectSlug == "" || *ticketID == 0 {
		fmt.Fprintf(os.Stderr, "Usage: autodev run --project=<slug> --ticket=<id>\n")
		os.Exit(1)
	}

	s := initStore(cfg)
	defer s.Close()

	pp := prodplanner.NewClient(cfg.ProdPlanner)
	exec := initExecutor(s, pp, cfg)

	// Load project
	project, err := s.GetProjectBySlug(*projectSlug)
	if err != nil {
		slog.Error("project not found", "slug", *projectSlug, "error", err)
		os.Exit(1)
	}

	// Fetch ticket
	ticket, err := pp.GetTicket(ctx, *ticketID)
	if err != nil {
		slog.Error("fetching ticket", "id", *ticketID, "error", err)
		os.Exit(1)
	}

	// Create generation
	gen := &store.Generation{
		ProjectID:           project.ID,
		ProdPlannerTicketID: ticket.ID,
		TicketNumber:        ticket.FormattedNumber,
		TicketTitle:         ticket.Title,
		TicketDescription:   ticket.Description,
		Status:              "queued",
		Attempt:             1,
	}
	if err := s.CreateGeneration(gen); err != nil {
		slog.Error("creating generation", "error", err)
		os.Exit(1)
	}

	slog.Info("starting generation", "id", gen.ID, "ticket", gen.TicketNumber, "project", project.Name)

	if err := exec.Process(ctx, gen); err != nil {
		slog.Error("generation failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Generation completed successfully!\n")
	fmt.Printf("  Ticket:   %s — %s\n", gen.TicketNumber, gen.TicketTitle)
	fmt.Printf("  Branch:   %s\n", gen.BranchName)
	fmt.Printf("  PR:       %s\n", gen.PRUrl)
	fmt.Printf("  Duration: %ds\n", gen.DurationSeconds)
}

func cmdRetry(ctx context.Context, cfg *config.Config) {
	fs := flag.NewFlagSet("retry", flag.ExitOnError)
	genID := fs.Int64("generation", 0, "Generation ID to retry")
	fs.Parse(os.Args[2:])

	if *genID == 0 {
		fmt.Fprintf(os.Stderr, "Usage: autodev retry --generation=<id>\n")
		os.Exit(1)
	}

	s := initStore(cfg)
	defer s.Close()

	pp := prodplanner.NewClient(cfg.ProdPlanner)
	exec := initExecutor(s, pp, cfg)

	gen, err := s.GetGeneration(*genID)
	if err != nil {
		slog.Error("generation not found", "id", *genID, "error", err)
		os.Exit(1)
	}

	gen.Status = "queued"
	gen.Attempt++
	gen.ErrorMessage = ""
	gen.StartedAt = nil
	gen.CompletedAt = nil
	if err := s.UpdateGeneration(gen); err != nil {
		slog.Error("resetting generation", "error", err)
		os.Exit(1)
	}

	slog.Info("retrying generation", "id", gen.ID, "attempt", gen.Attempt, "ticket", gen.TicketNumber)

	if err := exec.Process(ctx, gen); err != nil {
		slog.Error("retry failed", "error", err)
		os.Exit(1)
	}

	fmt.Printf("Retry completed successfully!\n")
	fmt.Printf("  PR: %s\n", gen.PRUrl)
}

func cmdProjects(cfg *config.Config) {
	s := initStore(cfg)
	defer s.Close()

	projects, err := s.ListProjects()
	if err != nil {
		slog.Error("listing projects", "error", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSLUG\tGITHUB\tIMAGE\tSTATUS")
	for _, p := range projects {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n", p.ID, p.Name, p.Slug, p.GithubRepo, p.DockerImage, p.Status)
	}
	w.Flush()
}

func cmdGenerations(cfg *config.Config) {
	fs := flag.NewFlagSet("generations", flag.ExitOnError)
	projectSlug := fs.String("project", "", "Project slug")
	fs.Parse(os.Args[2:])

	if *projectSlug == "" {
		fmt.Fprintf(os.Stderr, "Usage: autodev generations --project=<slug>\n")
		os.Exit(1)
	}

	s := initStore(cfg)
	defer s.Close()

	project, err := s.GetProjectBySlug(*projectSlug)
	if err != nil {
		slog.Error("project not found", "slug", *projectSlug, "error", err)
		os.Exit(1)
	}

	gens, err := s.ListGenerationsByProject(project.ID)
	if err != nil {
		slog.Error("listing generations", "error", err)
		os.Exit(1)
	}

	if len(gens) == 0 {
		fmt.Println("No generations found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTICKET\tSTATUS\tBRANCH\tPR\tDURATION\tATTEMPT")
	for _, g := range gens {
		pr := g.PRUrl
		if len(pr) > 50 {
			pr = pr[:50] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%ds\t%d\n",
			g.ID, g.TicketNumber, g.Status, g.BranchName, pr, g.DurationSeconds, g.Attempt)
	}
	w.Flush()
}
