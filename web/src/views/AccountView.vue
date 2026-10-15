<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import McpConnect from '../components/McpConnect.vue'
import { listConnections, openBillingPortal, revokeConnection, startCheckout } from '../api.js'
import { auth, currentUser, isAdmin, logout, myTeams } from '../auth.js'

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

const apps = ref(null)
const appsError = ref('')
const revoking = ref('')

async function loadApps() {
  try {
    apps.value = (await listConnections()).apps
  } catch (e) {
    appsError.value = e.message
  }
}

async function disconnect(app) {
  if (!window.confirm(`Disconnect ${app.client_name}? It will need to sign in again to use your account.`)) return
  revoking.value = app.client_id
  appsError.value = ''
  try {
    await revokeConnection(app.client_id)
    apps.value = apps.value.filter((a) => a.client_id !== app.client_id)
  } catch (e) {
    appsError.value = e.message
  } finally {
    revoking.value = ''
  }
}

const fmtDate = (d) => new Date(d).toLocaleDateString()

onMounted(loadApps)
</script>

<template>
  <div class="account">
    <h1>Account</h1>
    <p v-if="route.query.billing === 'success'" class="notice">Thanks! Your plan will update in a moment.</p>
    <p v-if="route.query.upgrade === 'failed'" class="notice error">
      Your account is ready, but checkout couldn't be opened. Use Upgrade to Individual below to try again.
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
          <dd>{{ isPro ? 'Individual' : 'Free' }}</dd>
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
            Upgrade to Individual
          </button>
          <button v-else type="button" class="btn" :disabled="busy" @click="go(openBillingPortal)">
            Manage billing
          </button>
        </template>
        <p v-else class="muted small">Billing is not enabled on this server.</p>
      </section>
    </div>

    <section class="panel mt">
      <div class="team-section-head">
        <h2>Teams</h2>
        <RouterLink to="/teams/new" class="btn small">Start a team</RouterLink>
      </div>
      <p v-if="!myTeams.length" class="muted">
        A team is a shared space where every member can view, edit and comment on the same pages.
        <RouterLink to="/pricing">See pricing</RouterLink>.
      </p>
      <ul v-else class="recent">
        <li v-for="t in myTeams" :key="t.id">
          <RouterLink :to="`/teams/${t.id}`">{{ t.name }}</RouterLink>
          <span class="badge" :class="t.active ? 'ok' : 'warn'">{{ t.active ? 'Active' : 'Not subscribed' }}</span>
          <span class="muted small">{{ t.role }} · {{ t.member_count }} members · {{ t.page_count }} pages</span>
        </li>
      </ul>
    </section>

    <section class="panel mt">
      <h2>Connected apps</h2>
      <p v-if="appsError" class="notice error">{{ appsError }}</p>
      <p v-if="apps && !apps.length" class="muted">No assistants are signed in to your account.</p>
      <ul v-else-if="apps" class="connected-apps">
        <li v-for="app in apps" :key="app.client_id">
          <div>
            <strong>{{ app.client_name }}</strong>
            <span class="muted small">
              · connected {{ fmtDate(app.connected_at) }} · last active {{ fmtDate(app.last_used_at) }}
            </span>
          </div>
          <button
            type="button"
            class="btn small danger"
            :disabled="revoking === app.client_id"
            @click="disconnect(app)"
          >
            Disconnect
          </button>
        </li>
      </ul>
    </section>

    <section class="panel mt">
      <h2>Connect an AI assistant</h2>
      <p class="muted">
        Add Canvasbin as a connector (MCP server) so your assistant can publish pages straight into your library.
      </p>
      <McpConnect />
    </section>
  </div>
</template>
