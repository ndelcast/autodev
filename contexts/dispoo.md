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
