<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { register } from '../api.js'
import { auth, continueAfterSignIn, safeNext } from '../auth.js'

const route = useRoute()
const router = useRouter()
const next = safeNext(route.query.next)
const plan = route.query.plan === 'pro' ? 'pro' : ''
const loginQuery = { ...(next !== '/' && { next }), ...(plan && { plan }) }
const loginLink = { path: '/login', query: loginQuery }

const displayName = ref('')
const email = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  busy.value = true
  error.value = ''
  try {
    auth.me = await register({ email: email.value, password: password.value, display_name: displayName.value })
    if (plan) {
      try {
        if (await continueAfterSignIn(plan)) return
      } catch {
        // The account exists; the upgrade button on the account page retries.
        await router.replace({ path: '/account', query: { upgrade: 'failed' } })
        return
      }
    }
    // OAuth consent is served by the backend, so it needs a real navigation.
    if (next.startsWith('/oauth/')) {
      window.location.assign(next)
      return
    }
    await router.replace(next)
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="panel auth-card">
    <h1>{{ plan ? 'Start with Pro' : 'Create an account' }}</h1>
    <p v-if="plan" class="notice">Create your account, then you'll go straight to secure checkout.</p>
    <p v-else class="muted">Keep your pages in a library, share them with other people and claim anonymous pages.</p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <form class="auth-form" @submit.prevent="submit">
      <label>
        Name <span class="muted small">(optional)</span>
        <input v-model="displayName" autocomplete="name" />
      </label>
      <label>
        Email
        <input v-model="email" type="email" autocomplete="email" required autofocus />
      </label>
      <label>
        Password
        <input v-model="password" type="password" autocomplete="new-password" minlength="8" required />
      </label>
      <button class="btn primary" type="submit" :disabled="busy">{{ busy ? 'Creating…' : plan ? 'Continue to payment' : 'Create account' }}</button>
    </form>
    <p class="muted small">Already have an account? <RouterLink :to="loginLink">Sign in</RouterLink></p>
  </div>
</template>
