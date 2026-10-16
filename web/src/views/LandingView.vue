<script setup>
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import PlanCards from '../components/PlanCards.vue'
import { auth } from '../auth.js'
import { useCases } from '../usecases.js'

const route = useRoute()
const notice = computed(
  () =>
    ({
      invalid: 'That verification link is invalid or has expired. Sign in to request a new one.',
      disabled: 'This account has been disabled.',
    })[route.query.verify] || '',
)

const features = [
  {
    title: 'Two links for every page',
    body: 'A read-only link for your audience and a private edit link for you and your collaborators.',
  },
  {
    title: 'HTML or Markdown',
    body: 'Write in a visual editor or straight in source, with a live preview of exactly what people will see.',
  },
  {
    title: 'Web, PDF and Markdown',
    body: 'Every page is a fast web page, a one-click PDF download and, for Markdown, the original source.',
  },
  {
    title: 'Built for AI agents',
    body: 'Claude, ChatGPT and Cursor can publish and update pages for you. Sign in once and they just work.',
  },
  {
    title: 'Share with people',
    body: 'Invite people by email as viewers or editors. Everything you own and everything shared with you, in one library.',
  },
  {
    title: 'Private by default',
    body: 'Unguessable links, pages kept out of search engines, and scripts in your pages run in a locked-down sandbox.',
  },
]
</script>

<template>
  <div class="landing">
    <p v-if="notice" class="notice">{{ notice }}</p>
    <section class="hero">
      <p class="eyebrow">HTML and Markdown pages, shared with a link</p>
      <h1>Turn anything you write — or your AI writes — into a page you can share.</h1>
      <p class="lead">
        Canvasbin gives every page a beautiful web view, a PDF and a private edit link. Reports, notes, prototypes,
        dashboards: publish them in seconds and keep them up to date.
      </p>
      <div class="hero-actions">
        <RouterLink to="/register" class="btn primary large">Get started free</RouterLink>
        <RouterLink to="/pricing" class="btn large">See pricing</RouterLink>
      </div>
      <p class="muted small">Free for up to {{ auth.freeQuota }} pages. No card needed.</p>
    </section>

    <section class="hero-art" aria-hidden="true">
      <div class="mock">
        <div class="mock-bar"><span /><span /><span /></div>
        <div class="mock-body">
          <div class="mock-title" />
          <div class="mock-line w90" />
          <div class="mock-line w75" />
          <div class="mock-line w82" />
          <div class="mock-links">
            <div class="mock-link read"><strong>Read-only link</strong> canvasbin.dev/x7Qa…html</div>
            <div class="mock-link write"><strong>Edit link</strong> canvasbin.dev/9fLk…</div>
          </div>
        </div>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title">How it works</h2>
      <ol class="steps">
        <li>
          <strong>Write or paste</strong>
          <span class="muted">HTML or Markdown, by hand or from your favourite AI assistant.</span>
        </li>
        <li>
          <strong>Get your links</strong>
          <span class="muted">A read-only link to share and an edit link to keep.</span>
        </li>
        <li>
          <strong>Share and update</strong>
          <span class="muted">Send the link, download a PDF, and edit any time — the link stays the same.</span>
        </li>
      </ol>
    </section>

    <section class="section">
      <h2 class="section-title">Everything you need to share a page</h2>
      <div class="feature-grid">
        <article v-for="f in features" :key="f.title" class="feature">
          <h3>{{ f.title }}</h3>
          <p class="muted">{{ f.body }}</p>
        </article>
      </div>
      <p class="center"><RouterLink to="/features">Explore all features →</RouterLink></p>
    </section>

    <section class="section band">
      <div>
        <h2>Let your AI publish for you</h2>
        <p class="muted">
          Ask Claude, ChatGPT or Cursor to turn its answer into a page, and you get a link back — ready to send. Your
          agent can update the page later, and everything it publishes lands in your library.
        </p>
      </div>
      <RouterLink to="/register" class="btn primary">Connect your AI</RouterLink>
    </section>

    <section class="section">
      <h2 class="section-title">Pages that keep themselves up to date</h2>
      <div class="usecase-cards">
        <RouterLink v-for="u in useCases" :key="u.slug" :to="`/use-cases/${u.slug}`" class="usecase-card">
          <span class="usecase-icon">{{ u.icon }}</span>
          <strong>{{ u.label }}</strong>
          <span class="muted">{{ u.summary }}</span>
          <span class="usecase-more">See live examples →</span>
        </RouterLink>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title">Simple pricing</h2>
      <PlanCards />
    </section>

    <section class="section cta">
      <h2>Start sharing in under a minute</h2>
      <RouterLink to="/register" class="btn primary large">Create your free account</RouterLink>
    </section>
  </div>
</template>
