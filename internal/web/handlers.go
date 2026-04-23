package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

	// Parsed from ClaudeOutput JSON
	ClaudeResult    string
	ClaudeCostUSD   string
	ClaudeTurns     int
	ClaudeStopReason string
	ClaudeDuration  string
	ClaudeModel     string
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

func (s *Server) render(w http.ResponseWriter, page string, data any) {
	t, ok := s.pages[page]
	if !ok {
		slog.Error("template not found", "page", page)
		http.Error(w, "Erreur interne", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, page, data); err != nil {
		slog.Error("rendering template", "page", page, "error", err)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	t := s.pages["partials"]
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("rendering partial", "name", name, "error", err)
	}
}

type logsData struct {
	Entries []LogEntry
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	entries := s.logBuffer.Entries()
	s.render(w, "logs.html", logsData{Entries: entries})
}

func (s *Server) handleLogsStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE non supporté", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.logBuffer.Subscribe()
	defer s.logBuffer.Unsubscribe(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-ch:
			levelClass := "text-zinc-400"
			switch entry.Level {
			case "ERROR":
				levelClass = "text-red-400"
			case "WARN":
				levelClass = "text-amber-400"
			case "INFO":
				levelClass = "text-emerald-400"
			}

			html := fmt.Sprintf(
				`<div class="flex gap-3 py-1 px-3 font-mono text-xs border-b border-surface-3/30 hover:bg-surface-2/50">`+
					`<span class="text-zinc-600 shrink-0">%s</span>`+
					`<span class="%s shrink-0 w-12">%s</span>`+
					`<span class="text-zinc-300">%s</span>`+
					`<span class="text-zinc-500">%s</span>`+
					`</div>`,
				entry.Time.Format("15:04:05"),
				levelClass,
				entry.Level,
				entry.Message,
				entry.Attrs,
			)

			fmt.Fprintf(w, "data: %s\n\n", html)
			flusher.Flush()
		}
	}
}

type projectFormData struct {
	Project      *store.Project
	IsEdit       bool
	DockerImages []string
	Error        string
}

func (s *Server) handleProjectNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, "project_form.html", projectFormData{
		Project:      &store.Project{Status: "idle"},
		DockerImages: s.listDockerImages(),
	})
}

func (s *Server) handleProjectCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	p := &store.Project{
		Name:                 r.FormValue("name"),
		Slug:                 r.FormValue("slug"),
		GithubRepo:           r.FormValue("github_repo"),
		DockerImage:          r.FormValue("docker_image"),
		ContextContent:       r.FormValue("context_content"),
		SkillsContent:        r.FormValue("skills_content"),
		ProdPlannerProjectID: atoi(r.FormValue("prodplanner_project_id")),
		AutodevDeveloperID:   atoi(r.FormValue("autodev_developer_id")),
		DoneColumnID:         atoi(r.FormValue("done_column_id")),
		Status:               "idle",
	}

	if p.Name == "" || p.Slug == "" || p.GithubRepo == "" || p.DockerImage == "" {
		s.render(w, "project_form.html", projectFormData{
			Project:      p,
			DockerImages: s.listDockerImages(),
			Error:        "Les champs Nom, Slug, Repo GitHub et Image Docker sont requis.",
		})
		return
	}

	if err := s.store.CreateProject(p); err != nil {
		slog.Error("creating project", "error", err)
		s.render(w, "project_form.html", projectFormData{
			Project:      p,
			DockerImages: s.listDockerImages(),
			Error:        fmt.Sprintf("Erreur: %v", err),
		})
		return
	}

	slog.Info("project created via dashboard", "id", p.ID, "slug", p.Slug)
	http.Redirect(w, r, fmt.Sprintf("/projects/%d", p.ID), http.StatusSeeOther)
}

func (s *Server) handleProjectEdit(w http.ResponseWriter, r *http.Request) {
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

	s.render(w, "project_form.html", projectFormData{
		Project:      project,
		IsEdit:       true,
		DockerImages: s.listDockerImages(),
	})
}

func (s *Server) handleProjectUpdate(w http.ResponseWriter, r *http.Request) {
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

	r.ParseForm()
	project.Name = r.FormValue("name")
	project.Slug = r.FormValue("slug")
	project.GithubRepo = r.FormValue("github_repo")
	project.DockerImage = r.FormValue("docker_image")
	project.ContextContent = r.FormValue("context_content")
	project.SkillsContent = r.FormValue("skills_content")
	project.ProdPlannerProjectID = atoi(r.FormValue("prodplanner_project_id"))
	project.AutodevDeveloperID = atoi(r.FormValue("autodev_developer_id"))
	project.DoneColumnID = atoi(r.FormValue("done_column_id"))

	if project.Name == "" || project.Slug == "" || project.GithubRepo == "" || project.DockerImage == "" {
		s.render(w, "project_form.html", projectFormData{
			Project:      project,
			IsEdit:       true,
			DockerImages: s.listDockerImages(),
			Error:        "Les champs Nom, Slug, Repo GitHub et Image Docker sont requis.",
		})
		return
	}

	if err := s.store.UpdateProject(project); err != nil {
		slog.Error("updating project", "error", err)
		s.render(w, "project_form.html", projectFormData{
			Project:      project,
			IsEdit:       true,
			DockerImages: s.listDockerImages(),
			Error:        fmt.Sprintf("Erreur: %v", err),
		})
		return
	}

	slog.Info("project updated via dashboard", "id", project.ID, "slug", project.Slug)
	http.Redirect(w, r, fmt.Sprintf("/projects/%d", project.ID), http.StatusSeeOther)
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := s.store.DeleteProject(id); err != nil {
		slog.Error("deleting project", "error", err)
		http.Error(w, "Erreur interne", 500)
		return
	}

	slog.Info("project deleted via dashboard", "id", id)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) listDockerImages() []string {
	return []string{
		"autodev-laravel:latest",
		"autodev-node:latest",
		"autodev-base:latest",
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func toGenerationRow(g store.Generation) generationRow {
	row := generationRow{
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

	// Parse Claude output JSON for summary display
	if g.ClaudeOutput != "" {
		var parsed struct {
			Result       string  `json:"result"`
			NumTurns     int     `json:"num_turns"`
			StopReason   string  `json:"stop_reason"`
			Subtype      string  `json:"subtype"`
			DurationMS   int     `json:"duration_ms"`
			TotalCostUSD float64 `json:"total_cost_usd"`
		}
		if err := json.Unmarshal([]byte(g.ClaudeOutput), &parsed); err == nil {
			row.ClaudeTurns = parsed.NumTurns
			row.ClaudeCostUSD = fmt.Sprintf("%.2f", parsed.TotalCostUSD)
			row.ClaudeStopReason = parsed.Subtype
			if row.ClaudeStopReason == "" {
				row.ClaudeStopReason = parsed.StopReason
			}
			if parsed.DurationMS > 0 {
				mins := parsed.DurationMS / 60000
				secs := (parsed.DurationMS % 60000) / 1000
				row.ClaudeDuration = fmt.Sprintf("%dm%02ds", mins, secs)
			}
			row.ClaudeResult = parsed.Result
		}
	}

	return row
}
