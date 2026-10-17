<script setup>
import { computed } from 'vue'
import PlanCards from '../components/PlanCards.vue'
import { auth } from '../auth.js'
import { pricingFaqs } from '../pricing.js'

const faqs = computed(() => pricingFaqs(auth.freeQuota))
</script>

<template>
  <div class="marketing">
    <header class="page-head">
      <h1>Pricing</h1>
      <p class="lead">Start free. Upgrade when you need more pages, or bring your team.</p>
    </header>
    <PlanCards />
    <section class="section">
      <h2 class="section-title left">Questions</h2>
      <dl class="faq">
        <template v-for="[q, a] in faqs" :key="q">
          <dt>{{ q }}</dt>
          <dd>{{ a }}</dd>
        </template>
      </dl>
    </section>
  </div>
</template>
