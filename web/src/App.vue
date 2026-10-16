<script setup>
import { RouterLink, RouterView } from 'vue-router'
import SiteFooter from './components/SiteFooter.vue'
import VerifyBanner from './components/VerifyBanner.vue'
import { accountsEnabled, currentUser } from './auth.js'
</script>

<template>
  <VerifyBanner v-if="accountsEnabled && currentUser && !currentUser.email_verified_at" />
  <header class="topbar">
    <RouterLink to="/" class="brand">
      <img src="/favicon.svg" alt="" width="24" height="24" />
      <span>Canvasbin</span>
    </RouterLink>
    <nav class="topnav">
      <template v-if="!accountsEnabled">
        <RouterLink to="/" class="btn ghost small">New page</RouterLink>
      </template>
      <template v-else-if="currentUser">
        <RouterLink to="/" class="btn ghost small">Pages</RouterLink>
        <RouterLink to="/account" class="btn small" :title="currentUser.email">
          <span class="topnav-email">{{ currentUser.display_name || currentUser.email }}</span>
        </RouterLink>
      </template>
      <template v-else>
        <RouterLink to="/features" class="topnav-link">Features</RouterLink>
        <RouterLink to="/use-cases" class="topnav-link">Use cases</RouterLink>
        <RouterLink to="/pricing" class="topnav-link">Pricing</RouterLink>
        <RouterLink to="/login" class="btn ghost small">Sign in</RouterLink>
        <RouterLink to="/register" class="btn primary small">Get started</RouterLink>
      </template>
    </nav>
  </header>
  <main class="shell">
    <RouterView :key="$route.fullPath" />
  </main>
  <SiteFooter v-if="accountsEnabled" />
</template>
