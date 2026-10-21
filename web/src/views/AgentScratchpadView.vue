<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import Breadcrumbs from '../components/Breadcrumbs.vue'
import PromptList from '../components/PromptList.vue'
import { currentUser } from '../auth.js'
import { findScratchpad, scratchpadPages } from '../scratchpad.js'

const props = defineProps({ slug: { type: String, required: true } })
const page = computed(() => findScratchpad(props.slug))
const others = computed(() => scratchpadPages.filter((p) => p.slug !== props.slug))
const crumbs = computed(() => {
  const p = page.value
  if (!p) return []
  return [
    { label: 'Home', to: '/' },
    { label: 'Agent scratchpad', to: '/agent-scratchpad' },
    { label: p.label, to: `/agent-scratchpad/${p.slug}` },
  ]
})
</script>

<template>
  <div v-if="page" class="marketing usecase">
    <Breadcrumbs :items="crumbs" />
    <section class="hero">
      <p class="eyebrow">{{ page.eyebrow }}</p>
      <h1>{{ page.title }}</h1>
      <p class="lead">{{ page.lead }}</p>
      <div class="hero-actions">
        <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
        <RouterLink v-else to="/connect" class="btn primary large">Connect your AI</RouterLink>
        <RouterLink to="/agent-scratchpad" class="btn large">All scratchpad guides</RouterLink>
      </div>
    </section>

    <section class="section">
      <h2 class="section-title left">The problem</h2>
      <ul class="schedulers">
        <li v-for="item in page.problem" :key="item">{{ item }}</li>
      </ul>
    </section>

    <section class="section">
      <h2 class="section-title left">How Canvasbin helps</h2>
      <ol class="steps">
        <li v-for="item in page.solution" :key="item">
          <span class="muted">{{ item }}</span>
        </li>
      </ol>
    </section>

    <section id="prompts" class="section">
      <h2 class="section-title left">Prompts to try</h2>
      <PromptList :prompts="page.prompts" />
    </section>

    <section v-if="others.length" class="section">
      <h2 class="section-title left">Also see</h2>
      <div class="usecase-cards">
        <RouterLink v-for="o in others" :key="o.slug" :to="`/agent-scratchpad/${o.slug}`" class="usecase-card">
          <strong>{{ o.label }}</strong>
          <span class="muted">{{ o.lead }}</span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>
