import { computed, reactive } from 'vue'
import { getConfig, getMe, logoutRequest, startCheckout } from './api.js'

// Shared auth state. `me` is the /api/me payload: { user, is_admin, plan, quota, page_count, teams, ... }.
export const auth = reactive({
  ready: false,
  accounts: false,
  billingEnabled: false,
  teamBillingEnabled: false,
  freeQuota: 5,
  proQuota: 100,
  teamQuota: 1000,
  me: null,
})

export const accountsEnabled = computed(() => auth.accounts)
export const currentUser = computed(() => auth.me?.user || null)
export const isAdmin = computed(() => Boolean(auth.me?.is_admin))
export const myTeams = computed(() => auth.me?.teams || [])
export const activeTeams = computed(() => myTeams.value.filter((t) => t.active))
export const inActiveTeam = computed(() => activeTeams.value.length > 0)

export async function loadMe() {
  if (!auth.accounts) {
    auth.me = null
    return null
  }
  try {
    auth.me = await getMe()
  } catch {
    auth.me = null
  }
  return auth.me
}

let initPromise = null

// initAuth loads the public config and, if accounts are on, the current session.
export function initAuth() {
  if (!initPromise) {
    initPromise = (async () => {
      try {
        const cfg = await getConfig()
        auth.accounts = Boolean(cfg?.accounts)
        auth.billingEnabled = Boolean(cfg?.billing_enabled)
        auth.teamBillingEnabled = Boolean(cfg?.team_billing_enabled)
        if (cfg?.free_quota) auth.freeQuota = cfg.free_quota
        if (cfg?.pro_quota) auth.proQuota = cfg.pro_quota
        if (cfg?.team_quota) auth.teamQuota = cfg.team_quota
      } catch {
        auth.accounts = false
      }
      await loadMe()
      auth.ready = true
    })()
  }
  return initPromise
}

// safeNext returns an in-app path to continue to after sign-in, or '/'.
// Rejects protocol-relative ("//host") and backslash tricks that browsers
// treat as another origin.
export function safeNext(value) {
  if (typeof value !== 'string' || !value.startsWith('/')) return '/'
  if (value.startsWith('//') || value.startsWith('/\\') || value.includes('\\')) return '/'
  return value
}

// continueAfterSignIn carries on with the plan the visitor picked before signing
// in: Stripe Checkout for Individual, team setup for Team. It returns false when
// there's nothing to continue so the caller navigates as usual.
export async function continueAfterSignIn(plan, router) {
  if (plan === 'team' && router) {
    await router.replace({ name: 'team-new' })
    return true
  }
  if (plan !== 'pro' || !auth.billingEnabled || auth.me?.plan === 'pro') return false
  const { url } = await startCheckout()
  window.location.assign(url)
  return true
}

export async function logout() {
  try {
    await logoutRequest()
  } finally {
    auth.me = null
  }
}
