<script setup>
import { computed, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import McpConnect from '../components/McpConnect.vue'
import { openBillingPortal, startCheckout } from '../api.js'
import { auth, currentUser, isAdmin, logout } from '../auth.js'

const route = useRoute()
const router = useRouter()
const busy = ref(false)
const error = ref('')

const me = computed(() => auth.me)
const isPro = computed(() => me.value?.plan === 'pro')
const quotaText = computed(() => {
  const m = me.value
  if (!m) return ''
  return m.quota > 0 ? `${m.page_count} of ${m.quota} pages` : `${m.page_count} pages`
})
const periodEnd = computed(() => {
  const d = currentUser.value?.current_period_end
  return d ? new Date(d).toLocaleDateString() : ''
})

async function go(fn) {
  busy.value = true
  error.value = ''
  try {
    const { url } = await fn()
    window.location.href = url
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}

async function signOut() {
  await logout()
  router.replace({ name: 'home' })
}
</script>

<template>
  <div class="account">
    <h1>Account</h1>
    <p v-if="route.query.billing === 'success'" class="notice">Thanks! Your plan will update in a moment.</p>
    <p v-if="route.query.upgrade === 'failed'" class="notice error">
      Your account is ready, but checkout couldn't be opened. Use Upgrade to Pro below to try again.
    </p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <div class="home-grid">
      <section class="panel">
        <h2>Profile</h2>
        <dl class="facts">
          <dt>Email</dt>
          <dd>{{ currentUser.email }}</dd>
          <template v-if="currentUser.display_name">
            <dt>Name</dt>
            <dd>{{ currentUser.display_name }}</dd>
          </template>
        </dl>
        <div class="notice-actions">
          <RouterLink v-if="isAdmin" to="/admin" class="btn small">Admin</RouterLink>
          <button type="button" class="btn small" @click="signOut">Sign out</button>
        </div>
      </section>

      <section class="panel">
        <h2>Plan</h2>
        <dl class="facts">
          <dt>Current plan</dt>
          <dd>{{ isPro ? 'Pro' : 'Free' }}</dd>
          <dt>Usage</dt>
          <dd>{{ quotaText }}</dd>
          <template v-if="currentUser.subscription_status && currentUser.subscription_status !== 'none'">
            <dt>Subscription</dt>
            <dd>
              {{ currentUser.subscription_status }}
              <span v-if="periodEnd" class="muted">
                · {{ currentUser.cancel_at_period_end ? 'ends' : 'renews' }} {{ periodEnd }}
              </span>
            </dd>
          </template>
        </dl>
        <template v-if="auth.billingEnabled">
          <button v-if="!isPro" type="button" class="btn primary" :disabled="busy" @click="go(startCheckout)">
            Upgrade to Pro
          </button>
          <button v-else type="button" class="btn" :disabled="busy" @click="go(openBillingPortal)">
            Manage billing
          </button>
        </template>
        <p v-else class="muted small">Billing is not enabled on this server.</p>
      </section>
    </div>

    <section class="panel mt">
      <h2>Connect an AI assistant</h2>
      <p class="muted">
        Add Canvasbin as a connector (MCP server) so your assistant can publish pages straight into your library.
      </p>
      <McpConnect />
    </section>
  </div>
</template>
