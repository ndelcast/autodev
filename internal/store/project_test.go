package store

import (
	"testing"
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
		ContextContent:       "# Dispoo context",
		SkillsContent:        "## Laravel\n## Data modeling",
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
	if got.ContextContent != "# Dispoo context" {
		t.Errorf("ContextContent = %q, want %q", got.ContextContent, "# Dispoo context")
	}
	if got.SkillsContent != "## Laravel\n## Data modeling" {
		t.Errorf("SkillsContent = %q", got.SkillsContent)
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

func TestUpdateProject(t *testing.T) {
	s := newTestStore(t)

	p := &Project{
		Name:                 "Test",
		Slug:                 "test",
		ProdPlannerProjectID: 1,
		GithubRepo:           "org/repo",
		DockerImage:          "autodev-base:latest",
		Status:               "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	p.Name = "Test Updated"
	p.GithubRepo = "org/repo-v2"
	p.SkillsContent = "## Go\n## Testing"
	if err := s.UpdateProject(p); err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}

	got, err := s.GetProject(p.ID)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if got.Name != "Test Updated" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Updated")
	}
	if got.GithubRepo != "org/repo-v2" {
		t.Errorf("GithubRepo = %q, want %q", got.GithubRepo, "org/repo-v2")
	}
	if got.SkillsContent != "## Go\n## Testing" {
		t.Errorf("SkillsContent = %q", got.SkillsContent)
	}
}

func TestDeleteProject(t *testing.T) {
	s := newTestStore(t)

	p := &Project{
		Name:       "ToDelete",
		Slug:       "to-delete",
		GithubRepo: "org/repo",
		DockerImage: "autodev-base:latest",
		Status:     "idle",
	}
	if err := s.CreateProject(p); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if err := s.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	projects, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("len = %d after delete, want 0", len(projects))
	}
}
