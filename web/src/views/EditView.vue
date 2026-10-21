<script setup>
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import PageEditor from '../components/PageEditor.vue'
import SharePanel from '../components/SharePanel.vue'
import TeamPagePanel from '../components/TeamPagePanel.vue'
import UpdatesPanel from '../components/UpdatesPanel.vue'
import UpgradePrompt from '../components/UpgradePrompt.vue'
import { claimPage, deletePage, getPage, updatePage } from '../api.js'
import { accountsEnabled, currentUser, loadMe } from '../auth.js'
import { forgetPage, rememberPage } from '../recent.js'
import { useLeaveGuard } from '../composables/useLeaveGuard.js'
import { watchPage } from '../live.js'

const props = defineProps({ pageKey: { type: String, required: true } })

const router = useRouter()
const dirty = useLeaveGuard()
const editor = ref(null)
const teamPanel = ref(null)
const page = ref(null)
const initial = shallowRef(null)
const loading = ref(true)
const error = ref('')
const saveError = ref('')
const saving = ref(false)
const status = ref('')
const fresh = ref(Boolean(window.history.state?.created))

function savedStatus(p) {
  return `Saved ${new Date(p.updated_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}`
}

function setTitle(p) {
  document.title = `${p.title || 'Untitled'} · Canvasbin`
}

// Bumped to remount the editor with content that changed elsewhere.
const editorKey = ref(0)
const changedElsewhere = ref(false)
let stopWatching = null
let recheckAfterSave = false

function load(p) {
  page.value = p
  initial.value = { title: p.title, format: p.format, content: p.content }
  status.value = savedStatus(p)
  setTitle(p)
}

onMounted(async () => {
  try {
    const p = await getPage(props.pageKey)
    load(p)
    if (p.can_write) rememberPage(p)
    stopWatching = watchPage(props.pageKey, onLiveEvent)
  } catch (e) {
    if (e.status === 404) forgetPage(props.pageKey)
    error.value = e.status === 404 ? 'This page does not exist or has been deleted.' : e.message
  } finally {
    loading.value = false
  }
})

onBeforeUnmount(() => stopWatching?.())

function onLiveEvent(msg) {
  if (msg.type === 'deleted') {
    forgetPage(props.pageKey)
    dirty.value = false
    error.value = 'This page has been deleted.'
    return
  }
  if (msg.type === 'hello' && page.value && Date.parse(msg.updated_at) === Date.parse(page.value.updated_at)) return
  if (saving.value) {
    recheckAfterSave = true
    return
  }
  checkForChanges()
}

// Our own saves also trigger events; only react if the server copy differs
// from the one we last loaded or saved.
async function checkForChanges() {
  let p
  try {
    p = await getPage(props.pageKey)
  } catch (e) {
    if (e.status === 404) onLiveEvent({ type: 'deleted' })
    return
  }
  if (!page.value || p.updated_at === page.value.updated_at) return
  if (dirty.value) {
    changedElsewhere.value = true
    return
  }
  applyRemote(p)
}

function applyRemote(p) {
  load(p)
  changedElsewhere.value = false
  editorKey.value++
  teamPanel.value?.refreshActivity()
}

async function loadLatest() {
  if (!window.confirm('Load the latest version? Your unsaved changes will be lost.')) return
  try {
    applyRemote(await getPage(props.pageKey))
  } catch (e) {
    saveError.value = e.message
  }
}

async function save(changes) {
  saving.value = true
  saveError.value = ''
  try {
    const p = await updatePage(props.pageKey, changes)
    editor.value?.markSaved(changes)
    page.value = p
    status.value = savedStatus(p)
    rememberPage(p)
    fresh.value = false
    changedElsewhere.value = false
    setTitle(p)
    teamPanel.value?.refreshActivity()
  } catch (e) {
    saveError.value = e.message
  } finally {
    saving.value = false
    if (recheckAfterSave) {
      recheckAfterSave = false
      checkForChanges()
    }
  }
}

const claiming = ref(false)
const claimFull = ref(false)
// A signed-out visitor on a read-only link may have been invited to edit it.
const showSignIn = () => Boolean(accountsEnabled.value && !currentUser.value && page.value && !page.value.can_write)
const signInRoute = () => ({ name: 'login', query: { next: `/${props.pageKey}` } })

const canClaim = () => Boolean(currentUser.value && page.value?.write_key && !page.value.owned)

async function claim() {
  claiming.value = true
  saveError.value = ''
  try {
    page.value = await claimPage(props.pageKey)
    loadMe()
  } catch (e) {
    if (e.code === 'quota_exceeded') claimFull.value = true
    else saveError.value = e.message
  } finally {
    claiming.value = false
  }
}

async function remove() {
  if (!window.confirm('Delete this page permanently? Both the read-only and edit links will stop working.')) return
  try {
    await deletePage(props.pageKey)
    forgetPage(props.pageKey)
    dirty.value = false
    router.replace({ name: 'home' })
  } catch (e) {
    saveError.value = e.message
  }
}
</script>

<template>
  <div class="edit">
    <p v-if="loading" class="muted">Loading…</p>
    <div v-else-if="error" class="panel empty">
      <h1>Page not found</h1>
      <p>{{ error }}</p>
      <RouterLink to="/" class="btn primary">{{ accountsEnabled ? 'Back to home' : 'Create a new page' }}</RouterLink>
    </div>
    <template v-else-if="page">
      <SharePanel :page="page" :fresh="fresh" />
      <UpdatesPanel :page="page" />
      <p v-if="showSignIn()" class="notice claim">
        Shared with you to edit?
        <RouterLink class="btn small" :to="signInRoute()">Sign in</RouterLink>
        <span class="muted small">Pages shared by email open with your access after you sign in.</span>
      </p>
      <p v-if="canClaim()" class="notice claim">
        This page isn't in your library yet.
        <button type="button" class="btn small" :disabled="claiming" @click="claim">
          {{ claiming ? 'Claiming…' : 'Claim page' }}
        </button>
        <span class="muted small">Claiming adds it to your library.</span>
      </p>
      <UpgradePrompt v-if="claimFull && canClaim()" from="claim" />
      <p v-if="saveError" class="notice error">{{ saveError }}</p>
      <p v-if="changedElsewhere" class="notice claim">
        This page was changed somewhere else since you started editing.
        <button type="button" class="btn small" @click="loadLatest">Load latest</button>
        <span class="muted small">Saving now will replace those changes with yours.</span>
      </p>
      <PageEditor
        ref="editor"
        :key="editorKey"
        :page="initial"
        :mode="page.can_write ? 'edit' : 'view'"
        :saving="saving"
        :status="status"
        @save="save"
        @delete="remove"
        @dirty="dirty = $event"
      />
      <TeamPagePanel v-if="page.team_role" ref="teamPanel" :page="page" />
    </template>
  </div>
</template>
