<script setup>
import { computed, ref, watch } from 'vue'
import CopyField from './CopyField.vue'
import { addShare, listShares, removeShare } from '../api.js'
import { accountsEnabled } from '../auth.js'

const props = defineProps({
  page: { type: Object, required: true },
  fresh: { type: Boolean, default: false },
})

const isOwner = computed(() => accountsEnabled.value && props.page.role === 'owner')
const shares = ref([])
const invites = ref([])
const email = ref('')
const role = ref('viewer')
const busy = ref(false)
const error = ref('')

function apply(data) {
  shares.value = data?.shares || []
  invites.value = data?.invites || []
}

async function load() {
  try {
    apply(await listShares(props.page.read_key))
  } catch (e) {
    error.value = e.message
  }
}

watch(isOwner, (v) => v && load(), { immediate: true })

async function invite() {
  busy.value = true
  error.value = ''
  try {
    apply(await addShare(props.page.read_key, email.value, role.value))
    email.value = ''
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}

async function remove(addr) {
  error.value = ''
  try {
    await removeShare(props.page.read_key, addr)
    shares.value = shares.value.filter((s) => s.email !== addr)
    invites.value = invites.value.filter((s) => s.email !== addr)
  } catch (e) {
    error.value = e.message
  }
}
</script>

<template>
  <section class="share" :class="{ fresh }">
    <div class="share-head">
      <h2>{{ fresh ? 'Page created — pick a link to share' : page.can_write ? 'Share' : 'Read-only page' }}</h2>
      <div class="share-actions">
        <a class="btn ghost small" :href="page.html_url" target="_blank" rel="noopener noreferrer">View page</a>
        <a class="btn ghost small" :href="page.pdf_url" download>Download PDF</a>
      </div>
    </div>
    <CopyField
      label="Read-only link"
      hint="Shows the rendered page. Swap .html for .pdf to get a PDF."
      :value="page.html_url"
      tone="read"
    />
    <CopyField
      v-if="page.write_url"
      label="Edit link"
      hint="Opens this editor. Anyone with it can change or delete the page — keep it private."
      :value="page.write_url"
      tone="write"
    />

    <div v-if="isOwner" class="people">
      <h3>People</h3>
      <form class="open-form" @submit.prevent="invite">
        <input v-model="email" type="email" placeholder="Invite by email" aria-label="Email to invite" required />
        <select v-model="role" aria-label="Role">
          <option value="viewer">Viewer</option>
          <option value="editor">Editor</option>
        </select>
        <button class="btn small" type="submit" :disabled="busy">Invite</button>
      </form>
      <p v-if="error" class="field-error">{{ error }}</p>
      <ul v-if="shares.length || invites.length" class="recent">
        <li v-for="s in shares" :key="`s${s.user_id}`">
          <span>{{ s.display_name || s.email }}</span>
          <span class="muted small">{{ s.email }} · {{ s.role }}</span>
          <button type="button" class="link-btn" @click="remove(s.email)">Remove</button>
        </li>
        <li v-for="i in invites" :key="`i${i.id}`">
          <span>{{ i.email }}</span>
          <span class="muted small">invited · {{ i.role }}</span>
          <button type="button" class="link-btn" @click="remove(i.email)">Remove</button>
        </li>
      </ul>
      <p v-else class="muted small">Not shared with anyone yet.</p>
    </div>
  </section>
</template>
