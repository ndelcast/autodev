package executor

import (
	"fmt"
	"strings"

	"github.com/outlined/autodev/internal/prodplanner"
	"github.com/outlined/autodev/internal/store"
)

type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// Build assembles the full prompt from project context, skills, and ticket details.
func (pb *PromptBuilder) Build(project *store.Project, ticket *prodplanner.Ticket) (string, error) {
	var b strings.Builder

	// 1. System preamble
	fmt.Fprintf(&b, "# AutoDev — Implémentation de ticket\n\n")
	fmt.Fprintf(&b, "Tu es AutoDev, un développeur IA qui travaille sur le projet **%s**.\n", project.Name)
	fmt.Fprintf(&b, "Tu implémente des fonctionnalités basées sur des tickets ProdPlanner.\n\n")

	// 2. Project context
	if project.ContextContent != "" {
		fmt.Fprintf(&b, "## Contexte du projet\n\n%s\n\n", strings.TrimSpace(project.ContextContent))
	}

	// 3. Skills
	if project.SkillsContent != "" {
		fmt.Fprintf(&b, "## Compétences & conventions\n\n%s\n\n", strings.TrimSpace(project.SkillsContent))
	}

	// 4. Ticket
	fmt.Fprintf(&b, "## Ticket à implémenter\n\n")
	fmt.Fprintf(&b, "- **Numéro** : %s\n", ticket.FormattedNumber)
	fmt.Fprintf(&b, "- **Type** : %s\n", ticket.Type)
	fmt.Fprintf(&b, "- **Titre** : %s\n", ticket.Title)
	fmt.Fprintf(&b, "- **Priorité** : %s\n", ticket.Priority)
	fmt.Fprintf(&b, "- **Taille** : %s\n\n", ticket.Size)
	fmt.Fprintf(&b, "### Description\n\n%s\n\n", ticket.Description)

	// 5. Instructions
	fmt.Fprintf(&b, "## Instructions\n\n")
	fmt.Fprintf(&b, "- Implémente le ticket ci-dessus en suivant les conventions du projet et les skills.\n")
	fmt.Fprintf(&b, "- Écris des tests si c'est pertinent pour la fonctionnalité.\n")
	fmt.Fprintf(&b, "- Ne fais PAS de git commit/push — le script appelant gère ça.\n")
	fmt.Fprintf(&b, "- Ne modifie pas de fichiers qui ne sont pas liés au ticket.\n")
	fmt.Fprintf(&b, "- Code en anglais, UI/messages en français.\n")

	return b.String(), nil
}

// BuildPRBody creates the markdown body for the pull request.
func (pb *PromptBuilder) BuildPRBody(ticket *prodplanner.Ticket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", ticket.Title)
	fmt.Fprintf(&b, "%s\n\n", ticket.Description)
	fmt.Fprintf(&b, "---\n")
	fmt.Fprintf(&b, "Généré par **AutoDev** | Ticket %s\n", ticket.FormattedNumber)
	return b.String()
}
