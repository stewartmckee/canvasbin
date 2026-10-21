<script setup>
import { computed, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { startCheckout } from '../api.js'
import { auth } from '../auth.js'
import { track } from '../analytics.js'
import { isPaidPlan, planName, site } from '../site.js'

// Shown as Free users near their page limit and whenever a create or claim
// hits it. `from` labels the analytics event.
const props = defineProps({
  from: { type: String, required: true },
})

const used = computed(() => auth.me?.page_count ?? 0)
const quota = computed(() => auth.me?.quota ?? auth.freeQuota)
const full = computed(() => used.value >= quota.value)
const pro = computed(() => isPaidPlan(auth.me?.plan))
const offer = computed(() => {
  if (!pro.value && auth.billingEnabled) return 'individual'
  if (pro.value && auth.teamBillingEnabled) return 'team'
  return ''
})

const busy = ref(false)
const error = ref('')

async function upgrade() {
  track('upgrade-click', { from: props.from, plan: 'individual' })
  busy.value = true
  error.value = ''
  try {
    const { url } = await startCheckout('pro')
    window.location.assign(url)
  } catch (e) {
    error.value = e.message
    busy.value = false
  }
}
</script>

<template>
  <div class="notice upsell" role="status">
    <p>
      <strong v-if="full">You've used all {{ quota }} pages on the {{ planName(auth.me?.plan) }} plan.</strong>
      <strong v-else>You've used {{ used }} of your {{ quota }} free pages.</strong>
      <template v-if="offer === 'individual'">
        Individual gives you {{ auth.proQuota }} pages with no Canvasbin badge, for {{ site.proPrice }} a
        {{ site.proInterval }}.
      </template>
      <template v-else-if="offer === 'team'">
        A Team gives everyone in it {{ auth.teamQuota.toLocaleString() }} shared pages, comments and a style guide, for
        {{ site.teamBasePrice }} a month plus {{ site.teamSeatPrice }} per member.
      </template>
      <template v-if="full">{{ offer ? 'Or delete' : 'Delete' }} a page you no longer need to make room.</template>
    </p>
    <div v-if="offer" class="notice-actions">
      <button v-if="offer === 'individual'" type="button" class="btn primary small" :disabled="busy" @click="upgrade">
        {{ busy ? 'Opening checkout…' : 'Upgrade to Individual' }}
      </button>
      <RouterLink
        v-else
        :to="{ name: 'team-new' }"
        class="btn primary small"
        @click="track('upgrade-click', { from, plan: 'team' })"
      >
        Start a team
      </RouterLink>
      <RouterLink to="/pricing" class="btn small">Compare plans</RouterLink>
    </div>
    <p v-if="error" class="field-error">{{ error }}</p>
  </div>
</template>
