import { track } from './analytics.js'

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

export const createPage = async (page) => {
  const res = await request('POST', '/api/pages', page)
  track('editor-publish', { format: res.format, team: !!page.team_id })
  return res
}
export const getPage = (key) => request('GET', `/api/pages/${encodeURIComponent(key)}`)
export const updatePage = (key, page) => request('PUT', `/api/pages/${encodeURIComponent(key)}`, page)
export const deletePage = (key) => request('DELETE', `/api/pages/${encodeURIComponent(key)}`)
export const renderPreview = (page) => request('POST', '/api/render', page)
export const setUpdatesEnabled = (key, enabled) =>
  request('PATCH', `/api/pages/${encodeURIComponent(key)}/updates-settings`, { updates_enabled: enabled })

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
export const startCheckout = () => {
  track('checkout-start', { plan: 'individual' })
  return request('POST', '/api/billing/checkout')
}
export const openBillingPortal = () => request('POST', '/api/billing/portal')

// Teams
const teamPath = (id) => `/api/teams/${encodeURIComponent(id)}`
export const listTeams = () => request('GET', '/api/teams')
export const createTeam = (name) => request('POST', '/api/teams', { name })
export const getTeam = (id) => request('GET', teamPath(id))
export const renameTeam = (id, name) => request('PATCH', teamPath(id), { name })
export const deleteTeam = (id) => request('DELETE', teamPath(id))
export const addTeamMember = (id, email, role) => request('POST', `${teamPath(id)}/members`, { email, role })
export const setTeamMemberRole = (id, userId, role) => request('PATCH', `${teamPath(id)}/members/${userId}`, { role })
export const removeTeamMember = (id, userId) => request('DELETE', `${teamPath(id)}/members/${userId}`)
export const cancelTeamInvite = (id, inviteId) => request('DELETE', `${teamPath(id)}/invites/${inviteId}`)
export const getTeamPages = (id) => request('GET', `${teamPath(id)}/pages`)
export const getTeamActivity = (id) => request('GET', `${teamPath(id)}/activity`)
export const getStyleGuide = (id) => request('GET', `${teamPath(id)}/style-guide`)
export const setStyleGuide = (id, styleGuide) => request('PUT', `${teamPath(id)}/style-guide`, { style_guide: styleGuide })
export const startTeamCheckout = (id) => {
  track('checkout-start', { plan: 'team' })
  return request('POST', `${teamPath(id)}/billing/checkout`)
}
export const openTeamBillingPortal = (id) => request('POST', `${teamPath(id)}/billing/portal`)

// Comments and activity on team pages
const pagePath = (key) => `/api/pages/${encodeURIComponent(key)}`
export const listComments = (key) => request('GET', `${pagePath(key)}/comments`)
export const addComment = (key, body) => request('POST', `${pagePath(key)}/comments`, { body })
export const deleteComment = (key, id) => request('DELETE', `${pagePath(key)}/comments/${id}`)
export const getPageActivity = (key) => request('GET', `${pagePath(key)}/activity`)

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
