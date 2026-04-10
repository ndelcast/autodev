package web

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/outlined/autodev/internal/store"
)

type projectsData struct {
	Projects []projectRow
}

type projectRow struct {
	ID         int64
	Name       string
	Slug       string
	GithubRepo string
	Image      string
	Status     string
	Total      int
	Completed  int
	Failed     int
	Running    int
}

type projectDetailData struct {
	Project     projectRow
	Generations []generationRow
}

type generationRow struct {
	ID              int64
	TicketNumber    string
	TicketTitle     string
	Status          string
	BranchName      string
	PRUrl           string
	DurationSeconds int
	Attempt         int
	ErrorMessage    string
	PromptSent      string
	ClaudeOutput    string
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	projects, err := s.store.ListProjects()
	if err != nil {
		slog.Error("listing projects", "error", err)
		http.Error(w, "Erreur interne", 500)
		return
	}

	var rows []projectRow
	for _, p := range projects {
		row := projectRow{
			ID:         p.ID,
			Name:       p.Name,
			Slug:       p.Slug,
			GithubRepo: p.GithubRepo,
			Image:      p.DockerImage,
			Status:     p.Status,
		}

		gens, err := s.store.ListGenerationsByProject(p.ID)
		if err == nil {
			row.Total = len(gens)
			for _, g := range gens {
				switch g.Status {
				case "completed":
					row.Completed++
				case "failed":
					row.Failed++
				case "running":
					row.Running++
				}
			}
		}
		rows = append(rows, row)
	}

	s.render(w, "projects.html", projectsData{Projects: rows})
}

func (s *Server) handleProjectDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	project, err := s.store.GetProject(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	gens, err := s.store.ListGenerationsByProject(id)
	if err != nil {
		slog.Error("listing generations", "error", err)
		http.Error(w, "Erreur interne", 500)
		return
	}

	var genRows []generationRow
	for _, g := range gens {
		genRows = append(genRows, toGenerationRow(g))
	}

	pRow := projectRow{
		ID:         project.ID,
		Name:       project.Name,
		Slug:       project.Slug,
		GithubRepo: project.GithubRepo,
		Image:      project.DockerImage,
		Status:     project.Status,
	}
	for _, g := range genRows {
		switch g.Status {
		case "completed":
			pRow.Completed++
		case "failed":
			pRow.Failed++
		case "running":
			pRow.Running++
		}
		pRow.Total++
	}

	s.render(w, "project_detail.html", projectDetailData{
		Project:     pRow,
		Generations: genRows,
	})
}

func (s *Server) handleGenerationsPartial(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	gens, err := s.store.ListGenerationsByProject(id)
	if err != nil {
		http.Error(w, "Erreur interne", 500)
		return
	}

	var genRows []generationRow
	for _, g := range gens {
		genRows = append(genRows, toGenerationRow(g))
	}

	s.renderPartial(w, "generation_rows", genRows)
}

func (s *Server) handleForcePoll(w http.ResponseWriter, r *http.Request) {
	if s.poller == nil {
		http.Error(w, "Poller non configuré", 500)
		return
	}

	n, err := s.poller.PollOnce(context.Background())
	if err != nil {
		slog.Error("force poll failed", "error", err)
		http.Error(w, "Erreur de polling", 500)
		return
	}

	slog.Info("force poll completed", "new_tickets", n)

	// Redirect back to the project detail page
	id := r.PathValue("id")
	http.Redirect(w, r, "/projects/"+id, http.StatusSeeOther)
}

func (s *Server) handleRetryGeneration(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	gen, err := s.store.GetGeneration(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if gen.Status != "failed" {
		http.Error(w, "Seules les générations échouées peuvent être relancées", 400)
		return
	}

	gen.Status = "queued"
	gen.Attempt++
	gen.ErrorMessage = ""
	gen.StartedAt = nil
	gen.CompletedAt = nil

	if err := s.store.UpdateGeneration(gen); err != nil {
		slog.Error("resetting generation", "error", err)
		http.Error(w, "Erreur interne", 500)
		return
	}

	slog.Info("generation queued for retry via dashboard", "id", gen.ID, "attempt", gen.Attempt)

	// Redirect back
	http.Redirect(w, r, "/projects/"+strconv.FormatInt(gen.ProjectID, 10), http.StatusSeeOther)
}

func (s *Server) handleGenerationLogs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	gen, err := s.store.GetGeneration(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	s.renderPartial(w, "generation_logs", toGenerationRow(*gen))
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("rendering template", "name", name, "error", err)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("rendering partial", "name", name, "error", err)
	}
}

func toGenerationRow(g store.Generation) generationRow {
	return generationRow{
		ID:              g.ID,
		TicketNumber:    g.TicketNumber,
		TicketTitle:     g.TicketTitle,
		Status:          g.Status,
		BranchName:      g.BranchName,
		PRUrl:           g.PRUrl,
		DurationSeconds: g.DurationSeconds,
		Attempt:         g.Attempt,
		ErrorMessage:    g.ErrorMessage,
		PromptSent:      g.PromptSent,
		ClaudeOutput:    g.ClaudeOutput,
	}
}
