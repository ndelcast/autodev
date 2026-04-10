# Template Laravel + Filament — Outlined

Template de base pour les projets Laravel / Filament d'Outlined.

## Stack

- **Laravel 12** + **Filament v4** (admin panel sur `/admin`)
- **Livewire 3** + **Tailwind CSS v4**
- **MySQL** via Laravel Sail (Docker)
- **Laravel Pint** (code style)
- **Laravel Boost** + **ProdPlanner** (MCP pour Claude Code)

## Utilisation

### 1. Cloner le template

```bash
git clone git@github.com:outlined-agence/laravel-filament-template.git mon-projet
cd mon-projet
rm -rf .git
```

### 2. Installer les dependances

```bash
composer install
npm install
```

### 3. Initialiser le projet

```bash
make init
```

Le script demande :
- **Nom du projet** (ex: `Mon Projet`) — mis a jour dans CLAUDE.md, Makefile
- **Prefixe tickets** (ex: `MP`) — utilise pour les branches git (`feat/MP-XXX/...`)
- **ID ProdPlanner** (ex: `12`) — reference dans CLAUDE.md

Une fois termine, le script se supprime automatiquement.

### 4. Configurer l'environnement

```bash
cp .env.example .env
php artisan key:generate
```

Renseigner les variables ProdPlanner dans `.env` :
```
PRODPLANNER_CLIENT_ID=xxx
PRODPLANNER_CLIENT_SECRET=xxx
```

### 5. Lancer le projet

```bash
vendor/bin/sail up -d
vendor/bin/sail artisan migrate
```

Le panel admin est accessible sur `http://localhost/admin`.

### 6. Premier commit

```bash
git init
git add .
git commit -m "Initial commit"
git remote add origin git@github.com:outlined-agence/mon-projet.git
git push -u origin main
```

## Commandes disponibles

| Commande | Description |
|----------|-------------|
| `make fresh` | Reset DB + seeders |
| `make seed` | Lancer les seeders |
| `make migrate` | Lancer les migrations |
| `make test` | Lancer les tests |
| `make lint` | Verifier le code style (Pint) |
| `make fix` | Corriger le code style |
| `make clear` | Vider tous les caches |

## Claude Code (MCP)

Le template inclut la config pour deux serveurs MCP :

- **laravel-boost** — introspection Laravel (schema DB, routes, docs)
- **prodplanner** — gestion de projet / tickets

Les permissions Claude Code sont pre-configurees dans `.claude/settings.local.json`.

## Conventions

- **Code en anglais** — variables, methodes, classes, commentaires
- **UI en francais** — labels, messages d'erreur, texte des boutons
- **Branches** — `feat/PREFIX-XXX/description`, `fix/PREFIX-XXX/description`
- **Commits** — `feat(scope): description`
