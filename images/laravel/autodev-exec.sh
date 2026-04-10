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

# Clean workspace from any previous run
git checkout -- . 2>/dev/null || true
git clean -fd 2>/dev/null || true
git checkout main
git pull origin main

# Delete branch if it already exists (from a failed retry)
git branch -D "$BRANCH_NAME" 2>/dev/null || true
git push origin --delete "$BRANCH_NAME" 2>/dev/null || true
git checkout -b "$BRANCH_NAME"

# 2. Claude Code
claude -p "$(cat "$PROMPT_FILE")" \
  --model "${CLAUDE_MODEL:-sonnet}" \
  --max-turns "${CLAUDE_MAX_TURNS:-30}" \
  --dangerously-skip-permissions \
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
