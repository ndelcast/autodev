# Plan d'implémentation — AutoDev POC

## Contexte

Service Go qui orchestre la génération automatique de code : poll ProdPlanner pour les tickets assignés, lance des containers Docker éphémères avec Claude Code CLI, et crée des PRs GitHub.

---

## Phase 1 — Core + CLI (Steps 1-7)

### Step 1 : Init Go module + Config ✅
**Fichiers :** `go.mod`, `.gitignore`, `config/config.go`, `config/config_test.go`, `config.yaml`

- Module `github.com/outlined/autodev`, Go 1.23
- Deps : `modernc.org/sqlite`, `github.com/docker/docker`, `gopkg.in/yaml.v3`
- `Config` struct avec nested `ProdPlannerConfig`, `ClaudeConfig`, `PollingConfig`, `DockerConfig`, `ProjectConfig[]`
- `Load(path)` : lit YAML, expand env vars (`os.ExpandEnv`), applique les défauts
- `.gitignore` : binary, `autodev.db`, `workspaces/`, `.env`, `*.log`
- **Vérif :** `go test ./config/...`

### Step 2 : Store SQLite — Schema + Project CRUD ✅
**Fichiers :** `internal/store/db.go`, `internal/store/models.go`, `internal/store/project.go`, `internal/store/project_test.go`

- `Store` struct wrappant `*sql.DB`
- `New(dbPath)` : ouvre SQLite (driver `modernc.org/sqlite`), WAL mode, FK, auto-migrate
- `models.go` : structs `Project` et `Generation` (du brief)
- `project.go` : `CreateProject`, `GetProject`, `GetProjectBySlug`, `ListProjects`, `UpdateProjectStatus`, `SeedProjects([]config.ProjectConfig)` (upsert par slug)
- Skills stockés en comma-separated string en DB
- **Vérif :** `go test ./internal/store/... -run TestProject` (SQLite `:memory:`)

### Step 3 : Store SQLite — Generation CRUD ✅
**Fichiers :** `internal/store/generation.go`, `internal/store/generation_test.go`

- `CreateGeneration`, `GetGeneration`, `UpdateGeneration`, `ListGenerationsByProject` (DESC), `GetGenerationByTicketID`, `ListQueuedGenerations`
- **Vérif :** `go test ./internal/store/... -run TestGeneration`

### Step 4 : Client HTTP ProdPlanner ✅
**Fichiers :** `internal/prodplanner/client.go`, `internal/prodplanner/models.go`, `internal/prodplanner/client_test.go`

- `Client` avec OAuth2 client credentials (token caché, refresh auto)
- `doRequest` : check expiry, re-auth si besoin, set Bearer header
- Méthodes : `ListTickets(ctx, opts)`, `GetTicket(ctx, id)`, `MoveTicket(ctx, ticketID, columnID)`, `CompleteTicket(ctx, id)`
- Models Go : `Ticket`, `BoardColumn`, `TokenResponse`
- **Vérif :** `go test ./internal/prodplanner/...` (httptest mocks)

### Step 5 : Prompt Builder + Skills/Contexts ✅
**Fichiers :** `internal/executor/prompt.go`, `internal/executor/prompt_test.go`, `skills/laravel.md`, `skills/inertia-vue.md`, `skills/data-modeling.md`, `contexts/dispoo.md`

- `PromptBuilder.Build(project, ticket)` : assemble context.md + skills + ticket en prompt structuré
- `BuildPRBody(ticket)` : markdown pour le body de la PR
- Fichiers .md avec le contenu décrit dans le brief
- **Vérif :** `go test ./internal/executor/... -run TestPrompt`

### Step 6 : Docker Executor ✅
**Fichiers :** `internal/executor/executor.go`, `internal/executor/docker.go`, `images/laravel/Dockerfile`, `images/laravel/autodev-exec.sh`, `images/base/Dockerfile`, `images/base/autodev-exec.sh`

- `DockerRunner.RunContainer(ctx, opts)` : Create → Start → Wait (avec timeout) → Logs → Remove
- `Executor.Process(ctx, gen)` : le flow complet — load project, fetch ticket, mark running, build prompt, write temp files, ensure workspace (git clone), run container, collect results, update ProdPlanner, save generation
- Dockerfiles et `autodev-exec.sh` tels que dans le brief
- **Vérif :** `go build ./internal/executor/...`, `docker build -t autodev-laravel images/laravel/`

### Step 7 : main.go + CLI + CLAUDE.md + Makefile ✅
**Fichiers :** `main.go`, `CLAUDE.md`, `Makefile`

- CLI avec `flag.NewFlagSet` par sous-commande : `serve`, `run --project --ticket`, `retry --generation`, `projects`, `generations --project`
- Wire tout : config → store → seed projects → prodplanner → docker → executor
- `slog` setup, `signal.NotifyContext` pour graceful shutdown
- Makefile : `build`, `test`, `lint`, `docker-laravel`, `docker-base`, `docker-all`, `clean`
- CLAUDE.md adapté au projet Go
- **Vérif :** `make build && make test`, `./autodev projects`, `./autodev run --project=dispoo --ticket=<id>`

---

## Phase 2 — Polling + Scheduler (Steps 8-10)

### Step 8 : Scheduler (parallélisme contrôlé) ✅
**Fichiers :** `internal/scheduler/scheduler.go`, `internal/scheduler/scheduler_test.go`

- Interface `Processor` pour découpler de l'Executor concret (testabilité)
- FIFO queue + semaphore global (`chan struct{}`, taille `max_concurrent`) + map `running` par project ID
- `Enqueue(gen)`, `Shutdown()` (drain wg)
- `processGeneration` : acquire sem → mark project running → exec → release → processNext du même projet
- **Vérif :** `go test ./internal/scheduler/...` — tests de concurrence (2 projets en parallèle, même projet séquentiel, semaphore respecté)

### Step 9 : Poller ✅
**Fichiers :** `internal/poller/poller.go`, `internal/poller/poller_test.go`

- `Start(ctx)` : ticker loop, pour chaque projet → `ListTickets(assigned_to=autodev_dev_id)` → filtre tickets déjà traités → crée Generation queued → enqueue dans scheduler
- `PollOnce(ctx)` : un cycle, retourne le nombre de nouveaux tickets (pour le dashboard "force poll")
- Interfaces `TicketClient` et `Enqueuer` pour testabilité
- **Vérif :** `go test ./internal/poller/...` (mocks prodplanner + scheduler)

### Step 10 : Commande `serve` + images Docker node ✅
**Fichiers :** update `main.go`, `images/node/Dockerfile`, `images/node/autodev-exec.sh`

- `serve` : scheduler + poller + dashboard, block sur ctx.Done, graceful shutdown
- Re-enqueue des générations "queued" au redémarrage
- Image Docker node (Node 22 + git + gh + claude)
- **Vérif :** `./autodev serve` poll et traite les tickets. Ctrl+C = shutdown propre.

---

## Phase 3 — Dashboard + Déploiement (Steps 11-12)

### Step 11 : Web Dashboard HTMX ✅
**Fichiers :** `internal/web/server.go`, `internal/web/handlers.go`, `internal/web/templates/layout.html`, `internal/web/templates/projects.html`, `internal/web/templates/project_detail.html`, `internal/web/templates/partials/generation_row.html`

- Templates embarqués via `//go:embed templates/*`
- Routes Go 1.22+ : `GET /`, `GET /projects/{id}`, `POST /projects/{id}/poll`, `POST /generations/{id}/retry`, `GET /generations/{id}/logs`, `GET /projects/{id}/generations-partial`
- HTMX : auto-refresh rows running/queued toutes les 5s, expand/collapse logs, actions sans navigation
- Tailwind CSS + HTMX via CDN. UI en français.
- Design basé sur les prototypes `docs/prototype/`
- **Vérif :** `./autodev serve` → `http://localhost:8080/`

### Step 12 : Dockerfile orchestrateur + docker-compose ✅
**Fichiers :** `Dockerfile`, `docker-compose.yaml`

- Multi-stage : golang:1.23-alpine builder → alpine:3.20 runtime + docker-cli
- Embed skills/, contexts/, config.yaml dans l'image
- docker-compose : monte `/var/run/docker.sock`, expose 8080, env_file
- **Vérif :** `docker compose up` → dashboard + polling fonctionnels

---

## Structure finale du projet

```
autodev/
├── main.go                          # CLI entry point (serve, run, retry, projects, generations)
├── config.yaml                      # Configuration YAML (credentials via ${ENV_VARS})
├── CLAUDE.md                        # Instructions pour Claude Code
├── Makefile                         # build, test, lint, docker-*
├── Dockerfile                       # Multi-stage (Go builder → Alpine runtime)
├── docker-compose.yaml              # Déploiement avec Docker socket
├── config/
│   ├── config.go                    # Chargement YAML + expansion env + défauts
│   └── config_test.go
├── internal/
│   ├── store/
│   │   ├── db.go                    # SQLite init + migrations (WAL, FK)
│   │   ├── models.go                # Project, Generation structs
│   │   ├── project.go               # CRUD projets + SeedProjects
│   │   ├── project_test.go
│   │   ├── generation.go            # CRUD générations + queries
│   │   └── generation_test.go
│   ├── prodplanner/
│   │   ├── client.go                # HTTP client OAuth2 (token auto-refresh)
│   │   ├── models.go                # Ticket, BoardColumn, etc.
│   │   └── client_test.go
│   ├── executor/
│   │   ├── executor.go              # Orchestration complète d'une génération
│   │   ├── docker.go                # DockerRunner (create → start → wait → logs → remove)
│   │   ├── prompt.go                # PromptBuilder (context + skills + ticket)
│   │   └── prompt_test.go
│   ├── scheduler/
│   │   ├── scheduler.go             # FIFO queue + semaphore + per-project lock
│   │   └── scheduler_test.go
│   ├── poller/
│   │   ├── poller.go                # Ticker loop → ProdPlanner → enqueue
│   │   └── poller_test.go
│   └── web/
│       ├── server.go                # HTTP server + template loading
│       ├── handlers.go              # Route handlers
│       └── templates/
│           ├── layout.html
│           ├── projects.html
│           ├── project_detail.html
│           └── partials/
│               └── generation_row.html
├── images/
│   ├── laravel/                     # PHP 8.4 + Composer + Claude Code
│   ├── node/                        # Node 22 + Claude Code
│   └── base/                        # Alpine + Node + Claude Code
├── skills/                          # Fichiers .md de compétences (laravel, inertia-vue, data-modeling)
├── contexts/                        # Fichiers .md de contexte projet (dispoo)
└── docs/
    ├── BRIEF-AUTODEV-POC.md         # Brief technique du POC
    ├── plan.md                      # Ce fichier
    └── prototype/                   # Prototypes HTML du dashboard
```

---

## Décisions clés

- **Projects en YAML** → seeded en SQLite au démarrage (pas de CRUD UI pour le POC)
- **Interfaces pour testabilité** : `Processor` (scheduler→executor), `TicketClient` et `Enqueuer` (poller→prodplanner/scheduler)
- **Temp dirs** pour I/O container (prompt.md, output/) — nettoyés après collecte
- **`embed.FS`** pour les templates — binaire auto-suffisant
- **Naming containers** : `autodev-{TICKET_NUMBER}` (identifiable via `docker ps`)
- **Workspaces persistants** : pas de re-clone, `git checkout main && git pull` à chaque run
- **Parallélisme** : semaphore global (`max_concurrent`) + verrou par projet (séquentiel intra-projet)
- **SQLite** : WAL mode, pure Go driver (`modernc.org/sqlite`), pas de CGO
