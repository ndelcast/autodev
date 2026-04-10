# Skill : Laravel

## Structure
- Laravel 12 — middleware registered in `bootstrap/app.php`
- Models in `app/Models/`
- Controllers in `app/Http/Controllers/`
- FormRequests in `app/Http/Requests/`
- Enums in `app/Enums/` (PHP 8.1+ BackedEnum string)
- Policies in `app/Policies/`

## Naming conventions
- Models: PascalCase singular (`Company`, `Contact`)
- Tables: snake_case plural (`companies`, `contacts`)
- Columns: snake_case (`first_name`, `created_at`)
- Relations: camelCase, singular for belongsTo/hasOne, plural for hasMany/belongsToMany

## Best practices
- Use FormRequests for validation, not inline validation
- Use `$fillable` explicitly on models, never `$guarded = []`
- Use `$casts` for dates, enums, and JSON columns
- Use typed relations: `public function company(): BelongsTo`
- Use route model binding
- Clean code: no unnecessary comments, no dead code
- Use `php artisan test` with Pest for testing
