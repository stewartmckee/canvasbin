async function request(method, path, body) {
  const res = await fetch(path, {
    method,
    credentials: 'include',
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  })
  if (res.status === 204) return null
  let data = null
  try {
    data = await res.json()
  } catch {
    // non-JSON error body
  }
  if (!res.ok) {
    const err = new Error(data?.error || `Request failed (${res.status})`)
    err.status = res.status
    err.code = data?.code
    throw err
  }
  return data
}

export const createPage = (page) => request('POST', '/api/pages', page)
export const getPage = (key) => request('GET', `/api/pages/${encodeURIComponent(key)}`)
export const updatePage = (key, page) => request('PUT', `/api/pages/${encodeURIComponent(key)}`, page)
export const deletePage = (key) => request('DELETE', `/api/pages/${encodeURIComponent(key)}`)
export const renderPreview = (page) => request('POST', '/api/render', page)

// Config and auth
export const getConfig = () => request('GET', '/api/config')
export const getMe = () => request('GET', '/api/me')
export const register = (body) => request('POST', '/api/register', body)
export const login = (email, password) => request('POST', '/api/login', { email, password })
export const logoutRequest = () => request('POST', '/api/logout')
export const resendVerification = (email) => request('POST', '/api/resend-verification', { email })
export const forgotPassword = (email) => request('POST', '/api/forgot-password', { email })
export const resetPassword = (token, password) => request('POST', '/api/reset-password', { token, password })

// Library, claiming and sharing
export const getLibrary = () => request('GET', '/api/library')
export const claimPage = (key) => request('POST', '/api/pages/claim', { key })
const sharesPath = (key) => `/api/pages/${encodeURIComponent(key)}/shares`
export const listShares = (key) => request('GET', sharesPath(key))
export const addShare = (key, email, role) => request('POST', sharesPath(key), { email, role })
export const removeShare = (key, email) => request('DELETE', `${sharesPath(key)}/${encodeURIComponent(email)}`)

// Connected apps (MCP clients signed in with OAuth)
export const listConnections = () => request('GET', '/api/connections')
export const revokeConnection = (clientId) => request('POST', '/api/connections/revoke', { client_id: clientId })

// Billing
export const startCheckout = () => request('POST', '/api/billing/checkout')
export const openBillingPortal = () => request('POST', '/api/billing/portal')

// Admin
export const listUsers = () => request('GET', '/api/admin/users')
export const setUserDisabled = (id, disabled) => request('POST', `/api/admin/users/${id}/disable`, { disabled })

// parseLink accepts a bare key or any Canvasbin URL and returns { key, ext },
// where ext is '.html', '.md', '.pdf' or '' (the app).
export function parseLink(input) {
  const s = (input || '').trim()
  const m = s.match(/(?:^|\/)(?:(v|e|raw)\/)?([A-Za-z0-9_-]{43})(\.html|\.md|\.pdf)?\/?(?:[?#].*)?$/)
  if (!m) return null
  const ext = m[3] || (m[1] === 'v' ? '.html' : '')
  return { key: m[2], ext }
}

export const htmlPath = (key) => `/${key}.html`
export const editRoute = (key) => ({ name: 'page', params: { key } })
