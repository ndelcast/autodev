package store

import (
	"testing"
	"time"
)

func createTestProject(t *testing.T, s *Store) *Project {
	t.Helper()
	p := &Project{
		ProdPlannerProjectID: 1,
		Name:                 "Test",
		Slug:                 "test",
		GithubRepo:           "org/repo",
		DockerImage:          "autodev-base:latest",
		AutodevDeveloperID:   1,
		DoneColumnID:         10,
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	return p
}

func TestCreateAndGetGeneration(t *testing.T) {
	s := newTestStore(t)
	p := createTestProject(t, s)

	g := &Generation{
		ProjectID:           p.ID,
		ProdPlannerTicketID: 397,
		TicketNumber:        "DISP-385",
		TicketTitle:         "Télécharger les icônes",
		TicketDescription:   "En tant que prestataire...",
		Status:              "queued",
		Attempt:             1,
	}

	if err := s.CreateGeneration(g); err != nil {
		t.Fatalf("CreateGeneration: %v", err)
	}
	if g.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.GetGeneration(g.ID)
	if err != nil {
		t.Fatalf("GetGeneration: %v", err)
	}

	if got.TicketNumber != "DISP-385" {
		t.Errorf("TicketNumber = %q, want DISP-385", got.TicketNumber)
	}
	if got.Status != "queued" {
		t.Errorf("Status = %q, want queued", got.Status)
	}
	if got.Attempt != 1 {
		t.Errorf("Attempt = %d, want 1", got.Attempt)
	}
}

func TestUpdateGeneration(t *testing.T) {
	s := newTestStore(t)
	p := createTestProject(t, s)

	g := &Generation{
		ProjectID:           p.ID,
		ProdPlannerTicketID: 100,
		TicketNumber:        "TEST-1",
		TicketTitle:         "Test ticket",
		Status:              "queued",
		Attempt:             1,
	}
	if err := s.CreateGeneration(g); err != nil {
		t.Fatalf("CreateGeneration: %v", err)
	}

	now := time.Now()
	g.Status = "completed"
	g.BranchName = "feat/TEST-1/test-ticket"
	g.PRUrl = "https://github.com/org/repo/pull/42"
	g.PromptSent = "test prompt"
	g.ClaudeOutput = "test output"
	g.DurationSeconds = 120
	g.StartedAt = &now
	g.CompletedAt = &now

	if err := s.UpdateGeneration(g); err != nil {
		t.Fatalf("UpdateGeneration: %v", err)
	}

	got, err := s.GetGeneration(g.ID)
	if err != nil {
		t.Fatalf("GetGeneration: %v", err)
	}

	if got.Status != "completed" {
		t.Errorf("Status = %q, want completed", got.Status)
	}
	if got.BranchName != "feat/TEST-1/test-ticket" {
		t.Errorf("BranchName = %q", got.BranchName)
	}
	if got.PRUrl != "https://github.com/org/repo/pull/42" {
		t.Errorf("PRUrl = %q", got.PRUrl)
	}
	if got.DurationSeconds != 120 {
		t.Errorf("DurationSeconds = %d, want 120", got.DurationSeconds)
	}
}

func TestListGenerationsByProject(t *testing.T) {
	s := newTestStore(t)
	p := createTestProject(t, s)

	for i, num := range []string{"TEST-1", "TEST-2", "TEST-3"} {
		g := &Generation{
			ProjectID:           p.ID,
			ProdPlannerTicketID: 100 + i,
			TicketNumber:        num,
			TicketTitle:         "Ticket " + num,
			Status:              "queued",
			Attempt:             1,
		}
		if err := s.CreateGeneration(g); err != nil {
			t.Fatalf("CreateGeneration(%s): %v", num, err)
		}
	}

	gens, err := s.ListGenerationsByProject(p.ID)
	if err != nil {
		t.Fatalf("ListGenerationsByProject: %v", err)
	}
	if len(gens) != 3 {
		t.Fatalf("len = %d, want 3", len(gens))
	}
	// Should be in DESC order (most recent first)
	if gens[0].TicketNumber != "TEST-3" {
		t.Errorf("first = %q, want TEST-3", gens[0].TicketNumber)
	}
}

func TestGetGenerationByTicketID(t *testing.T) {
	s := newTestStore(t)
	p := createTestProject(t, s)

	g := &Generation{
		ProjectID:           p.ID,
		ProdPlannerTicketID: 555,
		TicketNumber:        "DISP-555",
		TicketTitle:         "Some ticket",
		Status:              "queued",
		Attempt:             1,
	}
	if err := s.CreateGeneration(g); err != nil {
		t.Fatalf("CreateGeneration: %v", err)
	}

	got, err := s.GetGenerationByTicketID(555)
	if err != nil {
		t.Fatalf("GetGenerationByTicketID: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil generation")
	}
	if got.TicketNumber != "DISP-555" {
		t.Errorf("TicketNumber = %q, want DISP-555", got.TicketNumber)
	}

	// Non-existent ticket
	got, err = s.GetGenerationByTicketID(999)
	if err != nil {
		t.Fatalf("GetGenerationByTicketID(999): %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for non-existent ticket, got %v", got)
	}
}

func TestListQueuedGenerations(t *testing.T) {
	s := newTestStore(t)
	p := createTestProject(t, s)

	statuses := []string{"queued", "running", "queued", "completed"}
	for i, status := range statuses {
		g := &Generation{
			ProjectID:           p.ID,
			ProdPlannerTicketID: 200 + i,
			TicketNumber:        "T-" + status,
			TicketTitle:         "Ticket " + status,
			Status:              status,
			Attempt:             1,
		}
		if err := s.CreateGeneration(g); err != nil {
			t.Fatalf("CreateGeneration: %v", err)
		}
	}

	queued, err := s.ListQueuedGenerations()
	if err != nil {
		t.Fatalf("ListQueuedGenerations: %v", err)
	}
	if len(queued) != 2 {
		t.Fatalf("len = %d, want 2 queued", len(queued))
	}
	for _, g := range queued {
		if g.Status != "queued" {
			t.Errorf("got status %q, want queued", g.Status)
		}
	}
}
