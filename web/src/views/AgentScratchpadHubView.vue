<script setup>
import { RouterLink } from 'vue-router'
import { currentUser } from '../auth.js'
import { scratchpadPages } from '../scratchpad.js'
</script>

<template>
  <div class="marketing usecase">
    <section class="hero">
      <p class="eyebrow">Agent scratchpad</p>
      <h1>External storage your assistants can read and write.</h1>
      <p class="lead">
        Agentic-only pages stay out of your web library by default, but MCP tools always see them. Use them for
        cross-platform context, chat backups, and anything you want outside a single chat thread.
      </p>
      <div class="hero-actions">
        <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
        <RouterLink v-else to="/connect" class="btn primary large">Connect your AI</RouterLink>
      </div>
    </section>

    <section class="section">
      <div class="usecase-cards">
        <RouterLink
          v-for="p in scratchpadPages"
          :key="p.slug"
          :to="`/agent-scratchpad/${p.slug}`"
          class="usecase-card"
        >
          <strong>{{ p.label }}</strong>
          <span class="muted">{{ p.lead }}</span>
          <span class="usecase-more">Read more →</span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>
