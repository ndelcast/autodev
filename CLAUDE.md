# AutoDev — Claude Code Instructions

## Project Overview

Go-based orchestrator that polls ProdPlanner for tickets, launches ephemeral Docker containers with Claude Code CLI, and creates GitHub PRs.

## Tech Stack

- Go 1.23+ (binary at `/opt/homebrew/bin/go`)
- SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- Docker SDK (`github.com/docker/docker`)
- YAML config (`gopkg.in/yaml.v3`)
- HTMX + html/template + Tailwind CSS (dashboard)

## Build & Test

```bash
make build        # Build binary
make test         # Run all tests
make lint         # Run go vet
make docker-all   # Build all Docker images
make clean        # Remove binary + DB
```

**IMPORTANT**: Use `/opt/homebrew/bin/go` instead of `go` — a shell function overrides the Go binary on this machine.

## Code Conventions

- **Code in English** — variables, methods, classes, comments
- **UI in French** — labels, dashboard text
- Structured logging with `log/slog`
- `flag.NewFlagSet` per CLI subcommand (no external CLI framework)
- Interfaces for testability (`Processor`, `TicketClient`)
- Tests use in-memory SQLite (`:memory:`) and `httptest`

## Project Structure

```
config/             Config loading (YAML + env expansion)
internal/store/     SQLite persistence (projects, generations)
internal/prodplanner/  ProdPlanner API client (OAuth2)
internal/executor/  Docker runner + prompt builder
internal/scheduler/ FIFO queue + semaphore + per-project lock
internal/poller/    Ticker-based ProdPlanner polling
internal/web/       HTMX dashboard (Phase 3)
images/             Docker images (laravel, node, base)
skills/             Skill markdown files for prompts
contexts/           Project context files for prompts
docs/               Architecture docs + prototypes
```

## Git Workflow

- Branch from `main`
- Branch naming: `feat/short-description`, `fix/short-description`
- Commit format: `feat(scope): description`
