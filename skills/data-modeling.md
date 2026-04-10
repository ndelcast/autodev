# Skill : Data Modeling (Laravel)

## Migrations
- Always use `timestamps()` and `softDeletes()` if asked
- Add indexes on foreign keys: `$table->foreignId('company_id')->constrained()->cascadeOnDelete()`
- Use `string` for short text, `text` for long content
- Use `enum` via string column + PHP BackedEnum (not DB enum)

## Factories
- Use Faker with `fr_FR` locale
- Define `states()` for each enum value
- Realistic data: names, addresses, phone numbers, emails

## Seeders
- Realistic quantities: 30-50 per main entity
- Use factory `count()` and `has()` for relations
- Call in `DatabaseSeeder` in dependency order

## Models
- Explicit `$fillable` array
- `$casts` for dates, booleans, enums, JSON
- Typed relations with return types
- Scopes as `scopeActive()`, `scopeByCompany()` etc.
