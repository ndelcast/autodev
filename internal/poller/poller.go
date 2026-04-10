package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

// TicketClient abstracts the ProdPlanner API for testability.
type TicketClient interface {
	ListTickets(ctx context.Context, opts prodplanner.ListTicketsOptions) ([]prodplanner.Ticket, error)
}

// Enqueuer abstracts the scheduler for testability.
type Enqueuer interface {
	Enqueue(gen *store.Generation)
}

// Poller polls ProdPlanner for new tickets and enqueues generations.
type Poller struct {
	store    *store.Store
	client   TicketClient
	enqueuer Enqueuer
	interval time.Duration
}

// New creates a new Poller.
func New(s *store.Store, client TicketClient, enqueuer Enqueuer, interval time.Duration) *Poller {
	return &Poller{
		store:    s,
		client:   client,
		enqueuer: enqueuer,
		interval: interval,
	}
}

// Start runs the polling loop until the context is cancelled.
func (p *Poller) Start(ctx context.Context) {
	slog.Info("poller started", "interval", p.interval)

	// Run once immediately
	if n, err := p.PollOnce(ctx); err != nil {
		slog.Error("poll cycle failed", "error", err)
	} else {
		slog.Info("poll cycle completed", "new_tickets", n)
	}

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller stopped")
			return
		case <-ticker.C:
			slog.Debug("poll cycle starting")
			if n, err := p.PollOnce(ctx); err != nil {
				slog.Error("poll cycle failed", "error", err)
			} else {
				slog.Info("poll cycle completed", "new_tickets", n)
			}
		}
	}
}

// PollOnce runs a single poll cycle across all projects.
// Returns the number of new generations enqueued.
func (p *Poller) PollOnce(ctx context.Context) (int, error) {
	projects, err := p.store.ListProjects()
	if err != nil {
		return 0, err
	}

	total := 0
	for _, project := range projects {
		if project.AutodevDeveloperID == 0 {
			continue
		}

		n, err := p.pollProject(ctx, &project)
		if err != nil {
			slog.Error("polling project", "project", project.Slug, "error", err)
			continue
		}
		total += n
	}
	return total, nil
}

// pollProject polls tickets for a single project.
func (p *Poller) pollProject(ctx context.Context, project *store.Project) (int, error) {
	tickets, err := p.client.ListTickets(ctx, prodplanner.ListTicketsOptions{
		AssignedTo: project.AutodevDeveloperID,
		ProjectID:  project.ProdPlannerProjectID,
	})
	if err != nil {
		return 0, err
	}

	slog.Info("poller: tickets found",
		"project", project.Slug,
		"count", len(tickets),
		"developer_id", project.AutodevDeveloperID,
	)

	count := 0
	for _, ticket := range tickets {
		// Skip if already processed
		existing, err := p.store.GetGenerationByTicketID(ticket.ID)
		if err != nil {
			slog.Error("checking existing generation", "ticket_id", ticket.ID, "error", err)
			continue
		}
		if existing != nil {
			slog.Debug("poller: ticket already has generation, skipping",
				"ticket", ticket.FormattedNumber,
				"generation_id", existing.ID,
				"status", existing.Status,
			)
			continue
		}

		gen := &store.Generation{
			ProjectID:           project.ID,
			ProdPlannerTicketID: ticket.ID,
			TicketNumber:        ticket.FormattedNumber,
			TicketTitle:         ticket.Title,
			TicketDescription:   ticket.Description,
			Status:              "queued",
			Attempt:             1,
		}
		if err := p.store.CreateGeneration(gen); err != nil {
			slog.Error("creating generation", "ticket_id", ticket.ID, "error", err)
			continue
		}

		slog.Info("new ticket found, enqueuing",
			"ticket", ticket.FormattedNumber,
			"project", project.Slug,
			"generation_id", gen.ID,
		)
		p.enqueuer.Enqueue(gen)
		count++
	}
	return count, nil
}
