<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { login } from '../api.js'
import { auth, continueAfterSignIn, safeNext } from '../auth.js'

const route = useRoute()
const router = useRouter()
const plan = ['pro', 'team'].includes(route.query.plan) ? route.query.plan : ''
const email = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  busy.value = true
  error.value = ''
  try {
    auth.me = await login(email.value, password.value)
    if (plan) {
      try {
        if (await continueAfterSignIn(plan, router)) return
      } catch {
        await router.replace({ path: '/account', query: { upgrade: 'failed' } })
        return
      }
    }
    const next = safeNext(route.query.next)
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
    <h1>Sign in</h1>
    <p v-if="plan === 'pro'" class="notice">Sign in to continue to Individual checkout.</p>
    <p v-else-if="plan === 'team'" class="notice">Sign in to set up your team.</p>
    <p v-if="route.query.reset" class="notice">Password updated. Sign in with your new password.</p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <form class="auth-form" @submit.prevent="submit">
      <label>
        Email
        <input v-model="email" type="email" autocomplete="email" required autofocus />
      </label>
      <label>
        Password
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <button class="btn primary" type="submit" :disabled="busy">{{ busy ? 'Signing in…' : 'Sign in' }}</button>
    </form>
    <p class="muted small">
      <RouterLink to="/forgot-password">Forgot password?</RouterLink> ·
      <RouterLink :to="{ path: '/register', query: { ...(route.query.next && { next: route.query.next }), ...(plan && { plan }) } }">
        Create an account
      </RouterLink>
    </p>
  </div>
</template>
