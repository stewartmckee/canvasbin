<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import PlanCards from '../components/PlanCards.vue'
import { auth } from '../auth.js'
import { pricingFaqs } from '../pricing.js'

const faqs = computed(() => pricingFaqs(auth.freeQuota))
</script>

<template>
  <div class="marketing">
    <header class="page-head">
      <h1>Plans and pricing</h1>
      <p class="lead">Host HTML and Markdown pages free. Upgrade when you need more pages, or bring your team.</p>
    </header>
    <section id="plans">
      <h2 class="sr-only">Plans</h2>
      <PlanCards />
    </section>
    <section id="faq" class="section">
      <h2 class="section-title left">Questions</h2>
      <dl class="faq">
        <template v-for="[q, a] in faqs" :key="q">
          <dt>{{ q }}</dt>
          <dd>{{ a }}</dd>
        </template>
      </dl>
      <p class="related-links">
        <RouterLink to="/features">All features</RouterLink>
        <RouterLink to="/connect">Connect your AI</RouterLink>
        <RouterLink to="/use-cases">Use cases</RouterLink>
        <RouterLink to="/contact">Contact us</RouterLink>
      </p>
    </section>
  </div>
</template>
