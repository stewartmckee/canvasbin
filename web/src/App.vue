<script setup>
import { RouterLink, RouterView } from 'vue-router'
</script>

<template>
  <header class="topbar">
    <RouterLink to="/" class="brand">
      <img src="/favicon.svg" alt="" width="24" height="24" />
      <span>Canvasbin</span>
    </RouterLink>
    <RouterLink to="/" class="btn ghost small">New page</RouterLink>
  </header>
  <main class="shell">
    <RouterView :key="$route.fullPath" />
  </main>
</template>
