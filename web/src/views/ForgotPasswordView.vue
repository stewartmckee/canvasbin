<script setup>
import { ref } from 'vue'
import { RouterLink } from 'vue-router'
import { forgotPassword } from '../api.js'

const email = ref('')
const busy = ref(false)
const error = ref('')
const sent = ref(false)

async function submit() {
  busy.value = true
  error.value = ''
  try {
    await forgotPassword(email.value)
    sent.value = true
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="panel auth-card">
    <h1>Reset your password</h1>
    <p v-if="sent" class="notice">If an account exists for that email, a reset link is on its way.</p>
    <template v-else>
      <p v-if="error" class="notice error">{{ error }}</p>
      <form class="auth-form" @submit.prevent="submit">
        <label>
          Email
          <input v-model="email" type="email" autocomplete="email" required autofocus />
        </label>
        <button class="btn primary" type="submit" :disabled="busy">{{ busy ? 'Sending…' : 'Send reset link' }}</button>
      </form>
    </template>
    <p class="muted small"><RouterLink to="/login">Back to sign in</RouterLink></p>
  </div>
</template>
