<script setup>
import { computed, onMounted, ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import CopyLinkButton from '../components/CopyLinkButton.vue'
import { editRoute, getLibrary, getPage, htmlPath, parseLink } from '../api.js'
import { auth, currentUser, myTeams } from '../auth.js'
import { planName } from '../site.js'
import { forgetPage, pruneRecent, recentPages } from '../recent.js'

const router = useRouter()
const route = useRoute()
const library = ref([])
const loaded = ref(false)
const loadError = ref('')
const openInput = ref('')
const openError = ref('')

const notice = computed(() => {
  switch (route.query.verify || route.query.billing) {
    case 'ok':
      return 'Email verified. You can create pages now.'
    case 'invalid':
      return 'That verification link is invalid or has expired.'
    case 'disabled':
      return 'This account has been disabled.'
    case 'success':
      return 'Thanks for subscribing! Your plan will update in a moment.'
    case 'cancelled':
      return 'Checkout cancelled — no charge was made.'
    default:
      return ''
  }
})

const owned = computed(() => library.value.filter((p) => p.role === 'owner' && !p.team_id))
const shared = computed(() => library.value.filter((p) => p.role !== 'owner' && !p.team_id))
const teamSections = computed(() =>
  myTeams.value.map((t) => ({ team: t, pages: library.value.filter((p) => p.team_id === t.id) })),
)
const libraryReadKeys = computed(() => new Set(library.value.map((p) => p.read_key).filter(Boolean)))
const localPages = computed(() => recentPages.value.filter((p) => !libraryReadKeys.value.has(p.readKey)))
const verified = computed(() => Boolean(currentUser.value?.email_verified_at))

async function loadLibrary() {
  try {
    library.value = (await getLibrary()).pages || []
  } catch (e) {
    loadError.value = e.message
  } finally {
    loaded.value = true
  }
}

onMounted(async () => {
  const pruning = pruneRecent(getPage)
  await loadLibrary()
  // Opening recent pages by key fills in links for owned pages that lack one.
  if (library.value.some((p) => !p.read_key)) {
    await pruning
    await loadLibrary()
  }
})

function openLink() {
  openError.value = ''
  const parsed = parseLink(openInput.value)
  if (!parsed) {
    openError.value = 'That does not look like a Canvasbin link or key.'
    return
  }
  if (parsed.ext) window.location.href = `/${parsed.key}${parsed.ext}`
  else router.push(editRoute(parsed.key))
}

function formatDate(s) {
  return s ? new Date(s).toLocaleString() : ''
}
</script>

<template>
  <div class="home">
    <section class="intro pages-head">
      <div>
        <h1>Pages</h1>
        <p v-if="auth.me">
          {{ auth.me.page_count }} of {{ auth.me.quota }} personal pages used on the {{ planName(auth.me.plan) }} plan.
        </p>
      </div>
      <RouterLink v-if="verified" to="/new" class="btn primary">Create new page</RouterLink>
      <button v-else type="button" class="btn primary" disabled title="Verify your email address to create pages">
        Create new page
      </button>
    </section>

    <p v-if="notice" class="notice">{{ notice }}</p>
    <p v-if="loadError" class="notice error">{{ loadError }}</p>

    <section v-for="s in teamSections" :key="s.team.id" class="panel mt">
      <div class="team-section-head">
        <h2>
          <RouterLink :to="`/teams/${s.team.id}`">{{ s.team.name }}</RouterLink>
          <span class="badge" :class="s.team.active ? 'ok' : 'warn'">{{ s.team.active ? 'Team' : 'Not subscribed' }}</span>
        </h2>
        <span class="notice-actions">
          <span class="muted small">{{ s.team.page_count }} of {{ s.team.quota }} pages</span>
          <RouterLink v-if="s.team.active && verified" :to="{ path: '/new', query: { team: s.team.id } }" class="btn small">
            New team page
          </RouterLink>
          <RouterLink v-else :to="`/teams/${s.team.id}`" class="btn small">Set up team</RouterLink>
        </span>
      </div>
      <p v-if="loaded && !s.pages.length" class="muted">No team pages yet.</p>
      <ul v-else class="recent">
        <li v-for="p in s.pages" :key="p.read_key">
          <RouterLink :to="editRoute(p.read_key)">{{ p.title || 'Untitled' }}</RouterLink>
          <a :href="p.html_url" class="small" target="_blank" rel="noopener noreferrer">View</a>
          <span class="muted small">{{ p.owner_email || 'you' }} · {{ formatDate(p.updated_at) }}</span>
          <CopyLinkButton :url="p.html_url" />
        </li>
      </ul>
    </section>

    <section class="panel mt">
      <h2 v-if="teamSections.length">Personal</h2>
      <div v-if="loaded && !owned.length" class="empty-pages">
        <p class="muted">
          You have no pages yet.
          <template v-if="!verified">Verify your email address to start creating pages.</template>
        </p>
        <RouterLink v-if="verified" to="/new" class="btn primary">Create your first page</RouterLink>
      </div>
      <ul v-else class="recent">
        <li v-for="(p, i) in owned" :key="p.read_key || `nokey-${i}`">
          <template v-if="p.read_key">
            <RouterLink :to="editRoute(p.read_key)">{{ p.title || 'Untitled' }}</RouterLink>
            <a :href="p.html_url" class="small" target="_blank" rel="noopener noreferrer">View</a>
            <span class="muted small">{{ formatDate(p.updated_at) }}</span>
            <CopyLinkButton :url="p.html_url" />
          </template>
          <template v-else>
            <span>{{ p.title || 'Untitled' }}</span>
            <span class="muted small" title="This page was moved into your account before its link was stored. Open it once from its link and it will appear here.">
              Open once from its link to add it · {{ formatDate(p.updated_at) }}
            </span>
          </template>
        </li>
      </ul>
    </section>

    <div class="home-grid">
      <section class="panel">
        <h2>Shared with you</h2>
        <p v-if="loaded && !shared.length" class="muted">Pages other people share with you appear here.</p>
        <ul v-else class="recent">
          <li v-for="p in shared" :key="p.read_key">
            <RouterLink :to="editRoute(p.read_key)">{{ p.title || 'Untitled' }}</RouterLink>
            <a :href="p.html_url" class="small" target="_blank" rel="noopener noreferrer">View</a>
            <span class="muted small">{{ p.owner_email || 'someone' }} · {{ p.role }} · {{ formatDate(p.updated_at) }}</span>
            <CopyLinkButton :url="p.html_url" />
          </li>
        </ul>

        <h2 class="mt">Open a link</h2>
        <form class="open-form" @submit.prevent="openLink">
          <input v-model="openInput" placeholder="Paste a Canvasbin link or key" aria-label="Canvasbin link or key" />
          <button class="btn" type="submit">Open</button>
        </form>
        <p v-if="openError" class="field-error">{{ openError }}</p>
      </section>

      <section class="panel">
        <h2>Recent in this browser</h2>
        <p v-if="!localPages.length" class="muted">Pages you open with an edit link appear here.</p>
        <ul v-else class="recent">
          <li v-for="p in localPages" :key="p.writeKey">
            <RouterLink :to="editRoute(p.writeKey)">{{ p.title || 'Untitled' }}</RouterLink>
            <a :href="htmlPath(p.readKey)" class="small" target="_blank" rel="noopener noreferrer">View</a>
            <span class="muted small">{{ p.format === 'markdown' ? 'Markdown' : 'HTML' }} · {{ formatDate(p.updatedAt) }}</span>
            <button type="button" class="link-btn" title="Remove from this list" @click="forgetPage(p.writeKey)">
              Forget
            </button>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>
