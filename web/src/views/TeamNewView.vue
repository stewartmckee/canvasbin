<script setup>
import { computed, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { createTeam, startTeamCheckout } from '../api.js'
import { auth, currentUser, loadMe } from '../auth.js'
import { site } from '../site.js'

const router = useRouter()
const name = ref('')
const busy = ref(false)
const error = ref('')
const verified = computed(() => Boolean(currentUser.value?.email_verified_at))

async function submit() {
  busy.value = true
  error.value = ''
  try {
    const team = await createTeam(name.value)
    await loadMe()
    if (auth.teamBillingEnabled) {
      try {
        const { url } = await startTeamCheckout(team.id)
        window.location.assign(url)
        return
      } catch {
        // The team exists; its page offers checkout again.
      }
    }
    await router.replace(`/teams/${team.id}`)
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}
</script>

<template>
  <div class="panel auth-card">
    <p><RouterLink to="/">← Pages</RouterLink></p>
    <h1>Start a team</h1>
    <p class="muted">
      {{ site.teamBasePrice }} a {{ site.proInterval }} plus {{ site.teamSeatPrice }} per member, up to
      {{ auth.teamQuota }} shared pages. You can invite people once the team is set up.
    </p>
    <p v-if="!verified" class="notice">Verify your email address first — check your inbox for the link.</p>
    <p v-if="error" class="notice error">{{ error }}</p>
    <form class="auth-form" @submit.prevent="submit">
      <label>
        Team name
        <input v-model="name" maxlength="80" required autofocus placeholder="e.g. Acme Design" />
      </label>
      <button class="btn primary" type="submit" :disabled="busy || !verified">
        {{ busy ? 'Setting up…' : auth.teamBillingEnabled ? 'Continue to payment' : 'Create team' }}
      </button>
    </form>
  </div>
</template>
