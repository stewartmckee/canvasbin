import { computed, reactive } from 'vue'
import { getConfig, getMe, logoutRequest, startCheckout } from './api.js'

// Shared auth state. `me` is the /api/me payload: { user, is_admin, plan, quota, page_count, ... }.
export const auth = reactive({
  ready: false,
  accounts: false,
  billingEnabled: false,
  freeQuota: 5,
  proQuota: 1000,
  me: null,
})

export const accountsEnabled = computed(() => auth.accounts)
export const currentUser = computed(() => auth.me?.user || null)
export const isAdmin = computed(() => Boolean(auth.me?.is_admin))

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
        if (cfg?.free_quota) auth.freeQuota = cfg.free_quota
        if (cfg?.pro_quota) auth.proQuota = cfg.pro_quota
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

// continueAfterSignIn goes on to Stripe Checkout when the visitor picked Pro
// before signing in, otherwise returns false so the caller navigates as usual.
export async function continueAfterSignIn(plan) {
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
