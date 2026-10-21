<script setup>
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { setUpdatesEnabled } from '../api.js'
import { auth } from '../auth.js'
import { site } from '../site.js'

const props = defineProps({
  page: { type: Object, required: true },
})

const enabled = ref(Boolean(props.page.updates_enabled))
const count = ref(props.page.subscriber_count || 0)
const busy = ref(false)
const error = ref('')

watch(
  () => props.page,
  (p) => {
    enabled.value = Boolean(p.updates_enabled)
    count.value = p.subscriber_count || 0
  },
)

async function toggle(ev) {
  if (!props.page.can_write || busy.value) return
  busy.value = true
  error.value = ''
  const next = ev.target.checked
  try {
    const res = await setUpdatesEnabled(props.page.write_key || props.page.read_key, next)
    enabled.value = res.updates_enabled
    count.value = res.subscriber_count || 0
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section v-if="auth.updates && page.can_write && page.updates_available" class="panel updates-panel">
    <h2>Email updates</h2>
    <p class="muted small">
      Let visitors subscribe on your rendered page. Post updates with
      <code>post_update</code> from your assistant.
    </p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <label class="row">
      <input type="checkbox" :checked="enabled" :disabled="busy" @change="toggle" />
      <span>Public updates and subscriptions</span>
    </label>
    <p v-if="enabled && count" class="muted small">{{ count }} active subscriber{{ count === 1 ? '' : 's' }}</p>
  </section>
  <section v-else-if="auth.updates && page.role === 'owner' && !page.team_id" class="panel updates-panel">
    <h2>Email updates</h2>
    <p class="muted small">
      Post updates on this page and let readers subscribe by email with the Signal plan ({{ site.signalPrice }} a
      {{ site.proInterval }}).
    </p>
    <RouterLink to="/pricing" class="btn small">See Signal</RouterLink>
  </section>
</template>
