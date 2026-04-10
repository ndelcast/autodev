<script setup>
import { Head } from '@inertiajs/vue3';
import Button from 'primevue/button';

defineProps({
    laravelVersion: String,
    phpVersion: String,
});
</script>

<template>
    <Head title="Accueil" />

    <div class="min-h-screen flex items-center justify-center bg-gray-50">
        <div class="text-center space-y-6">
            <h1 class="text-4xl font-bold text-gray-900">Bienvenue</h1>
            <p class="text-gray-600">
                Laravel {{ laravelVersion }} &middot; PHP {{ phpVersion }}
            </p>
            <div class="flex gap-3 justify-center">
                <Button label="Commencer" icon="pi pi-arrow-right" />
            </div>
        </div>
    </div>
</template>
