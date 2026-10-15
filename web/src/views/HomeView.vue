<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import PageEditor from '../components/PageEditor.vue'
import McpConnect from '../components/McpConnect.vue'
import { createPage, editRoute, getPage, htmlPath, parseLink } from '../api.js'
import { accountsEnabled, activeTeams, currentUser, loadMe } from '../auth.js'
import { forgetPage, pruneRecent, recentPages, rememberPage } from '../recent.js'
import { useLeaveGuard } from '../composables/useLeaveGuard.js'

const router = useRouter()
const route = useRoute()
const dirty = useLeaveGuard()
const requestedTeam = Number(route.query.team) || 0
const space = ref(activeTeams.value.some((t) => t.id === requestedTeam) ? String(requestedTeam) : '')
const editor = ref(null)
const saving = ref(false)
const error = ref('')
const openInput = ref('')
const openError = ref('')

const blank = { title: '', format: 'html', content: '' }

if (!accountsEnabled.value) pruneRecent(getPage)

async function create(page) {
  saving.value = true
  error.value = ''
  try {
    const created = await createPage(space.value ? { ...page, team_id: Number(space.value) } : page)
    rememberPage(created)
    if (currentUser.value) loadMe()
    editor.value?.markSaved()
    dirty.value = false
    await router.replace({ ...editRoute(created.write_key), state: { created: true } })
  } catch (e) {
    error.value = e.message
  } finally {
    saving.value = false
  }
}

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
    <section v-if="accountsEnabled" class="intro">
      <p><RouterLink to="/">← Pages</RouterLink></p>
      <h1>New page</h1>
    </section>
    <section v-else class="intro">
      <h1>Share an HTML or Markdown page with a link</h1>
      <p>
        No accounts. Every page gets a <strong>read-only link</strong> and an <strong>edit link</strong> — share
        whichever fits. Coding agents can publish pages through MCP.
      </p>
    </section>

    <p v-if="error" class="notice error">{{ error }}</p>
    <div v-if="accountsEnabled && activeTeams.length" class="space-picker">
      <label for="space">Create in</label>
      <select id="space" v-model="space">
        <option value="">Personal</option>
        <option v-for="t in activeTeams" :key="t.id" :value="String(t.id)">{{ t.name }} (team)</option>
      </select>
    </div>
    <PageEditor ref="editor" :page="blank" mode="create" :saving="saving" @save="create" @dirty="dirty = $event" />

    <div v-if="!accountsEnabled" class="home-grid">
      <section class="panel">
        <h2>Open a link</h2>
        <form class="open-form" @submit.prevent="openLink">
          <input v-model="openInput" placeholder="Paste a Canvasbin link or key" aria-label="Canvasbin link or key" />
          <button class="btn" type="submit">Open</button>
        </form>
        <p v-if="openError" class="field-error">{{ openError }}</p>

        <h2 class="mt">Your pages</h2>
        <p v-if="!recentPages.length" class="muted">Pages you create in this browser appear here.</p>
        <ul v-else class="recent">
          <li v-for="p in recentPages" :key="p.writeKey">
            <RouterLink :to="editRoute(p.writeKey)">{{ p.title || 'Untitled' }}</RouterLink>
            <a :href="htmlPath(p.readKey)" class="small" target="_blank" rel="noopener noreferrer">View</a>
            <span class="muted small">{{ p.format === 'markdown' ? 'Markdown' : 'HTML' }} · {{ formatDate(p.updatedAt) }}</span>
            <button type="button" class="link-btn" title="Remove from this list" @click="forgetPage(p.writeKey)">
              Forget
            </button>
          </li>
        </ul>
      </section>

      <section class="panel">
        <h2>Publish from a coding agent</h2>
        <p class="muted">
          Add the MCP server to Cursor, Claude Code or any MCP client. Tools:
          <code>create_page</code>, <code>get_page</code>, <code>update_page</code>, <code>delete_page</code>.
        </p>
        <McpConnect :oauth="false" />
      </section>
    </div>
  </div>
</template>
