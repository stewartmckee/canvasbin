<script setup>
import { onMounted, ref } from 'vue'
import { listUsers, setUserDisabled } from '../api.js'
import { currentUser } from '../auth.js'

const users = ref([])
const loading = ref(true)
const error = ref('')

async function load() {
  try {
    users.value = (await listUsers()).users || []
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function toggle(u) {
  const disable = !u.disabled_at
  if (disable && !window.confirm(`Disable ${u.email}? They will be signed out and unable to sign in.`)) return
  error.value = ''
  try {
    await setUserDisabled(u.id, disable)
    u.disabled_at = disable ? new Date().toISOString() : null
  } catch (e) {
    error.value = e.message
  }
}

const formatDate = (s) => (s ? new Date(s).toLocaleDateString() : '')
</script>

<template>
  <div class="admin">
    <h1>Users</h1>
    <p v-if="error" class="notice error">{{ error }}</p>
    <p v-if="loading" class="muted">Loading…</p>
    <div v-else class="panel table-wrap">
      <table class="table">
        <thead>
          <tr>
            <th>Email</th>
            <th>Name</th>
            <th>Verified</th>
            <th>Plan</th>
            <th>Pages</th>
            <th>Joined</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in users" :key="u.id" :class="{ off: u.disabled_at }">
            <td>{{ u.email }}</td>
            <td>{{ u.display_name }}</td>
            <td>{{ u.email_verified_at ? 'Yes' : 'No' }}</td>
            <td>{{ u.subscription_status || 'none' }}</td>
            <td>{{ u.page_count }}</td>
            <td>{{ formatDate(u.created_at) }}</td>
            <td>
              <button
                type="button"
                class="btn small"
                :class="{ danger: !u.disabled_at }"
                :disabled="u.id === currentUser.id"
                @click="toggle(u)"
              >
                {{ u.disabled_at ? 'Enable' : 'Disable' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
