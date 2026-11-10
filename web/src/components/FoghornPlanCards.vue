<script setup>
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { startCheckout } from '../api.js'
import { auth, currentUser, loadMe } from '../auth.js'
import { isPaidPlan, site } from '../site.js'
import { track } from '../analytics.js'

const plan = computed(() => auth.me?.plan || 'free')
const isPaid = computed(() => isPaidPlan(plan.value))
const busy = ref('')
const error = ref('')

async function upgrade() {
  busy.value = 'pro'
  error.value = ''
  try {
    const res = await startCheckout('pro')
    if (res.url) {
      window.location.href = res.url
      return
    }
    await loadMe()
  } catch (e) {
    error.value = e.message
  }
  busy.value = ''
}
</script>

<template>
  <div class="plans plans-2">
    <article class="plan">
      <h3>Trial</h3>
      <p class="price">£0</p>
      <p class="muted">Try your site and updates before you pay.</p>
      <ul>
        <li>One hosted page (your website)</li>
        <li>Post updates; readers subscribe by email</li>
        <li>Edit with your AI assistant</li>
        <li>No badge on your page</li>
      </ul>
      <RouterLink v-if="!currentUser" to="/register" class="btn" @click="track('plan-click', { plan: 'trial' })">
        Start trial
      </RouterLink>
      <span v-else-if="!isPaid" class="btn ghost" aria-disabled="true">Your current plan</span>
    </article>
    <article class="plan featured">
      <h3>{{ site.name }}</h3>
      <p class="price">
        {{ site.proPrice }}<span class="per">/{{ site.proInterval }}</span>
      </p>
      <p class="muted">Keep the site live after trial; same features, billed monthly.</p>
      <ul>
        <li>One hosted page</li>
        <li>Unlimited updates and email subscribers</li>
        <li>Priority delivery for update emails</li>
        <li>Cancel any time from your account</li>
      </ul>
      <RouterLink
        v-if="!currentUser"
        :to="{ path: '/register', query: { plan: 'pro' } }"
        class="btn primary"
        @click="track('plan-click', { plan: 'paid' })"
      >
        Get started
      </RouterLink>
      <RouterLink v-else-if="plan === 'pro'" to="/account" class="btn">Manage billing</RouterLink>
      <button
        v-else-if="auth.billingEnabled"
        type="button"
        class="btn primary"
        :disabled="Boolean(busy)"
        @click="upgrade"
      >
        {{ busy ? 'Opening checkout…' : 'Upgrade' }}
      </button>
      <p v-if="error" class="field-error">{{ error }}</p>
    </article>
  </div>
</template>

<style scoped>
.plans-2 {
  grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
}
</style>
