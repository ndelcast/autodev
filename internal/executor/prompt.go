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

// BuildClaudeMD assembles the CLAUDE.md content from project context and skills.
// This is loaded automatically by Claude Code, keeping the prompt lean.
func (pb *PromptBuilder) BuildClaudeMD(project *store.Project) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — Instructions AutoDev\n\n", project.Name)

	if project.ContextContent != "" {
		fmt.Fprintf(&b, "## Contexte du projet\n\n%s\n\n", strings.TrimSpace(project.ContextContent))
	}

	if project.SkillsContent != "" {
		fmt.Fprintf(&b, "## Compétences & conventions\n\n%s\n\n", strings.TrimSpace(project.SkillsContent))
	}

	fmt.Fprintf(&b, "## Règles générales\n\n")
	fmt.Fprintf(&b, "- Code en anglais, UI/messages en français.\n")
	fmt.Fprintf(&b, "- Ne fais PAS de git commit/push — le script appelant gère ça.\n")
	fmt.Fprintf(&b, "- Ne modifie pas de fichiers qui ne sont pas liés au ticket.\n")
	fmt.Fprintf(&b, "- Écris des tests si c'est pertinent pour la fonctionnalité.\n")

	return b.String()
}

// BuildPrompt assembles a lean prompt containing only the ticket to implement.
// Project context and skills are in CLAUDE.md, loaded separately by Claude Code.
func (pb *PromptBuilder) BuildPrompt(ticket *prodplanner.Ticket) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Implémente le ticket suivant :\n\n")
	fmt.Fprintf(&b, "- **Numéro** : %s\n", ticket.FormattedNumber)
	fmt.Fprintf(&b, "- **Type** : %s\n", ticket.Type)
	fmt.Fprintf(&b, "- **Titre** : %s\n", ticket.Title)
	fmt.Fprintf(&b, "- **Priorité** : %s\n", ticket.Priority)
	fmt.Fprintf(&b, "- **Taille** : %s\n\n", ticket.Size)
	fmt.Fprintf(&b, "### Description\n\n%s\n", ticket.Description)

	return b.String()
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
