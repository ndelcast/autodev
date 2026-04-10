# Template Laravel Filament

.PHONY: help init fresh seed migrate test lint fix clear

help:
	@echo "Commandes disponibles:"
	@echo "  make init     - Initialiser le template pour un nouveau projet"
	@echo "  make fresh    - Reset DB + seeders"
	@echo "  make seed     - Lancer les seeders"
	@echo "  make migrate  - Lancer les migrations"
	@echo "  make test     - Lancer les tests"
	@echo "  make lint     - Verifier code style"
	@echo "  make fix      - Corriger code style"
	@echo "  make clear    - Vider les caches"

init:
	@bash scripts/init.sh

fresh:
	vendor/bin/sail artisan migrate:fresh --seed

seed:
	vendor/bin/sail artisan db:seed

migrate:
	vendor/bin/sail artisan migrate

test:
	vendor/bin/sail artisan test --compact

lint:
	vendor/bin/sail bin pint --test

fix:
	vendor/bin/sail bin pint

clear:
	vendor/bin/sail artisan config:clear
	vendor/bin/sail artisan route:clear
	vendor/bin/sail artisan view:clear
	vendor/bin/sail artisan cache:clear
