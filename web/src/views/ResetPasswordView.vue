<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { resetPassword } from '../api.js'

const route = useRoute()
const router = useRouter()
const token = typeof route.query.token === 'string' ? route.query.token : ''
const password = ref('')
const confirm = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  if (password.value !== confirm.value) {
    error.value = 'Passwords do not match.'
    return
  }
  busy.value = true
  try {
    await resetPassword(token, password.value)
    await router.replace({ name: 'login', query: { reset: '1' } })
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="panel auth-card">
    <h1>Choose a new password</h1>
    <template v-if="!token">
      <p class="notice error">This reset link is missing its token.</p>
      <RouterLink to="/forgot-password" class="btn">Request a new link</RouterLink>
    </template>
    <template v-else>
      <p v-if="error" class="notice error">{{ error }}</p>
      <form class="auth-form" @submit.prevent="submit">
        <label>
          New password
          <input v-model="password" type="password" autocomplete="new-password" minlength="8" required autofocus />
        </label>
        <label>
          Confirm password
          <input v-model="confirm" type="password" autocomplete="new-password" minlength="8" required />
        </label>
        <button class="btn primary" type="submit" :disabled="busy">{{ busy ? 'Saving…' : 'Update password' }}</button>
      </form>
    </template>
  </div>
</template>
