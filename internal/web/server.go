package web

import (
	"embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/outlined/autodev/internal/poller"
	"github.com/outlined/autodev/internal/store"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// Server is the web dashboard HTTP server.
type Server struct {
	store  *store.Store
	poller *poller.Poller
	tmpl   *template.Template
	port   int
}

// New creates a new web server.
func New(s *store.Store, p *poller.Poller, port int) (*Server, error) {
	funcMap := template.FuncMap{
		"statusClass": statusClass,
		"truncate":    truncate,
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parsing templates: %w", err)
	}

	return &Server{
		store:  s,
		poller: p,
		tmpl:   tmpl,
		port:   port,
	}, nil
}

// Start starts the HTTP server (blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", s.handleProjects)
	mux.HandleFunc("GET /projects/{id}", s.handleProjectDetail)
	mux.HandleFunc("POST /projects/{id}/poll", s.handleForcePoll)
	mux.HandleFunc("POST /generations/{id}/retry", s.handleRetryGeneration)
	mux.HandleFunc("GET /generations/{id}/logs", s.handleGenerationLogs)

	// HTMX partial: auto-refresh generation rows
	mux.HandleFunc("GET /projects/{id}/generations-partial", s.handleGenerationsPartial)

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
