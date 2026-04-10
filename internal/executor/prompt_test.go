package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

func TestBuildPrompt(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	contextsDir := filepath.Join(dir, "contexts")
	os.MkdirAll(skillsDir, 0755)
	os.MkdirAll(contextsDir, 0755)

	os.WriteFile(filepath.Join(contextsDir, "test.md"), []byte("# Test Project\n\nThis is a test project."), 0644)
	os.WriteFile(filepath.Join(skillsDir, "go.md"), []byte("Use Go idioms. Handle errors."), 0644)
	os.WriteFile(filepath.Join(skillsDir, "testing.md"), []byte("Write table-driven tests."), 0644)

	pb := NewPromptBuilder(skillsDir, contextsDir)

	project := &store.Project{
		Name:        "Test Project",
		ContextFile: "test.md",
		Skills:      []string{"go", "testing"},
	}

	ticket := &prodplanner.Ticket{
		FormattedNumber: "TEST-42",
		Type:            "feat",
		Title:           "Add user authentication",
		Description:     "Implement JWT-based auth.",
		Priority:        "high",
		Size:            "m",
	}

	prompt, err := pb.Build(project, ticket)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Check all sections are present
	checks := []string{
		"AutoDev",
		"Test Project",
		"This is a test project",
		"Skill : go",
		"Use Go idioms",
		"Skill : testing",
		"Write table-driven tests",
		"TEST-42",
		"Add user authentication",
		"Implement JWT-based auth",
		"Ne fais PAS de git commit",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}

func TestBuildPromptMissingContext(t *testing.T) {
	dir := t.TempDir()
	pb := NewPromptBuilder(filepath.Join(dir, "skills"), filepath.Join(dir, "contexts"))

	project := &store.Project{
		Name:        "Test",
		ContextFile: "nonexistent.md",
	}
	ticket := &prodplanner.Ticket{Title: "test"}

	_, err := pb.Build(project, ticket)
	if err == nil {
		t.Fatal("expected error for missing context file")
	}
}

func TestBuildPromptNoContext(t *testing.T) {
	dir := t.TempDir()
	pb := NewPromptBuilder(filepath.Join(dir, "skills"), filepath.Join(dir, "contexts"))

	project := &store.Project{
		Name:        "Test",
		ContextFile: "", // no context file
	}
	ticket := &prodplanner.Ticket{
		FormattedNumber: "T-1",
		Title:           "Test",
	}

	prompt, err := pb.Build(project, ticket)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !strings.Contains(prompt, "T-1") {
		t.Error("prompt missing ticket number")
	}
}

func TestBuildPRBody(t *testing.T) {
	pb := NewPromptBuilder("", "")
	ticket := &prodplanner.Ticket{
		FormattedNumber: "DISP-385",
		Title:           "Télécharger les icônes",
		Description:     "En tant que prestataire...",
	}

	body := pb.BuildPRBody(ticket)
	if !strings.Contains(body, "Télécharger les icônes") {
		t.Error("PR body missing title")
	}
	if !strings.Contains(body, "AutoDev") {
		t.Error("PR body missing AutoDev attribution")
	}
	if !strings.Contains(body, "DISP-385") {
		t.Error("PR body missing ticket number")
	}
}
