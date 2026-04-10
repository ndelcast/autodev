# [PROJECT_NAME] Claude Code Instructions

## Git Workflow (OBLIGATOIRE)

### Branch Naming Convention

```
feat/[PREFIX]-XXX/short-description
fix/[PREFIX]-XXX/short-description
refactor/[PREFIX]-XXX/short-description
```

### PR Creation Process

1. **Create branch** from `develop`:
   ```bash
   git checkout develop && git pull
   git checkout -b feat/[PREFIX]-XXX/description
   ```

2. **Commit format**:
   ```
   feat(scope): description

   Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>
   ```

3. **Create PR** with `gh pr create`:
   ```bash
   gh pr create --base develop --title "feat([PREFIX]-XXX): Title" --body "..."
   ```

---

## Tech Stack

- Laravel 12 + Filament v4
- Livewire 3 + Tailwind CSS v4
- MySQL via Laravel Sail

## Code Conventions

- **Code in English** — variables, methods, classes, comments
- **UI in French** — labels, error messages, button text

## Quick Reference

### Run Commands via Sail

```bash
vendor/bin/sail artisan migrate
vendor/bin/sail artisan test --compact
vendor/bin/sail bin pint --dirty
```

### Makefile

```
make fresh / make seed / make migrate / make test / make lint / make fix / make clear
```

---

## ProdPlanner MCP

Project ID: [PRODPLANNER_ID]

If MCP connection fails with "No session" error:
1. Check environment variables: `PRODPLANNER_CLIENT_ID`, `PRODPLANNER_CLIENT_SECRET`
2. These must be exported in shell, not just in `.env`
3. Restart Claude Code to reinitialize connection
