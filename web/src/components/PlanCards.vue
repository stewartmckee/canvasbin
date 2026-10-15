<script setup>
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { startCheckout } from '../api.js'
import { auth, currentUser, myTeams } from '../auth.js'
import { site } from '../site.js'

const isPro = computed(() => auth.me?.plan === 'pro')
const busy = ref(false)
const error = ref('')

async function upgrade() {
  busy.value = true
  error.value = ''
  try {
    const { url } = await startCheckout()
    window.location.href = url
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}
</script>

<template>
  <div class="plans plans-3">
    <article class="plan">
      <h3>Free</h3>
      <p class="price">£0</p>
      <p class="muted">For trying things out and occasional sharing.</p>
      <ul>
        <li>Up to {{ auth.freeQuota }} pages</li>
        <li>Read-only and edit links for every page</li>
        <li>Rendered page, PDF and Markdown downloads</li>
        <li>Publish from Claude, ChatGPT and Cursor</li>
      </ul>
      <RouterLink v-if="!currentUser" to="/register" class="btn">Get started</RouterLink>
      <span v-else-if="!isPro" class="btn ghost" aria-disabled="true">Your current plan</span>
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
        <li>Cancel any time from your account</li>
      </ul>
      <RouterLink v-if="!currentUser" :to="{ path: '/register', query: { plan: 'pro' } }" class="btn primary">
        Start with Individual
      </RouterLink>
      <RouterLink v-else-if="isPro" to="/account" class="btn">Manage billing</RouterLink>
      <button v-else-if="auth.billingEnabled" type="button" class="btn primary" :disabled="busy" @click="upgrade">
        {{ busy ? 'Opening checkout…' : 'Upgrade to Individual' }}
      </button>
      <p v-if="error" class="field-error">{{ error }}</p>
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
        <li>Share pages with people by email</li>
        <li>Owner and admin roles</li>
      </ul>
      <RouterLink v-if="!currentUser" :to="{ path: '/register', query: { plan: 'team' } }" class="btn">
        Start a team
      </RouterLink>
      <RouterLink v-else-if="myTeams.length" :to="`/teams/${myTeams[0].id}`" class="btn">Your team</RouterLink>
      <RouterLink v-else to="/teams/new" class="btn">Start a team</RouterLink>
    </article>
  </div>
</template>
