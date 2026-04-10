package executor

import (
	"strings"
	"testing"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

func TestBuildPrompt(t *testing.T) {
	pb := NewPromptBuilder()

	project := &store.Project{
		Name:           "Test Project",
		ContextContent: "# Test Project\n\nThis is a test project.",
		SkillsContent:  "Use Go idioms. Handle errors.\n\nWrite table-driven tests.",
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

	checks := []string{
		"AutoDev",
		"Test Project",
		"This is a test project",
		"Use Go idioms",
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

func TestBuildPromptNoContext(t *testing.T) {
	pb := NewPromptBuilder()

	project := &store.Project{
		Name: "Test",
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
	if strings.Contains(prompt, "Contexte du projet") {
		t.Error("should not have context section when empty")
	}
}

func TestBuildPRBody(t *testing.T) {
	pb := NewPromptBuilder()
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
