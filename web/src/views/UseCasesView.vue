<script setup>
import { RouterLink } from 'vue-router'
import SamplePreview from '../components/SamplePreview.vue'
import { currentUser } from '../auth.js'
import { capabilities, schedulers, useCases } from '../usecases.js'

const highlights = useCases.map((u) => ({ ...u.samples[0], slug: u.slug, label: u.label }))
</script>

<template>
  <div class="marketing usecase">
    <section class="hero">
      <p class="eyebrow">Use cases</p>
      <h1>Living pages, built and refreshed by your AI agent.</h1>
      <p class="lead">
        Canvasbin pages can be anything a web page can be: charts, diagrams, interactive dashboards. Your agent creates
        them through the Canvasbin MCP and updates them on a schedule, so one link always shows the latest version.
      </p>
      <div class="hero-actions">
        <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
        <RouterLink v-else to="/account" class="btn primary large">Connect your AI</RouterLink>
      </div>
    </section>

    <section class="section">
      <div class="usecase-cards">
        <RouterLink v-for="u in useCases" :key="u.slug" :to="`/use-cases/${u.slug}`" class="usecase-card">
          <span class="usecase-icon">{{ u.icon }}</span>
          <strong>{{ u.label }}</strong>
          <span class="muted">{{ u.summary }}</span>
          <span class="usecase-more">See examples →</span>
        </RouterLink>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title">A taste of what agents build</h2>
      <div class="samples">
        <div v-for="h in highlights" :key="h.key">
          <SamplePreview :sample="h" />
          <p class="center"><RouterLink :to="`/use-cases/${h.slug}`">More {{ h.label.toLowerCase() }} examples →</RouterLink></p>
        </div>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title left">Anything a web page can do</h2>
      <div class="feature-grid">
        <article v-for="[title, body] in capabilities" :key="title" class="feature">
          <h3>{{ title }}</h3>
          <p class="muted">{{ body }}</p>
        </article>
      </div>
    </section>

    <section class="section band">
      <div>
        <h2>Keep it current on a schedule</h2>
        <ul class="schedulers">
          <li v-for="[title, body] in schedulers" :key="title"><strong>{{ title }}.</strong> {{ body }}</li>
        </ul>
      </div>
      <RouterLink :to="currentUser ? '/account' : '/register'" class="btn primary">Connect your AI</RouterLink>
    </section>
  </div>
</template>
