# LeadsMarket (Hygilead)

> Marketplace B2B de leads avec achat par credits, CRM pipeline, paiement Stripe et import Google Sheets.

## Stack
- Laravel 12, PHP 8.4 | Filament v4 (admin) | Inertia.js + Vue 3 + PrimeVue 4 (client)
- Tailwind CSS v4 (dark mode force) | Stripe (paiement) | Google Sheets API (import)
- MySQL (Laravel Sail) | Queue (ShouldQueue)

## Conventions
- Code en anglais, UI en francais
- Enums PHP 8.1+ BackedEnum string dans app/Enums/
- FormRequests pour la validation (jamais inline)
- Models : $fillable explicite, $casts pour dates/enums/JSON, relations typees
- Frontend : Composition API avec script setup, PrimeVue components, Aura theme
- Marque Hygilead : fond #0a0a0a, texte blanc

## Logique metier cle
- Achat de lead : transaction DB avec row-locking (lockForUpdate), verifie dispo + solde, debite credits
- Prix lead configurable via Setting (default 3 credits)
- Departement auto-derive du code postal (DOM-TOM 97x, Corse 20, metro 2 digits)
- Import Google Sheets : mapping colonnes flexible FR/EN, dedup email OU phone, curseur last_row_imported
- Pipeline CRM : 7 statuts (nouveau_lead a archive), archivage auto via artisan leads:archive
- Stripe : checkout sessions + webhook checkout.session.completed, protection doublon via stripe_session_id
