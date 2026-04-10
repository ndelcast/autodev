# Skill : Inertia.js + Vue 3 + PrimeVue

## Inertia.js v3
- Controllers return `Inertia::render('Page/Name', ['prop' => $value])`
- Shared data via `HandleInertiaRequests` middleware
- Use `router.visit()`, `router.post()` for navigation
- Use `useForm()` for form handling

## Vue 3
- Composition API with `<script setup>`
- Props defined with `defineProps<{...}>()`
- Use `ref()`, `computed()`, `watch()`
- Pages in `resources/js/Pages/`
- Reusable components in `resources/js/Components/`

## PrimeVue
- Use PrimeVue components: DataTable, Column, InputText, Button, Dialog, Toast, Select, Tag
- Aura theme
- Use PrimeVue's built-in icons via `PrimeIcons`

## Tailwind CSS v4
- Utility-first classes
- Use `@apply` sparingly
- Responsive: mobile-first (`sm:`, `md:`, `lg:`)
