# Brief AutoDev — POC

## Vision

**AutoDev** est un développeur IA pour Outlined SAS. Il fonctionne comme un membre de l'équipe : il prend des tickets depuis le board ProdPlanner, génère le code via Claude Code, et crée une PR sur GitHub.

C'est un service **Go** qui tourne sur le VPS. Pour chaque ticket, il lance un **container Docker éphémère** adapté à la stack du projet (PHP, Node, Python...) dans lequel Claude Code s'exécute avec tous les outils nécessaires.

## Flow

```
ProdPlanner                                  AutoDev (Go sur VPS)              GitHub
───────────                                  ────────────────────              ──────

1. Story Map
   └── Convertir story → ticket
       └── Placer sur le board
           └── Assigner à "AutoDev"

                                    2. Polling : détecte le ticket
                                       assigné à AutoDev

                                    3. Lit le ticket (titre, description,
                                       critères d'acceptation)

                                    4. Construit le prompt
                                       (context.md + skills + ticket)

                                    5. Lance un container Docker éphémère
                                       avec l'image de la stack du projet
                                       ┌──────────────────────────────┐
                                       │  Container éphémère          │
                                       │  image: autodev-laravel      │
                                       │                              │
                                       │  git checkout -b feat/...    │
                                       │  claude -p "{prompt}"        │
                                       │  git commit + push           │
                                       │  gh pr create         ──────────►  PR créée
                                       └──────────────────────────────┘

                                    6. Met à jour le ticket            ──►  Déplace dans
                                       dans ProdPlanner                     "À déployer"
```

L'utilisateur review la PR sur GitHub, merge, et le CI déploie.

## Stack technique

### AutoDev — Orchestrateur (Go)
- **Go 1.23+** — binaire unique, tourne sur le VPS
- **SQLite** — stockage local (2 tables)
- **net/http** + **html/template** + **HTMX** + **Tailwind CSS** — dashboard server-rendered
- **Docker SDK (docker client)** — lance les containers éphémères
- Pas de framework : stdlib Go + SQLite driver

### Containers d'exécution (Docker)
Chaque projet déclare son **image Docker**. Le container est éphémère : il démarre, exécute le flow complet, et meurt.

Le container a accès à :
- Le workspace du repo (volume monté)
- La clé API Anthropic (env var)
- Le token GitHub (env var)
- Le prompt (passé en argument)

### Images Docker par stack

**`autodev-laravel`** (majorité des projets Outlined) :
- PHP 8.4, Composer 2
- Node 22, npm
- Git, gh (GitHub CLI)
- Claude Code CLI (`@anthropic-ai/claude-code`)
- SQLite (pour `php artisan test`)
- Script d'entrée : `autodev-exec`

**`autodev-node`** (projets frontend / API Node) :
- Node 22, npm
- Git, gh
- Claude Code CLI

**`autodev-base`** (minimal, pour les projets sans stack spécifique) :
- Git, gh
- Claude Code CLI

Chaque image contient un script **`autodev-exec`** qui enchaîne les étapes :
```bash
#!/bin/sh
set -e

# Args passés par le Go orchestrateur
BRANCH_NAME=$1
TICKET_NUMBER=$2
TICKET_TITLE=$3
PROMPT_FILE=$4

cd /workspace

# Git prep
git checkout main && git pull origin main
git checkout -b "$BRANCH_NAME"

# Claude Code
claude -p "$(cat $PROMPT_FILE)" \
  --model "$CLAUDE_MODEL" \
  --max-turns "$CLAUDE_MAX_TURNS" \
  --allowedTools bash,write,read,edit \
  --output-format json \
  > /output/claude_result.json 2>&1

# Commit + Push
git add -A
git commit -m "feat($TICKET_NUMBER): $TICKET_TITLE"
git push -u origin "$BRANCH_NAME"

# Create PR
gh pr create \
  --title "feat($TICKET_NUMBER): $TICKET_TITLE" \
  --body-file /output/pr_body.md \
  > /output/pr_url.txt
```

Le Go orchestrateur récupère les résultats via les fichiers dans `/output/` (volume monté).

## Intégration ProdPlanner

### API utilisée

| Action | Endpoint ProdPlanner | Quand |
|--------|---------------------|-------|
| Lister les tickets assignés | `GET /api/tickets?assigned_to={id}` | Polling régulier |
| Lire un ticket | `GET /api/tickets/{id}` | Au démarrage d'une génération |
| Déplacer un ticket | `POST /api/tickets/{id}/move` | Après génération (→ "À déployer") |
| Compléter un ticket | `POST /api/tickets/{id}/complete` | Après PR créée |
| Lire le projet | `GET /api/projects/{id}` | Pour le contexte |

### Identification d'AutoDev

AutoDev est enregistré comme un **developer** dans ProdPlanner (via le système existant d'`assigned_to`). Quand un ticket est assigné à ce developer_id, AutoDev le prend en charge.

### Données d'un ticket ProdPlanner

```json
{
  "id": 397,
  "formatted_number": "DISP-385",
  "type": "feat",
  "title": "Télécharger les icônes de services",
  "description": "**En tant que** prestataire, je peux...\n\n## Critères d'acceptation\n- ...\n\n## Tâches techniques\n- ...\n\n## Fichiers\n- ...",
  "priority": "medium",
  "size": "xs",
  "assigned_to": 3,
  "board_column": { "id": 41, "name": "À Faire" },
  "project": { "id": 13, "name": "Dispoo", "ticket_prefix": "DISP" }
}
```

## Contexte projet

Chaque projet a un fichier **`project-context.md`** qui donne à Claude le contexte nécessaire :

```markdown
# Dispoo — Contexte projet

## Description
SaaS de prise de rendez-vous en ligne pour prestataires de services.

## Stack
- Laravel 12, Filament 3 (admin + app panels), Livewire 3
- MySQL, Redis, Laravel Reverb
- Stripe pour les paiements

## Architecture
- Multi-tenant : chaque prestataire a son propre espace
- Panel admin : /admin (backoffice Outlined)
- Panel app : /app (espace prestataire)
- Widget public : /booking/{slug} (réservation client)

## Conventions
- Models dans app/Models/
- Enums dans app/Enums/ (BackedEnum string)
- Policies dans app/Policies/
- Filament Resources dans app/Filament/{Panel}/Resources/
- Tests Pest dans tests/Feature/ et tests/Unit/

## Notes importantes
- Le prestataire = User avec rôle "provider"
- Les services ont un champ `emoji` pour l'icône
- Le widget de réservation est en Livewire, pas Filament
```

Ce fichier est écrit et maintenu manuellement. Il est injecté dans chaque prompt en plus du ticket.

## Architecture Go

```
autodev/
├── main.go                         # Point d'entrée, CLI (serve, run, retry)
├── go.mod
├── go.sum
│
├── config/
│   └── config.go                   # Chargement config YAML + env vars
│
├── internal/
│   ├── prodplanner/
│   │   └── client.go               # Client HTTP API ProdPlanner (OAuth)
│   │
│   ├── poller/
│   │   └── poller.go               # Goroutine de polling, détecte les tickets
│   │
│   ├── scheduler/
│   │   └── scheduler.go            # File d'attente + ordonnancement parallèle
│   │
│   ├── executor/
│   │   ├── executor.go             # Orchestre : prep → docker run → collect results → update
│   │   ├── docker.go               # Lance le container éphémère via Docker SDK
│   │   └── prompt.go               # Assemble le prompt : context + skills + ticket
│   │
│   ├── store/
│   │   ├── db.go                   # Init SQLite, migrations auto
│   │   ├── project.go              # CRUD projects
│   │   └── generation.go           # CRUD generations
│   │
│   └── web/
│       ├── server.go               # HTTP server setup, routes
│       ├── handlers.go             # Handlers : list projects, detail, retry, poll
│       └── templates/
│           ├── layout.html         # Base layout (Tailwind + HTMX)
│           ├── projects.html       # Liste des projets
│           └── project_detail.html # Détail + générations
│
├── skills/                         # Skills injectés dans le prompt
│   ├── laravel.md
│   ├── inertia-vue.md
│   └── data-modeling.md
│
├── contexts/                       # Contexte projet (1 fichier par projet)
│   ├── dispoo.md
│   └── crm-granit.md
│
├── images/                         # Dockerfiles des images d'exécution
│   ├── laravel/
│   │   ├── Dockerfile
│   │   └── autodev-exec.sh
│   ├── node/
│   │   ├── Dockerfile
│   │   └── autodev-exec.sh
│   └── base/
│       ├── Dockerfile
│       └── autodev-exec.sh
│
├── Dockerfile                      # Image de l'orchestrateur Go
└── config.yaml
```

### Packages et responsabilités

#### `config` — Configuration

```go
type Config struct {
    ProdPlanner ProdPlannerConfig
    Claude      ClaudeConfig
    Polling     PollingConfig
    Docker      DockerConfig
    WebPort     int
    DBPath      string
    SkillsDir   string
    ContextsDir string
    WorkspaceBasePath string  // racine des workspaces clonés
}

type ClaudeConfig struct {
    Model    string        // "sonnet"
    MaxTurns int           // 30
    Timeout  time.Duration // 10m (timeout du container)
}

type DockerConfig struct {
    NetworkMode   string // "host" ou custom
    MaxConcurrent int    // nombre max de containers simultanés (défaut: 3)
}
```

Chargé depuis `config.yaml` + variables d'environnement (env vars prioritaires).

#### `internal/prodplanner` — Client API

```go
type Client struct {
    baseURL    string
    httpClient *http.Client
    token      string
    tokenExp   time.Time
}

func (c *Client) ListTicketsAssignedTo(developerID int) ([]Ticket, error)
func (c *Client) GetTicket(ticketID int) (*Ticket, error)
func (c *Client) MoveTicket(ticketID int, columnID int) error
func (c *Client) CompleteTicket(ticketID int) error
func (c *Client) GetProject(projectID int) (*Project, error)
```

Auth OAuth2 client credentials. Le token est caché et refreshé automatiquement.

#### `internal/poller` — Polling

```go
type Poller struct {
    store    *store.Store
    pp       *prodplanner.Client
    exec     *executor.Executor
    interval time.Duration
}

func (p *Poller) Start(ctx context.Context)
```

Goroutine qui tourne en boucle :
1. Pour chaque projet en base, appelle `ListTicketsAssignedTo`
2. Filtre les tickets déjà traités (check en base par `prodplanner_ticket_id`)
3. Crée une Generation `queued`
4. Dispatch vers le scheduler

#### `internal/scheduler` — Ordonnancement parallèle

```go
type Scheduler struct {
    store    *store.Store
    exec     *executor.Executor
    sem      chan struct{}            // semaphore global (max_concurrent)
    mu       sync.Mutex
    running  map[int64]struct{}       // project IDs en cours d'exécution
    queue    []*store.Generation      // file d'attente FIFO
}

func (s *Scheduler) Enqueue(gen *store.Generation)
func (s *Scheduler) Start(ctx context.Context)
```

Règles d'ordonnancement :
- **1 container max par projet** — évite les conflits Git sur le même workspace
- **N containers en parallèle** sur des projets différents (limité par `max_concurrent`)
- Les tickets d'un même projet sont traités séquentiellement (FIFO)
- Un **semaphore global** (`chan struct{}` de taille `max_concurrent`) limite la charge

```
Exemple avec max_concurrent=3 :

t0: DISP-385 (Dispoo) ████████████████░░░░░░░░
    CRM-12   (CRM)    ████████░░░░░░░░░░░░░░░░
    BLOG-5   (Blog)   ██████████████░░░░░░░░░░

t1: DISP-386 (Dispoo) en attente (Dispoo occupé)
    → démarre quand DISP-385 termine

t2: DISP-386 (Dispoo) ████████████████████░░░░
    CRM-13   (CRM)    ██████████░░░░░░░░░░░░░░
    BLOG-6   (Blog)   en attente (slot libre quand CRM-13 ou DISP-386 termine)
```

#### `internal/executor` — Exécution via Docker

Le coeur du système. Orchestre l'exécution d'un ticket dans un container éphémère.

```go
type Executor struct {
    store         *store.Store
    pp            *prodplanner.Client
    docker        *client.Client  // Docker SDK
    cfg           *config.Config
}

func (e *Executor) Process(ctx context.Context, gen *store.Generation) error
```

**Étapes de `Process()` :**

**1. Préparer le workspace**
```go
// S'assurer que le repo est cloné
workspacePath := filepath.Join(e.cfg.WorkspaceBasePath, project.Slug)
if !exists(workspacePath) {
    exec.Command("git", "clone", repoURL, workspacePath).Run()
}
```

**2. Construire le prompt**
```go
prompt, err := e.buildPrompt(project, ticket)
// Écrit le prompt dans un fichier temporaire pour le passer au container
promptFile := filepath.Join(tmpDir, "prompt.md")
os.WriteFile(promptFile, []byte(prompt), 0644)

// Prépare le body de la PR
prBody := fmt.Sprintf("%s\n\n---\nGénéré par AutoDev", ticket.Description)
os.WriteFile(filepath.Join(tmpDir, "pr_body.md"), []byte(prBody), 0644)
```

**3. Lancer le container Docker éphémère**
```go
resp, err := e.docker.ContainerCreate(ctx, &container.Config{
    Image: project.DockerImage,  // "autodev-laravel:latest"
    Cmd: []string{
        "/usr/local/bin/autodev-exec",
        branchName,       // "feat/DISP-385/telecharger-icones"
        ticket.Number,    // "DISP-385"
        ticket.Title,     // "Télécharger les icônes de services"
        "/prompt/prompt.md",
    },
    Env: []string{
        "ANTHROPIC_API_KEY=" + os.Getenv("ANTHROPIC_API_KEY"),
        "GITHUB_TOKEN=" + os.Getenv("GITHUB_TOKEN"),
        "CLAUDE_MODEL=" + e.cfg.Claude.Model,
        "CLAUDE_MAX_TURNS=" + strconv.Itoa(e.cfg.Claude.MaxTurns),
    },
}, &container.HostConfig{
    Binds: []string{
        workspacePath + ":/workspace",       // repo monté
        tmpDir + "/prompt.md:/prompt/prompt.md:ro",  // prompt
        tmpDir + "/pr_body.md:/output/pr_body.md:ro",
        outputDir + ":/output",              // résultats
    },
    NetworkMode: container.NetworkMode(e.cfg.Docker.NetworkMode),
}, nil, nil, "autodev-"+gen.TicketNumber)

e.docker.ContainerStart(ctx, resp.ID, container.StartOptions{})
```

**4. Attendre la fin + timeout**
```go
statusCh, errCh := e.docker.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
select {
case err := <-errCh:
    // erreur Docker
case status := <-statusCh:
    // status.StatusCode == 0 → succès
}
// Cleanup
defer e.docker.ContainerRemove(ctx, resp.ID, container.RemoveOptions{})
```

**5. Collecter les résultats**
```go
// Lire les fichiers de sortie du container
claudeResult, _ := os.ReadFile(filepath.Join(outputDir, "claude_result.json"))
prURL, _ := os.ReadFile(filepath.Join(outputDir, "pr_url.txt"))
```

**6. Mettre à jour ProdPlanner**
```go
e.pp.MoveTicket(ticket.ID, project.DoneColumnID)
```

**7. Sauvegarder la génération en base**
```go
gen.Status = "completed"
gen.PRUrl = strings.TrimSpace(string(prURL))
gen.ClaudeOutput = string(claudeResult)
gen.BranchName = branchName
gen.DurationSeconds = int(time.Since(startedAt).Seconds())
gen.CompletedAt = timePtr(time.Now())
e.store.UpdateGeneration(gen)
```

#### `internal/store` — Persistance SQLite

```go
type Project struct {
    ID                    int64
    ProdPlannerProjectID  int
    Name                  string
    Slug                  string
    GithubRepo            string    // "outlined/dispoo"
    DockerImage           string    // "autodev-laravel:latest"
    ContextFile           string    // "dispoo.md"
    Skills                []string  // ["laravel", "data-modeling"]
    AutodevDeveloperID    int
    DoneColumnID          int
    Status                string    // "idle", "running"
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

type Generation struct {
    ID                    int64
    ProjectID             int64
    ProdPlannerTicketID   int
    TicketNumber          string    // "DISP-385"
    TicketTitle           string
    TicketDescription     string
    Status                string    // "queued", "running", "completed", "failed"
    BranchName            string
    PRUrl                 string
    PromptSent            string
    ClaudeOutput          string
    ErrorMessage          string
    DurationSeconds       int
    Attempt               int
    StartedAt             *time.Time
    CompletedAt           *time.Time
    CreatedAt             time.Time
    UpdatedAt             time.Time
}
```

#### `internal/web` — Dashboard HTTP

Server-rendered avec `html/template` + HTMX + Tailwind CSS (CDN).

```go
func (s *Server) routes() {
    s.mux.HandleFunc("GET /", s.handleProjects)
    s.mux.HandleFunc("GET /projects/{id}", s.handleProjectDetail)
    s.mux.HandleFunc("POST /projects/{id}/poll", s.handleForcePoll)
    s.mux.HandleFunc("POST /generations/{id}/retry", s.handleRetry)
    s.mux.HandleFunc("GET /api/generations/{id}/logs", s.handleGenerationLogs)
}
```

HTMX pour :
- Polling automatique du statut (refresh partiel toutes les 5s)
- Expand/collapse des logs
- Actions (retry, force poll) sans navigation

### `main.go` — Point d'entrée

```go
func main() {
    cfg := config.Load()
    db := store.New(cfg.DBPath)
    ppClient := prodplanner.NewClient(cfg.ProdPlanner)
    dockerClient, _ := client.NewClientWithOpts(client.FromEnv)
    exec := executor.New(db, ppClient, dockerClient, cfg)
    sched := scheduler.New(db, exec, cfg.Docker.MaxConcurrent)
    poll := poller.New(db, ppClient, sched, cfg.Polling.Interval)
    srv := web.NewServer(db, poll, sched, cfg.WebPort)

    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()

    switch os.Args[1] {
    case "serve":
        go sched.Start(ctx)
        go poll.Start(ctx)
        srv.ListenAndServe(ctx)
    case "run":
        // one-shot: traiter un ticket
    case "retry":
        // relancer une génération
    case "projects":
        // lister les projets
    case "generations":
        // lister les générations
    }
}
```

## CLI

```bash
# Lancer le daemon (poller + dashboard)
autodev serve

# Traiter un ticket manuellement
autodev run --project=1 --ticket=397

# Relancer une génération échouée
autodev retry --generation=42

# Lister les projets configurés
autodev projects

# Lister les générations d'un projet
autodev generations --project=1
```

## Images Docker d'exécution

### `images/laravel/Dockerfile`

```dockerfile
FROM php:8.4-cli-alpine

# PHP extensions
RUN apk add --no-cache \
    git nodejs npm curl sqlite-dev \
    && docker-php-ext-install pdo_sqlite

# Composer
COPY --from=composer:2 /usr/bin/composer /usr/local/bin/composer

# GitHub CLI
RUN apk add --no-cache github-cli

# Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Script d'exécution
COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

WORKDIR /workspace
```

### `images/node/Dockerfile`

```dockerfile
FROM node:22-alpine

RUN apk add --no-cache git github-cli
RUN npm install -g @anthropic-ai/claude-code

COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

WORKDIR /workspace
```

### `images/base/Dockerfile`

```dockerfile
FROM alpine:3.20

RUN apk add --no-cache git nodejs npm github-cli
RUN npm install -g @anthropic-ai/claude-code

COPY autodev-exec.sh /usr/local/bin/autodev-exec
RUN chmod +x /usr/local/bin/autodev-exec

WORKDIR /workspace
```

### Script `autodev-exec.sh` (commun, adapté par image)

```bash
#!/bin/sh
set -e

BRANCH_NAME="$1"
TICKET_NUMBER="$2"
TICKET_TITLE="$3"
PROMPT_FILE="$4"

cd /workspace

# 1. Git prep
git config user.name "AutoDev"
git config user.email "autodev@outlined.io"
git checkout main
git pull origin main
git checkout -b "$BRANCH_NAME"

# 2. Claude Code
claude -p "$(cat "$PROMPT_FILE")" \
  --model "${CLAUDE_MODEL:-sonnet}" \
  --max-turns "${CLAUDE_MAX_TURNS:-30}" \
  --allowedTools bash,write,read,edit \
  --output-format json \
  > /output/claude_result.json 2>/output/claude_stderr.log

# 3. Commit + Push (seulement s'il y a des changements)
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "feat($TICKET_NUMBER): $TICKET_TITLE"
  git push -u origin "$BRANCH_NAME"
else
  echo "No changes detected" > /output/error.txt
  exit 1
fi

# 4. Create PR
gh pr create \
  --title "feat($TICKET_NUMBER): $TICKET_TITLE" \
  --body-file /output/pr_body.md \
  > /output/pr_url.txt 2>&1
```

## Skills

Fichiers Markdown dans `skills/`, injectés dans le prompt.

### `skills/laravel.md`
- Structure Laravel 12 (middleware dans bootstrap/app.php)
- Naming : modèles PascalCase singulier, tables snake_case pluriel
- Enums PHP 8.1+ BackedEnum string
- FormRequests pour les validations
- Code propre, pas de commentaires inutiles

### `skills/inertia-vue.md`
- Inertia.js v3 : controllers retournent `Inertia::render()`
- Vue 3 Composition API (`<script setup>`)
- PrimeVue : DataTable, InputText, Button, Dialog, Toast...
- Tailwind CSS v4
- Pages dans `resources/js/Pages/`, composants dans `resources/js/Components/`

### `skills/data-modeling.md`
- Migrations : timestamps(), softDeletes() si demandé, index sur FK
- Factories : Faker fr_FR, states pour chaque enum
- Seeders : quantités réalistes (30-50 par entité)
- Modèles : $fillable explicite, $casts, relations typées

## Configuration

```yaml
# config.yaml
prodplanner:
  base_url: "https://prodplanner.outlined.dev/api"
  client_id: "${PRODPLANNER_CLIENT_ID}"
  client_secret: "${PRODPLANNER_CLIENT_SECRET}"

claude:
  model: "sonnet"
  max_turns: 30
  timeout: "10m"  # timeout du container

polling:
  interval: "60s"

docker:
  network_mode: "host"
  max_concurrent: 3  # nombre max de containers simultanés

workspace_base_path: "./workspaces"
web_port: 8080
db_path: "./autodev.db"
skills_dir: "./skills"
contexts_dir: "./contexts"

# Projets configurés (alternative à la DB pour le POC)
projects:
  - name: "Dispoo"
    slug: "dispoo"
    prodplanner_project_id: 13
    github_repo: "outlined/dispoo"
    docker_image: "autodev-laravel:latest"
    context_file: "dispoo.md"
    skills: ["laravel", "data-modeling"]
    autodev_developer_id: 3
    done_column_id: 43
```

## Dockerfile de l'orchestrateur Go

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o autodev .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates docker-cli
COPY --from=builder /app/autodev /usr/local/bin/autodev
COPY skills/ /app/skills/
COPY contexts/ /app/contexts/
COPY config.yaml /app/config.yaml

WORKDIR /app
EXPOSE 8080
CMD ["autodev", "serve"]
```

Note : l'orchestrateur Go n'a besoin que de `docker-cli` (pour communiquer avec le Docker daemon du host via le socket monté). Pas besoin de PHP, Node, etc.

## Phases de développement

### Phase 1 — Core + CLI
- `config/` — chargement config YAML + env
- `internal/store/` — SQLite, models, migrations auto
- `internal/prodplanner/` — client HTTP avec OAuth
- `internal/executor/` — prompt builder + docker launcher
- `images/laravel/` — Dockerfile + autodev-exec.sh
- `main.go` — commandes `run` et `retry`
- Skills .md + un contexte de test
- Tests Go (`_test.go`)

### Phase 2 — Polling + Scheduler
- `internal/poller/` — goroutine de polling
- `internal/scheduler/` — ordonnancement parallèle (1 par projet, N global)
- Commande `serve` (poller + scheduler)
- Gestion du shutdown graceful (drain des containers en cours)
- Images Docker additionnelles (node, base)

### Phase 3 — Dashboard
- `internal/web/` — serveur HTTP, templates, HTMX
- Pages : projets, détail, logs
- Actions : force poll, retry
- Commande `serve` complète (poller + web)
- Dockerfile orchestrateur

## Contraintes techniques

- **Pas de framework web** — `net/http` standard (Go 1.22+ routing patterns)
- **Pas de ORM** — `database/sql` + `modernc.org/sqlite` (pure Go, pas de CGO)
- **Docker SDK** — `github.com/docker/docker/client` pour lancer les containers
- **Context partout** — toutes les opérations longues acceptent un `context.Context`
- **Structured logging** — `log/slog` (stdlib Go 1.21+)
- **Erreurs wrappées** — `fmt.Errorf("doing X: %w", err)`
- **Code en anglais**
- **UI du dashboard en français**
- **Parallélisme contrôlé** — 1 container max par projet (évite conflits Git), N containers en parallèle sur projets différents, limité par `max_concurrent`
- **Container éphémère** — créé et détruit pour chaque ticket, pas de state résiduel

## Test du POC

Pour valider :
1. Builder l'image : `docker build -t autodev-laravel images/laravel/`
2. Configurer un projet dans `config.yaml` (Dispoo)
3. Créer un ticket dans ProdPlanner, l'assigner au developer_id d'AutoDev
4. Lancer `autodev run --project=dispoo --ticket=397`
5. Vérifier que le container a tourné et est détruit
6. Vérifier qu'une PR est créée sur GitHub
7. Vérifier que le ticket est déplacé dans "À déployer" sur ProdPlanner
8. Vérifier que la génération est en base SQLite avec statut `completed`
