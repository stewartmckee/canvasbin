<script setup>
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import PageEditor from '../components/PageEditor.vue'
import CopyField from '../components/CopyField.vue'
import { createPage, editRoute, htmlPath, parseLink } from '../api.js'
import { forgetPage, recentPages, rememberPage } from '../recent.js'
import { useLeaveGuard } from '../composables/useLeaveGuard.js'

const router = useRouter()
const dirty = useLeaveGuard()
const editor = ref(null)
const saving = ref(false)
const error = ref('')
const openInput = ref('')
const openError = ref('')

const blank = { title: '', format: 'html', content: '' }
const origin = window.location.origin
const mcpUrl = `${origin}/mcp`
const mcpConfig = computed(() =>
  JSON.stringify({ mcpServers: { canvasbin: { url: mcpUrl } } }, null, 2),
)

async function create(page) {
  saving.value = true
  error.value = ''
  try {
    const created = await createPage(page)
    rememberPage(created)
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
    <section class="intro">
      <h1>Share an HTML or Markdown page with a link</h1>
      <p>
        No accounts. Every page gets a <strong>read-only link</strong> and an <strong>edit link</strong> — share
        whichever fits. Coding agents can publish pages through MCP.
      </p>
    </section>

    <p v-if="error" class="notice error">{{ error }}</p>
    <PageEditor ref="editor" :page="blank" mode="create" :saving="saving" @save="create" @dirty="dirty = $event" />

    <div class="home-grid">
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
        <CopyField label="MCP endpoint" :value="mcpUrl" hint="Streamable HTTP, no auth." />
        <pre class="snippet">{{ mcpConfig }}</pre>
      </section>
    </div>
  </div>
</template>
