<script setup>
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import CopyLinkButton from '../components/CopyLinkButton.vue'
import {
  addTeamMember,
  cancelTeamInvite,
  deleteTeam,
  editRoute,
  getStyleGuide,
  getTeam,
  getTeamActivity,
  getTeamPages,
  openTeamBillingPortal,
  removeTeamMember,
  renameTeam,
  setStyleGuide,
  setTeamMemberRole,
  startTeamCheckout,
} from '../api.js'
import { currentUser, loadMe } from '../auth.js'
import { describeActivity, formatWhen } from '../activity.js'
import { site } from '../site.js'
import { track } from '../analytics.js'

const props = defineProps({ teamId: { type: String, required: true } })
const route = useRoute()
const router = useRouter()

const team = ref(null)
const members = ref([])
const invites = ref([])
const pages = ref([])
const activity = ref([])
const billingEnabled = ref(false)
const loadError = ref('')
const error = ref('')
const busy = ref(false)
const email = ref('')
const role = ref('member')
const newName = ref('')
const styleGuide = ref('')
const savedStyle = ref('')
const styleMax = ref(8000)

const canManage = computed(() => ['owner', 'admin'].includes(team.value?.role))
const isOwner = computed(() => team.value?.role === 'owner')
const monthly = computed(() => {
  const n = members.value.length || 1
  return `${site.teamBasePrice} + ${n} × ${site.teamSeatPrice}`
})
const notice = computed(() => {
  if (route.query.billing === 'success') return 'Thanks! The team plan will be active in a moment.'
  if (route.query.billing === 'cancelled') return 'Checkout cancelled — no charge was made.'
  return ''
})

async function load() {
  loadError.value = ''
  try {
    const t = await getTeam(props.teamId)
    team.value = t.team
    members.value = t.members
    invites.value = t.invites
    billingEnabled.value = t.billing_enabled
    newName.value = t.team.name
    document.title = `${t.team.name} · Canvasbin`
    const [p, a, g] = await Promise.all([
      getTeamPages(props.teamId),
      getTeamActivity(props.teamId),
      t.team.active ? getStyleGuide(props.teamId) : null,
    ])
    pages.value = p.pages
    activity.value = a.activity
    styleGuide.value = g?.style_guide || ''
    savedStyle.value = styleGuide.value
    styleMax.value = g?.max_length || 8000
  } catch (e) {
    loadError.value = e.status === 404 ? 'This team does not exist or you are not a member.' : e.message
  }
}

watch(() => props.teamId, load, { immediate: true })

// Stripe's webhook may land just after the redirect back.
if (route.query.billing === 'success') {
  track('checkout-complete', { plan: 'team' })
  setTimeout(() => load().then(loadMe), 3000)
}

async function act(fn) {
  busy.value = true
  error.value = ''
  try {
    return await fn()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

const applyMembers = (data) => {
  if (!data) return
  members.value = data.members
  invites.value = data.invites
}

const invite = () =>
  act(async () => {
    applyMembers(await addTeamMember(props.teamId, email.value, role.value))
    email.value = ''
    activity.value = (await getTeamActivity(props.teamId)).activity
  })

const changeRole = (m, r) => act(async () => applyMembers(await setTeamMemberRole(props.teamId, m.user_id, r)))

const remove = (m) => {
  if (!window.confirm(`Remove ${m.email} from the team?`)) return
  act(async () => applyMembers(await removeTeamMember(props.teamId, m.user_id)))
}

const cancelInvite = (i) => act(async () => applyMembers(await cancelTeamInvite(props.teamId, i.id)))

const leave = () => {
  if (!window.confirm('Leave this team? You will lose access to its pages.')) return
  act(async () => {
    await removeTeamMember(props.teamId, currentUser.value.id)
    await loadMe()
    router.replace('/')
  })
}

const rename = () =>
  act(async () => {
    team.value = await renameTeam(props.teamId, newName.value)
    loadMe()
  })

const saveStyle = () =>
  act(async () => {
    const g = await setStyleGuide(props.teamId, styleGuide.value)
    styleGuide.value = g.style_guide
    savedStyle.value = g.style_guide
    activity.value = (await getTeamActivity(props.teamId)).activity
  })

const remove_ = () => {
  if (!window.confirm('Delete this team? Its pages move back to the people who created them.')) return
  act(async () => {
    await deleteTeam(props.teamId)
    await loadMe()
    router.replace('/')
  })
}

const billing = (fn) =>
  act(async () => {
    const { url } = await fn(props.teamId)
    window.location.href = url
  })
</script>

<template>
  <div class="account">
    <p><RouterLink to="/">← Pages</RouterLink></p>
    <p v-if="loadError" class="notice error">{{ loadError }}</p>
    <template v-else-if="team">
      <section class="intro pages-head">
        <div>
          <h1>{{ team.name }}</h1>
          <p>
            <span class="badge" :class="team.active ? 'ok' : 'warn'">
              {{ team.active ? 'Team plan active' : 'Not subscribed' }}
            </span>
            <span class="muted"> · {{ team.page_count }} of {{ team.quota }} pages · your role: {{ team.role }}</span>
          </p>
        </div>
        <RouterLink v-if="team.active" :to="{ path: '/new', query: { team: team.id } }" class="btn primary">
          New team page
        </RouterLink>
      </section>

      <p v-if="notice" class="notice">{{ notice }}</p>
      <p v-if="error" class="notice error">{{ error }}</p>
      <p v-if="!team.active" class="notice">
        <template v-if="canManage">
          Subscribe to start creating team pages: {{ monthly }} a {{ site.proInterval }}.
          <button
            v-if="billingEnabled"
            type="button"
            class="btn small primary"
            :disabled="busy"
            @click="billing(startTeamCheckout)"
          >
            Subscribe
          </button>
          <span v-else class="muted small">Billing is not enabled on this server.</span>
        </template>
        <template v-else>The team plan isn't active yet. Ask a team owner or admin to subscribe.</template>
      </p>

      <div class="home-grid">
        <section class="panel">
          <h2>Pages</h2>
          <p v-if="!pages.length" class="muted">No team pages yet.</p>
          <ul v-else class="recent">
            <li v-for="p in pages" :key="p.read_key">
              <RouterLink :to="editRoute(p.read_key)">{{ p.title || 'Untitled' }}</RouterLink>
              <a :href="p.html_url" class="small" target="_blank" rel="noopener noreferrer">View</a>
              <span class="muted small">{{ formatWhen(p.updated_at) }}</span>
              <CopyLinkButton :url="p.html_url" />
            </li>
          </ul>
        </section>

        <section class="panel">
          <h2>Timeline</h2>
          <p v-if="!activity.length" class="muted">Nothing has happened yet.</p>
          <ul v-else class="timeline">
            <li v-for="a in activity" :key="a.id">
              <RouterLink v-if="a.page_read_key" :to="editRoute(a.page_read_key)">{{ describeActivity(a) }}</RouterLink>
              <span v-else>{{ describeActivity(a) }}</span>
              <div class="muted small">{{ formatWhen(a.created_at) }}</div>
            </li>
          </ul>
        </section>
      </div>

      <section class="panel mt">
        <h2>Style guide</h2>
        <p class="muted">
          Tell assistants how this team’s pages should be written. They read it before creating a team page. Style
          guides are part of the Team plan.
        </p>
        <form v-if="team.active" class="style-guide-form" @submit.prevent="saveStyle">
          <textarea
            v-model="styleGuide"
            class="style-guide"
            :maxlength="styleMax"
            aria-label="Style guide"
            placeholder="For example: short sentences, British English, no emoji, navy and cream colours."
          />
          <div class="style-guide-actions">
            <button class="btn small primary" type="submit" :disabled="busy || styleGuide === savedStyle">
              Save style guide
            </button>
            <span class="muted small">{{ styleGuide.length }} / {{ styleMax }}</span>
          </div>
        </form>
        <p v-else class="muted">
          A style guide is available once the Team plan is active.
          <RouterLink to="/pricing">See the Team plan</RouterLink>.
        </p>
      </section>

      <section class="panel mt">
        <h2>Members</h2>
        <ul class="recent">
          <li v-for="m in members" :key="m.user_id" class="member-row">
            <span>{{ m.display_name || m.email }}</span>
            <span class="muted small">{{ m.email }}</span>
            <select
              v-if="isOwner && m.role !== 'owner'"
              :value="m.role"
              aria-label="Role"
              :disabled="busy"
              @change="changeRole(m, $event.target.value)"
            >
              <option value="member">Member</option>
              <option value="admin">Admin</option>
            </select>
            <span v-else class="muted small">{{ m.role }}</span>
            <button
              v-if="canManage && m.role !== 'owner' && m.user_id !== currentUser.id && (isOwner || m.role === 'member')"
              type="button"
              class="link-btn"
              @click="remove(m)"
            >
              Remove
            </button>
          </li>
          <li v-for="i in invites" :key="`i${i.id}`">
            <span>{{ i.email }}</span>
            <span class="muted small">invited · {{ i.role }}</span>
            <button v-if="canManage" type="button" class="link-btn" @click="cancelInvite(i)">Cancel</button>
          </li>
        </ul>
        <form v-if="canManage" class="open-form mt" @submit.prevent="invite">
          <input v-model="email" type="email" placeholder="Add someone by email" aria-label="Email" required />
          <select v-model="role" aria-label="Role">
            <option value="member">Member</option>
            <option v-if="isOwner" value="admin">Admin</option>
          </select>
          <button class="btn small" type="submit" :disabled="busy">Add</button>
        </form>
        <p v-if="canManage" class="muted small">
          Each member is a {{ site.teamSeatPrice }}/{{ site.proInterval }} seat, prorated. People without an account get
          an invitation and join when they sign up.
        </p>
      </section>

      <section class="panel mt">
        <h2>Settings</h2>
        <form v-if="canManage" class="open-form" @submit.prevent="rename">
          <input v-model="newName" maxlength="80" aria-label="Team name" required />
          <button class="btn small" type="submit" :disabled="busy || newName === team.name">Rename</button>
        </form>
        <div class="notice-actions mt">
          <button
            v-if="canManage && team.active && billingEnabled"
            type="button"
            class="btn small"
            :disabled="busy"
            @click="billing(openTeamBillingPortal)"
          >
            Manage billing
          </button>
          <button v-if="!isOwner" type="button" class="btn small" :disabled="busy" @click="leave">Leave team</button>
          <button
            v-if="isOwner && !team.active"
            type="button"
            class="btn small danger"
            :disabled="busy"
            @click="remove_"
          >
            Delete team
          </button>
        </div>
      </section>
    </template>
    <p v-else class="muted">Loading…</p>
  </div>
</template>
