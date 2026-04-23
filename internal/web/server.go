package web

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/outlined/autodev/internal/poller"
	"github.com/outlined/autodev/internal/scheduler"
	"github.com/outlined/autodev/internal/store"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// Server is the web dashboard HTTP server.
type Server struct {
	store     *store.Store
	poller    *poller.Poller
	scheduler *scheduler.Scheduler
	logBuffer *LogBuffer
	pages     map[string]*template.Template
	port      int
}

var funcMap = template.FuncMap{
	"statusClass": statusClass,
	"truncate":    truncate,
}

// New creates a new web server.
func New(s *store.Store, p *poller.Poller, sched *scheduler.Scheduler, logBuf *LogBuffer, port int) (*Server, error) {
	// Parse each page template independently so they can each define
	// their own "content" and "title" blocks without conflicting.
	pages := map[string]*template.Template{}

	pageFiles := map[string]string{
		"projects.html":       "templates/projects.html",
		"project_detail.html": "templates/project_detail.html",
		"project_form.html":   "templates/project_form.html",
		"logs.html":           "templates/logs.html",
	}

	for name, path := range pageFiles {
		t, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
			"templates/layout.html",
			"templates/partials/*.html",
			path,
		)
		if err != nil {
			return nil, fmt.Errorf("parsing template %s: %w", name, err)
		}
		pages[name] = t
	}

	// Partials-only template for HTMX fragments
	partials, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parsing partials: %w", err)
	}
	pages["partials"] = partials

	return &Server{
		store:     s,
		poller:    p,
		scheduler: sched,
		logBuffer: logBuf,
		pages:     pages,
		port:      port,
	}, nil
}

// Start starts the HTTP server (blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleProjects)
	mux.HandleFunc("GET /projects/new", s.handleProjectNew)
	mux.HandleFunc("POST /projects/new", s.handleProjectCreate)
	mux.HandleFunc("GET /projects/{id}", s.handleProjectDetail)
	mux.HandleFunc("GET /projects/{id}/edit", s.handleProjectEdit)
	mux.HandleFunc("POST /projects/{id}/edit", s.handleProjectUpdate)
	mux.HandleFunc("POST /projects/{id}/delete", s.handleProjectDelete)
	mux.HandleFunc("POST /projects/{id}/poll", s.handleForcePoll)
	mux.HandleFunc("POST /generations/{id}/retry", s.handleRetryGeneration)
	mux.HandleFunc("GET /generations/{id}/logs", s.handleGenerationLogs)

	// HTMX partial: auto-refresh generation rows
	mux.HandleFunc("GET /projects/{id}/generations-partial", s.handleGenerationsPartial)

	// Logs
	mux.HandleFunc("GET /logs", s.handleLogs)
	mux.HandleFunc("GET /logs/stream", s.handleLogsStream)

	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("dashboard listening", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// Template helpers

func statusClass(status string) string {
	switch status {
	case "queued":
		return "badge-queued"
	case "running":
		return "badge-running"
	case "completed":
		return "badge-completed"
	case "failed":
		return "badge-failed"
	default:
		return "badge-queued"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

