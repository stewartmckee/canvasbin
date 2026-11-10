<script setup>
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import FoghornPlanCards from '../components/FoghornPlanCards.vue'
import { auth } from '../auth.js'
import { site } from '../site.js'

const route = useRoute()
const notice = computed(
  () =>
    ({
      invalid: 'That verification link is invalid or has expired. Sign in to request a new one.',
      disabled: 'This account has been disabled.',
    })[route.query.verify] || '',
)
</script>

<template>
  <div class="landing foghorn-landing">
    <p v-if="notice" class="notice">{{ notice }}</p>
    <section class="hero">
      <p class="eyebrow">Your business website, kept current by your AI</p>
      <h1>One link for your business. Updates your customers can follow.</h1>
      <p class="lead">
        {{ site.name }} hosts a single page that is your site—not a folder of files. You and your assistant
        edit the page in place, post short news updates, and readers subscribe by email when something changes.
        No CMS wrestling, no plugin graveyard.
      </p>
      <div class="hero-actions">
        <RouterLink to="/register" class="btn primary large">Start free trial</RouterLink>
        <RouterLink to="/pricing" class="btn large">See plans</RouterLink>
      </div>
      <p class="muted small">Trial includes your site and unlimited updates. No card needed to begin.</p>
    </section>

    <section class="section">
      <h2 class="section-title">How it works</h2>
      <ol class="steps">
        <li>
          <strong>Create your site</strong> — Connect your agent once. Ask it to build your page
          (hours, menu, services, whatever you need).
        </li>
        <li>
          <strong>Post updates</strong> — “We’re open late this Friday” becomes a notice on the page and an email
          to everyone subscribed.
        </li>
        <li>
          <strong>Share one link</strong> — Put the same URL on your van, your Google listing, and your email
          signature. It always shows the latest version.
        </li>
      </ol>
    </section>

    <section id="plans" class="section">
      <h2 class="section-title">Simple pricing</h2>
      <FoghornPlanCards />
    </section>

    <section class="section">
      <h2 class="section-title">Built for assistants</h2>
      <p class="lead narrow">
        {{ site.name }} speaks MCP—the same connection you use for other agent tools. Your assistant can update
        the page, publish an update, and list what’s scheduled without you clicking through an admin panel.
      </p>
      <RouterLink to="/connect" class="btn">Connect your AI</RouterLink>
    </section>
  </div>
</template>

<style scoped>
.foghorn-landing .hero h1 {
  max-width: 22ch;
}
.steps {
  margin: 0;
  padding-left: 1.25rem;
  display: grid;
  gap: 1rem;
  max-width: 42rem;
}
.steps li {
  line-height: 1.5;
}
.lead.narrow {
  max-width: 40rem;
}
</style>
