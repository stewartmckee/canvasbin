<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import PromptList from '../components/PromptList.vue'
import SamplePreview from '../components/SamplePreview.vue'
import { currentUser } from '../auth.js'
import { capabilities, findUseCase, schedulers, useCases } from '../usecases.js'

const props = defineProps({ slug: { type: String, required: true } })
const uc = computed(() => findUseCase(props.slug))
const others = computed(() => useCases.filter((u) => u.slug !== props.slug))
const crumbs = computed(() => [
  { label: 'Home', to: '/' },
  { label: 'Use cases', to: '/use-cases' },
  { label: uc.value.label, to: `/use-cases/${uc.value.slug}` },
])
</script>

<template>
  <div v-if="uc" class="marketing usecase">
    <Breadcrumbs :items="crumbs" />
    <section class="hero">
      <p class="eyebrow">{{ uc.eyebrow }}</p>
      <h1>{{ uc.title }}</h1>
      <p class="lead">{{ uc.lead }}</p>
      <div class="hero-actions">
        <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
        <RouterLink v-else to="/connect" class="btn primary large">Connect your AI</RouterLink>
        <a href="#samples" class="btn large">See live examples</a>
      </div>
    </section>

    <section id="samples" class="section">
      <h2 class="section-title">Live examples</h2>
      <p class="center muted usecase-note">
        These are real Canvasbin pages built by an agent. Scroll and click inside them: the charts, tabs and filters
        all work.
      </p>
      <div class="samples" :class="{ single: uc.samples.length === 1 }">
        <SamplePreview v-for="s in uc.samples" :key="s.key" :sample="s" :tall="s.featured" />
      </div>
    </section>

    <section id="how-it-works" class="section">
      <h2 class="section-title">How it stays up to date</h2>
      <ol class="steps">
        <li>
          <strong>Ask your agent</strong>
          <span class="muted">
            Connect <RouterLink to="/connect/cursor">Cursor</RouterLink>, <RouterLink to="/connect/claude">Claude</RouterLink>
            or <RouterLink to="/connect/chatgpt">ChatGPT</RouterLink> to Canvasbin and describe the page. It writes the HTML
            and publishes it with one tool call.
          </span>
        </li>
        <li>
          <strong>Point it at your data</strong>
          <span class="muted">The agent reads from the tools it already has: repos, CRMs, spreadsheets, inboxes. Canvasbin just hosts the result.</span>
        </li>
        <li>
          <strong>Put it on a schedule</strong>
          <span class="muted">Run the same prompt hourly, daily or weekly. The agent updates the page in place, so the link never changes.</span>
        </li>
      </ol>
    </section>

    <section id="prompts" class="section">
      <h2 class="section-title left">Prompts to try</h2>
      <PromptList :prompts="uc.prompts" />
    </section>

    <section id="ideas" class="section">
      <h2 class="section-title left">More ideas</h2>
      <div class="feature-grid">
        <article v-for="[title, body] in uc.ideas" :key="title" class="feature">
          <h3>{{ title }}</h3>
          <p class="muted">{{ body }}</p>
        </article>
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
        <h2>Run it on a schedule</h2>
        <ul class="schedulers">
          <li v-for="[title, body] in schedulers" :key="title"><strong>{{ title }}.</strong> {{ body }}</li>
        </ul>
      </div>
      <RouterLink to="/connect" class="btn primary">Connect your AI</RouterLink>
    </section>

    <section class="section">
      <h2 class="section-title left">Other use cases</h2>
      <div class="usecase-cards">
        <RouterLink v-for="o in others" :key="o.slug" :to="`/use-cases/${o.slug}`" class="usecase-card">
          <span class="usecase-icon">{{ o.icon }}</span>
          <strong>{{ o.label }}</strong>
          <span class="muted">{{ o.summary }}</span>
        </RouterLink>
      </div>
      <p class="related-links">
        <RouterLink to="/use-cases">All use cases</RouterLink>
        <RouterLink to="/features">Features</RouterLink>
        <RouterLink to="/pricing">Pricing</RouterLink>
      </p>
    </section>
  </div>
</template>
