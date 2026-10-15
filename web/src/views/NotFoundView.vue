<script setup>
import { accountsEnabled } from '../auth.js'
</script>

<template>
  <div class="panel empty">
    <h1>Nothing here</h1>
    <p>This address is not a Canvasbin page.</p>
    <RouterLink to="/" class="btn primary">{{ accountsEnabled ? 'Back to home' : 'Create a new page' }}</RouterLink>
  </div>
</template>
