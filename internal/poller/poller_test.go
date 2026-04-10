package poller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

type mockTicketClient struct {
	tickets map[int][]prodplanner.Ticket // keyed by project ID
}

func (m *mockTicketClient) ListTickets(_ context.Context, opts prodplanner.ListTicketsOptions) ([]prodplanner.Ticket, error) {
	return m.tickets[opts.ProjectID], nil
}

type mockEnqueuer struct {
	mu   sync.Mutex
	gens []*store.Generation
}

func (m *mockEnqueuer) Enqueue(gen *store.Generation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gens = append(m.gens, gen)
}

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPollOnce_NewTickets(t *testing.T) {
	s := setupTestStore(t)

	// Seed a project
	p := &store.Project{
		ProdPlannerProjectID: 10,
		Name:                 "Test",
		Slug:                 "test",
		GithubRepo:           "org/test",
		DockerImage:          "autodev-base",
		AutodevDeveloperID:   42,
		DoneColumnID:         5,
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatal(err)
	}

	client := &mockTicketClient{
		tickets: map[int][]prodplanner.Ticket{
			10: {
				{ID: 100, FormattedNumber: "TST-1", Title: "Feature A", Description: "Do A"},
				{ID: 101, FormattedNumber: "TST-2", Title: "Feature B", Description: "Do B"},
			},
		},
	}
	enq := &mockEnqueuer{}
	poll := New(s, client, enq, time.Minute)

	n, err := poll.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 new tickets, got %d", n)
	}
	if len(enq.gens) != 2 {
		t.Fatalf("expected 2 enqueued, got %d", len(enq.gens))
	}
	if enq.gens[0].TicketNumber != "TST-1" {
		t.Errorf("expected TST-1, got %s", enq.gens[0].TicketNumber)
	}
}

func TestPollOnce_SkipExisting(t *testing.T) {
	s := setupTestStore(t)

	p := &store.Project{
		ProdPlannerProjectID: 10,
		Name:                 "Test",
		Slug:                 "test",
		GithubRepo:           "org/test",
		DockerImage:          "autodev-base",
		AutodevDeveloperID:   42,
		DoneColumnID:         5,
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatal(err)
	}

	// Pre-create a generation for ticket 100
	gen := &store.Generation{
		ProjectID:           p.ID,
		ProdPlannerTicketID: 100,
		TicketNumber:        "TST-1",
		TicketTitle:         "Feature A",
		Status:              "completed",
		Attempt:             1,
	}
	if err := s.CreateGeneration(gen); err != nil {
		t.Fatal(err)
	}

	client := &mockTicketClient{
		tickets: map[int][]prodplanner.Ticket{
			10: {
				{ID: 100, FormattedNumber: "TST-1", Title: "Feature A"},
				{ID: 101, FormattedNumber: "TST-2", Title: "Feature B"},
			},
		},
	}
	enq := &mockEnqueuer{}
	poll := New(s, client, enq, time.Minute)

	n, err := poll.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 new ticket (skip existing), got %d", n)
	}
	if enq.gens[0].TicketNumber != "TST-2" {
		t.Errorf("expected TST-2, got %s", enq.gens[0].TicketNumber)
	}
}

func TestPollOnce_SkipProjectWithoutDeveloper(t *testing.T) {
	s := setupTestStore(t)

	// Project without AutodevDeveloperID
	p := &store.Project{
		ProdPlannerProjectID: 10,
		Name:                 "NoBot",
		Slug:                 "nobot",
		GithubRepo:           "org/nobot",
		DockerImage:          "autodev-base",
		AutodevDeveloperID:   0, // not configured
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatal(err)
	}

	client := &mockTicketClient{
		tickets: map[int][]prodplanner.Ticket{
			10: {{ID: 100, FormattedNumber: "NB-1", Title: "Feature"}},
		},
	}
	enq := &mockEnqueuer{}
	poll := New(s, client, enq, time.Minute)

	n, err := poll.PollOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0, project has no developer configured, got %d", n)
	}
}
