<script setup>
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { startCheckout } from '../api.js'
import { auth, currentUser, loadMe, myTeams } from '../auth.js'
import { isPaidPlan, site } from '../site.js'
import { track } from '../analytics.js'

const plan = computed(() => auth.me?.plan || 'free')
const isPaid = computed(() => isPaidPlan(plan.value))
const busy = ref('')
const error = ref('')
const errorPlan = ref('')

async function choose(next) {
  busy.value = next
  error.value = ''
  errorPlan.value = next
  try {
    const res = await startCheckout(next)
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
  <div class="plans" :class="site.showSignal ? 'plans-4' : 'plans-3'">
    <article class="plan">
      <h3>Free</h3>
      <p class="price">£0</p>
      <p class="muted">For trying things out and occasional sharing.</p>
      <ul>
        <li>Up to {{ auth.freeQuota }} pages</li>
        <li>Read-only and edit links for every page</li>
        <li>Rendered page, PDF and Markdown downloads</li>
        <li>Publish from Claude, ChatGPT and Cursor</li>
        <li>Small “Made with Canvasbin” badge on pages</li>
      </ul>
      <RouterLink v-if="!currentUser" to="/register" class="btn" @click="track('plan-click', { plan: 'free' })">
        Get started
      </RouterLink>
      <span v-else-if="!isPaid" class="btn ghost" aria-disabled="true">Your current plan</span>
    </article>
    <article class="plan featured">
      <h3>Individual</h3>
      <p class="price">
        {{ site.proPrice }}<span class="per">/{{ site.proInterval }}</span>
      </p>
      <p class="muted">For one person who publishes a lot, or lets their agents do it.</p>
      <ul>
        <li>Up to {{ auth.proQuota }} pages</li>
        <li>Everything in Free</li>
        <li>No Canvasbin badge</li>
        <li>Cancel any time from your account</li>
      </ul>
      <RouterLink
        v-if="!currentUser"
        :to="{ path: '/register', query: { plan: 'pro' } }"
        class="btn primary"
        @click="track('plan-click', { plan: 'individual' })"
      >
        Start with Individual
      </RouterLink>
      <RouterLink v-else-if="plan === 'pro'" to="/account" class="btn">Manage billing</RouterLink>
      <button
        v-else-if="auth.billingEnabled"
        type="button"
        class="btn primary"
        :disabled="Boolean(busy)"
        @click="choose('pro')"
      >
        {{ busy === 'pro' ? 'Opening checkout…' : isPaid ? 'Switch to Individual' : 'Upgrade to Individual' }}
      </button>
      <p v-if="error && errorPlan === 'pro'" class="field-error">{{ error }}</p>
    </article>
    <article v-if="site.showSignal" class="plan">
      <h3>Signal</h3>
      <p class="price">
        {{ site.signalPrice }}<span class="per">/{{ site.proInterval }}</span>
      </p>
      <p class="muted">For pages people follow: post updates and readers get them by email.</p>
      <ul>
        <li>Up to {{ auth.signalQuota }} pages</li>
        <li>Post updates on your pages</li>
        <li>Readers subscribe by email and get every update</li>
        <li>No Canvasbin badge</li>
        <li>Cancel any time from your account</li>
      </ul>
      <RouterLink
        v-if="!currentUser"
        :to="{ path: '/register', query: { plan: 'signal' } }"
        class="btn"
        @click="track('plan-click', { plan: 'signal' })"
      >
        Start with Signal
      </RouterLink>
      <RouterLink v-else-if="plan === 'signal'" to="/account" class="btn">Manage billing</RouterLink>
      <button
        v-else-if="auth.signalBillingEnabled"
        type="button"
        class="btn"
        :disabled="Boolean(busy)"
        @click="choose('signal')"
      >
        {{ busy === 'signal' ? 'Opening checkout…' : isPaid ? 'Switch to Signal' : 'Upgrade to Signal' }}
      </button>
      <p v-if="error && errorPlan === 'signal'" class="field-error">{{ error }}</p>
    </article>
    <article class="plan">
      <h3>Team</h3>
      <p class="price">
        {{ site.teamBasePrice }}<span class="per">/{{ site.proInterval }}</span>
        <span class="per"> + {{ site.teamSeatPrice }} per user</span>
      </p>
      <p class="muted">A shared space where everyone works on the same pages.</p>
      <ul>
        <li>Up to {{ auth.teamQuota }} team pages</li>
        <li>Every member can view and edit team pages</li>
        <li>Comments and a team timeline</li>
        <li>A style guide assistants follow when writing team pages</li>
        <li>Share pages with people by email</li>
        <li>Owner and admin roles</li>
        <li>No Canvasbin badge</li>
      </ul>
      <RouterLink
        v-if="!currentUser"
        :to="{ path: '/register', query: { plan: 'team' } }"
        class="btn"
        @click="track('plan-click', { plan: 'team' })"
      >
        Start a team
      </RouterLink>
      <RouterLink v-else-if="myTeams.length" :to="`/teams/${myTeams[0].id}`" class="btn">Your team</RouterLink>
      <RouterLink v-else to="/teams/new" class="btn">Start a team</RouterLink>
    </article>
  </div>
</template>
