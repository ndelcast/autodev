package executor

import (
	"strings"
	"testing"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

func TestBuildClaudeMD(t *testing.T) {
	pb := NewPromptBuilder()

	project := &store.Project{
		Name:           "Test Project",
		ContextContent: "# Test Project\n\nThis is a test project.",
		SkillsContent:  "Use Go idioms. Handle errors.\n\nWrite table-driven tests.",
	}

	claudeMD := pb.BuildClaudeMD(project)

	checks := []string{
		"Test Project",
		"This is a test project",
		"Use Go idioms",
		"Write table-driven tests",
		"Ne fais PAS de git commit",
		"Code en anglais",
	}
	for _, check := range checks {
		if !strings.Contains(claudeMD, check) {
			t.Errorf("CLAUDE.md missing %q", check)
		}
	}
}

func TestBuildClaudeMDEmpty(t *testing.T) {
	pb := NewPromptBuilder()

	project := &store.Project{Name: "Empty"}
	claudeMD := pb.BuildClaudeMD(project)

	if strings.Contains(claudeMD, "Contexte du projet") {
		t.Error("should not have context section when empty")
	}
	if strings.Contains(claudeMD, "Compétences") {
		t.Error("should not have skills section when empty")
	}
	if !strings.Contains(claudeMD, "Règles générales") {
		t.Error("should always have general rules")
	}
}

func TestBuildPrompt(t *testing.T) {
	pb := NewPromptBuilder()

	ticket := &prodplanner.Ticket{
		FormattedNumber: "TEST-42",
		Type:            "feat",
		Title:           "Add user authentication",
		Description:     "Implement JWT-based auth.",
		Priority:        "high",
		Size:            "m",
	}

	prompt := pb.BuildPrompt(ticket)

	checks := []string{
		"TEST-42",
		"Add user authentication",
		"Implement JWT-based auth",
		"feat",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}

	// Should NOT contain context/skills — those are in CLAUDE.md
	if strings.Contains(prompt, "Contexte") {
		t.Error("prompt should not contain context (moved to CLAUDE.md)")
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
