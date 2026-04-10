# Guide de configuration et lancement — AutoDev

## Prérequis

- **Go 1.23+**
- **Docker** (daemon en cours d'exécution)
- **Git**
- Un compte **ProdPlanner** avec des identifiants OAuth2 (client credentials)
- Un **GitHub Token** (`GITHUB_TOKEN`) avec les permissions `repo` pour créer des PRs
- Une clé API **Anthropic** (`ANTHROPIC_API_KEY`) pour Claude Code CLI

---

## 1. Cloner le projet

```bash
git clone git@github.com:outlined/autodev.git
cd autodev
```

## 2. Configurer les variables d'environnement

Créer un fichier `.env` à la racine du projet :

```bash
cp .env.example .env  # ou créer manuellement
```

Contenu du `.env` :

```env
# ProdPlanner OAuth2
PRODPLANNER_CLIENT_ID=votre_client_id
PRODPLANNER_CLIENT_SECRET=votre_client_secret

# Anthropic (utilisé dans les containers Docker)
ANTHROPIC_API_KEY=sk-ant-...

# GitHub (utilisé dans les containers Docker pour créer les PRs)
GITHUB_TOKEN=ghp_...
```

Ces variables sont référencées dans `config.yaml` via la syntaxe `${VAR}` et sont aussi transmises aux containers Docker éphémères.

## 3. Configurer les projets

Éditer `config.yaml` pour ajouter ou modifier les projets :

```yaml
prodplanner:
  base_url: "https://prodplanner.outlined.io/api"
  client_id: "${PRODPLANNER_CLIENT_ID}"
  client_secret: "${PRODPLANNER_CLIENT_SECRET}"

claude:
  model: "sonnet"          # Modèle Claude à utiliser (sonnet, opus, haiku)
  max_turns: 30            # Nombre max de tours de conversation
  timeout: "10m"           # Timeout par génération

polling:
  interval: "60s"          # Intervalle de polling ProdPlanner

docker:
  network_mode: "host"     # Mode réseau des containers
  max_concurrent: 3        # Nombre max de générations en parallèle

web_port: 8080             # Port du dashboard
db_path: "./autodev.db"    # Chemin de la base SQLite

projects:
  - name: "Mon Projet"
    slug: "mon-projet"                    # Identifiant unique (URL-safe)
    prodplanner_project_id: 13            # ID du projet dans ProdPlanner
    github_repo: "org/mon-projet"         # Repo GitHub (owner/repo)
    docker_image: "autodev-laravel:latest" # Image Docker à utiliser
    context_file: "mon-projet.md"         # Fichier de contexte dans contexts/
    skills: ["laravel", "data-modeling"]  # Skills à inclure dans le prompt
    autodev_developer_id: 3              # ID du dev "AutoDev" dans ProdPlanner
    done_column_id: 43                   # ID de la colonne "Terminé" du board
```

### Ajouter un contexte projet

Créer un fichier Markdown dans `contexts/` décrivant le projet :

```bash
# contexts/mon-projet.md
```

Ce fichier sera injecté dans le prompt envoyé à Claude Code pour lui donner le contexte métier.

### Ajouter des skills

Les skills sont des fichiers Markdown dans `skills/` qui décrivent des conventions techniques. Skills disponibles :

- `skills/laravel.md` — Conventions Laravel (Eloquent, migrations, etc.)
- `skills/inertia-vue.md` — Conventions Inertia.js + Vue 3 + PrimeVue
- `skills/data-modeling.md` — Conventions de modélisation de données

Pour ajouter un skill, créer un fichier `skills/nom-du-skill.md` et le référencer dans la config du projet.

---

## 4. Compiler le projet

```bash
make build
```

Vérifier que tout fonctionne :

```bash
make test
```

## 5. Construire les images Docker

Les images Docker sont les environnements d'exécution dans lesquels Claude Code travaille.

```bash
# Toutes les images
make docker-all

# Ou individuellement
make docker-laravel   # PHP 8.4 + Composer + Claude Code
make docker-node      # Node 22 + Claude Code
make docker-base      # Alpine minimal + Claude Code
```

---

## 6. Lancement

### Mode daemon (polling automatique + dashboard)

```bash
./autodev serve
```

Cela démarre :
- Le **poller** qui interroge ProdPlanner toutes les 60s (configurable)
- Le **scheduler** qui gère la file d'attente et le parallélisme
- Le **dashboard** web sur `http://localhost:8080`

Arrêt propre avec `Ctrl+C` (attend la fin des générations en cours).

### Exécution manuelle d'un ticket

```bash
./autodev run --project=dispoo --ticket=42
```

### Relancer une génération échouée

```bash
./autodev retry --generation=7
```

### Lister les projets configurés

```bash
./autodev projects
```

### Lister les générations d'un projet

```bash
./autodev generations --project=dispoo
```

---

## 7. Lancement via Docker Compose

Pour un déploiement conteneurisé :

```bash
docker compose up -d
```

Cela :
- Compile le binaire dans un container Go
- Lance l'orchestrateur avec accès au Docker socket
- Expose le dashboard sur le port 8080
- Monte les workspaces et la base de données en volumes

Le fichier `.env` est automatiquement chargé.

```bash
# Voir les logs
docker compose logs -f

# Arrêter
docker compose down
```

---

## Variables d'environnement

| Variable | Description | Requis |
|---|---|---|
| `PRODPLANNER_CLIENT_ID` | ID client OAuth2 ProdPlanner | Oui |
| `PRODPLANNER_CLIENT_SECRET` | Secret client OAuth2 ProdPlanner | Oui |
| `ANTHROPIC_API_KEY` | Clé API Anthropic (pour Claude Code dans les containers) | Oui |
| `GITHUB_TOKEN` | Token GitHub avec permissions `repo` | Oui |
| `AUTODEV_CONFIG` | Chemin vers le fichier de config (défaut: `config.yaml`) | Non |
| `LOG_LEVEL` | Niveau de log : `info` ou `debug` (défaut: `info`) | Non |

---

## Fonctionnement

1. Le **poller** interroge ProdPlanner pour chaque projet configuré et récupère les tickets assignés au développeur AutoDev
2. Les nouveaux tickets sont transformés en **générations** (statut `queued`) et ajoutés à la file du scheduler
3. Le **scheduler** respecte deux contraintes :
   - **Parallélisme global** : max `max_concurrent` générations simultanées
   - **Séquentialité par projet** : un seul ticket à la fois par projet (pour éviter les conflits Git)
4. L'**executor** pour chaque génération :
   - Clone ou met à jour le workspace du projet
   - Construit le prompt (contexte + skills + ticket)
   - Lance un container Docker éphémère avec Claude Code CLI
   - Collecte les résultats (branche, PR, logs)
   - Met à jour ProdPlanner (déplace le ticket dans "Terminé")
   - Sauvegarde le résultat en base
5. Le **dashboard** permet de suivre l'avancement en temps réel et de relancer les générations échouées
