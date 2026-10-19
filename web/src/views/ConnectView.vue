<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import McpConnect from '../components/McpConnect.vue'
import PromptList from '../components/PromptList.vue'
import { auth, currentUser } from '../auth.js'
import { assistants, connectHub, connectTools, findAssistant, starterPrompts } from '../connect.js'
import { useCases } from '../usecases.js'

// Without a slug this is the /connect hub; with one, a single assistant's page.
const props = defineProps({ slug: { type: String, default: '' } })
const a = computed(() => (props.slug ? findAssistant(props.slug) : null))
const page = computed(() => a.value || connectHub)
const others = computed(() => assistants.filter((x) => x.slug !== props.slug))
const crumbs = computed(() => [
  { label: 'Home', to: '/' },
  { label: 'Connect your AI', to: '/connect' },
  { label: a.value.label, to: `/connect/${a.value.slug}` },
])
</script>

<template>
  <div class="marketing connect">
    <Breadcrumbs v-if="a" :items="crumbs" />
    <section class="hero">
      <p class="eyebrow">{{ a ? `Canvasbin for ${a.label}` : connectHub.eyebrow }}</p>
      <h1>{{ page.title }}</h1>
      <p class="lead">{{ page.lead }}</p>
      <div class="hero-actions">
        <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Create a free account</RouterLink>
        <a href="#setup" class="btn large">Setup steps</a>
      </div>
    </section>

    <section v-if="!a" class="section">
      <h2 class="section-title">Pick your assistant</h2>
      <div class="usecase-cards">
        <RouterLink v-for="x in assistants" :key="x.slug" :to="`/connect/${x.slug}`" class="usecase-card">
          <strong>{{ x.label }}</strong>
          <span class="muted">{{ x.seoDescription }}</span>
          <span class="usecase-more">Set up {{ x.label }} →</span>
        </RouterLink>
      </div>
    </section>

    <section id="setup" class="section">
      <h2 class="section-title left">{{ a ? `Add Canvasbin to ${a.label}` : 'Setup for every client' }}</h2>
      <McpConnect :only="a ? a.tabs : []" />
    </section>

    <section id="first-pages" class="section">
      <h2 class="section-title left">Connected? Try making a page</h2>
      <p class="muted">Paste one of these into {{ a ? a.label : 'your assistant' }} to make your first page.</p>
      <PromptList :prompts="starterPrompts" />
      <p class="related-links">
        <RouterLink to="/use-cases/news-apps-games">See a news page, a newsroom app and a 3D game made this way</RouterLink>
      </p>
    </section>

    <section v-if="a" id="prompts" class="section">
      <h2 class="section-title left">More prompts for {{ a.label }}</h2>
      <PromptList :prompts="a.prompts" />
    </section>

    <section id="tools" class="section">
      <h2 class="section-title left">What your assistant can do</h2>
      <div class="feature-grid">
        <article v-for="[title, body, tool] in connectTools" :key="tool" class="feature">
          <h3>{{ title }}</h3>
          <p class="muted">{{ body }}</p>
          <p class="muted small"><code>{{ tool }}</code></p>
        </article>
      </div>
      <p class="related-links">
        <RouterLink to="/features#ai">More about working with AI</RouterLink>
      </p>
    </section>

    <section id="faq" class="section">
      <h2 class="section-title left">Questions</h2>
      <dl class="faq">
        <template v-for="[q, ans] in page.faqs" :key="q">
          <dt>{{ q }}</dt>
          <dd>{{ ans }}</dd>
        </template>
      </dl>
    </section>

    <section class="section">
      <h2 class="section-title left">What people build with it</h2>
      <div class="usecase-cards">
        <RouterLink v-for="u in useCases" :key="u.slug" :to="`/use-cases/${u.slug}`" class="usecase-card">
          <span class="usecase-icon">{{ u.icon }}</span>
          <strong>{{ u.label }}</strong>
          <span class="muted">{{ u.summary }}</span>
        </RouterLink>
      </div>
    </section>

    <section v-if="a" class="section">
      <h2 class="section-title left">Other assistants</h2>
      <p class="related-links">
        <RouterLink v-for="x in others" :key="x.slug" :to="`/connect/${x.slug}`">Canvasbin for {{ x.label }}</RouterLink>
        <RouterLink to="/connect">All clients</RouterLink>
        <RouterLink to="/pricing">Pricing</RouterLink>
      </p>
    </section>

    <section class="section cta">
      <h2>Free for up to {{ auth.freeQuota }} pages, no card needed</h2>
      <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
      <RouterLink v-else to="/account" class="btn primary large">Go to your account</RouterLink>
    </section>
  </div>
</template>
