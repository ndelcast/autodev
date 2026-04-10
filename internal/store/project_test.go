package store

import (
	"testing"

	"github.com/outlined/autodev/config"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetProject(t *testing.T) {
	s := newTestStore(t)

	p := &Project{
		ProdPlannerProjectID: 13,
		Name:                 "Dispoo",
		Slug:                 "dispoo",
		GithubRepo:           "outlined/dispoo",
		DockerImage:          "autodev-laravel:latest",
		ContextFile:          "dispoo.md",
		Skills:               []string{"laravel", "data-modeling"},
		AutodevDeveloperID:   3,
		DoneColumnID:         43,
		Status:               "idle",
	}

	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if p.ID == 0 {
		t.Fatal("expected non-zero ID after create")
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}

	if got.Name != "Dispoo" {
		t.Errorf("Name = %q, want %q", got.Name, "Dispoo")
	}
	if got.Slug != "dispoo" {
		t.Errorf("Slug = %q, want %q", got.Slug, "dispoo")
	}
	if got.GithubRepo != "outlined/dispoo" {
		t.Errorf("GithubRepo = %q, want %q", got.GithubRepo, "outlined/dispoo")
	}
	if len(got.Skills) != 2 || got.Skills[0] != "laravel" || got.Skills[1] != "data-modeling" {
		t.Errorf("Skills = %v, want [laravel data-modeling]", got.Skills)
	}
	if got.Status != "idle" {
		t.Errorf("Status = %q, want %q", got.Status, "idle")
	}
}

func TestGetProjectBySlug(t *testing.T) {
	s := newTestStore(t)

	p := &Project{
		ProdPlannerProjectID: 1,
		Name:                 "Test",
		Slug:                 "test-slug",
		GithubRepo:           "org/repo",
		DockerImage:          "autodev-base:latest",
		AutodevDeveloperID:   1,
		DoneColumnID:         10,
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	got, err := s.GetProjectBySlug("test-slug")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if got.ID != p.ID {
		t.Errorf("ID = %d, want %d", got.ID, p.ID)
	}
}

func TestListProjects(t *testing.T) {
	s := newTestStore(t)

	for _, name := range []string{"Beta", "Alpha"} {
		p := &Project{
			ProdPlannerProjectID: 1,
			Name:                 name,
			Slug:                 name,
			GithubRepo:           "org/" + name,
			DockerImage:          "autodev-base:latest",
			AutodevDeveloperID:   1,
			DoneColumnID:         10,
			Status:               "idle",
		}
		if err := s.CreateProject(p); err != nil {
			t.Fatalf("CreateProject(%s): %v", name, err)
		}
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("len = %d, want 2", len(projects))
	}
	// Ordered by name
	if projects[0].Name != "Alpha" {
		t.Errorf("first project = %q, want Alpha", projects[0].Name)
	}
}

func TestUpdateProjectStatus(t *testing.T) {
	s := newTestStore(t)

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

	if err := s.UpdateProjectStatus(p.ID, "running"); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Status != "running" {
		t.Errorf("Status = %q, want %q", got.Status, "running")
	}
}

func TestSeedProjects(t *testing.T) {
	s := newTestStore(t)

	configs := []config.ProjectConfig{
		{
			Name:                 "Dispoo",
			Slug:                 "dispoo",
			ProdPlannerProjectID: 13,
			GithubRepo:           "outlined/dispoo",
			DockerImage:          "autodev-laravel:latest",
			ContextFile:          "dispoo.md",
			Skills:               []string{"laravel"},
			AutodevDeveloperID:   3,
			DoneColumnID:         43,
		},
	}

	if err := s.SeedProjects(configs); err != nil {
		t.Fatalf("SeedProjects: %v", err)
	}

	got, err := s.GetProjectBySlug("dispoo")
	if err != nil {
		t.Fatalf("GetProjectBySlug: %v", err)
	}
	if got.Name != "Dispoo" {
		t.Errorf("Name = %q, want Dispoo", got.Name)
	}

	// Seed again with updated name — should upsert
	configs[0].Name = "Dispoo V2"
	if err := s.SeedProjects(configs); err != nil {
		t.Fatalf("SeedProjects (upsert): %v", err)
	}

	got, err = s.GetProjectBySlug("dispoo")
	if err != nil {
		t.Fatalf("GetProjectBySlug after upsert: %v", err)
	}
	if got.Name != "Dispoo V2" {
		t.Errorf("Name after upsert = %q, want Dispoo V2", got.Name)
	}

	// Should still be only 1 project
	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("len = %d after upsert, want 1", len(projects))
	}
}
