#!/bin/bash
# scripts/init.sh — Initialise le template pour un nouveau projet

set -e

echo ""
echo "=== Initialisation du projet ==="
echo ""

# Demander les infos du projet
read -p "Nom du projet (ex: Mon Projet): " PROJECT_NAME
read -p "Prefixe tickets (ex: JSMV): " PROJECT_PREFIX
read -p "ID ProdPlanner du projet (ex: 12): " PRODPLANNER_ID

if [ -z "$PROJECT_NAME" ] || [ -z "$PROJECT_PREFIX" ] || [ -z "$PRODPLANNER_ID" ]; then
    echo "Erreur: tous les champs sont obligatoires."
    exit 1
fi

# Generer le slug (lowercase, tirets)
PROJECT_SLUG=$(echo "$PROJECT_NAME" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd '[:alnum:]-')

echo ""
echo "Configuration:"
echo "  Nom:        $PROJECT_NAME"
echo "  Prefixe:    $PROJECT_PREFIX"
echo "  Slug:       $PROJECT_SLUG"
echo "  ProdPlanner: $PRODPLANNER_ID"
echo ""
read -p "Confirmer ? (y/N): " CONFIRM

if [ "$CONFIRM" != "y" ] && [ "$CONFIRM" != "Y" ]; then
    echo "Annule."
    exit 0
fi

# Mettre a jour CLAUDE.md
sed -i '' "s/\[PROJECT_NAME\]/$PROJECT_NAME/g" CLAUDE.md
sed -i '' "s/\[PREFIX\]/$PROJECT_PREFIX/g" CLAUDE.md
sed -i '' "s/\[PRODPLANNER_ID\]/$PRODPLANNER_ID/g" CLAUDE.md

# Mettre a jour composer.json (nom du package)
sed -i '' "s/\"name\": \"laravel\/laravel\"/\"name\": \"outlined\/$PROJECT_SLUG\"/g" composer.json

# Mettre a jour Makefile (titre)
sed -i '' "s/# Template Laravel Filament/# $PROJECT_NAME/g" Makefile

# Supprimer la commande init du Makefile
sed -i '' '/^init:/,/^$/d' Makefile
sed -i '' 's/ init//' Makefile
# Supprimer la ligne d'aide init
sed -i '' '/make init/d' Makefile

# Supprimer le script d'init et le dossier scripts/ s'il est vide
rm -f scripts/init.sh
rmdir scripts 2>/dev/null || true

echo ""
echo "Projet '$PROJECT_NAME' initialise !"

