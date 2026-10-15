<script setup>
import { ref } from 'vue'
import { resendVerification } from '../api.js'
import { currentUser } from '../auth.js'

const state = ref('idle') // idle | sending | sent | error
const error = ref('')

async function resend() {
  state.value = 'sending'
  error.value = ''
  try {
    await resendVerification(currentUser.value.email)
    state.value = 'sent'
  } catch (e) {
    error.value = e.message
    state.value = 'error'
  }
}
</script>

<template>
  <div class="verify-banner" role="status">
    <span>
      Please verify your email address. We sent a link to <strong>{{ currentUser.email }}</strong>.
    </span>
    <span v-if="state === 'sent'">Email sent — check your inbox.</span>
    <span v-else-if="state === 'error'">{{ error }}</span>
    <button v-if="state !== 'sent'" type="button" class="link-btn" :disabled="state === 'sending'" @click="resend">
      {{ state === 'sending' ? 'Sending…' : 'Resend email' }}
    </button>
  </div>
</template>
