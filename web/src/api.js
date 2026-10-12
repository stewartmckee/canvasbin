async function request(method, path, body) {
  const res = await fetch(path, {
    method,
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
    throw err
  }
  return data
}

export const createPage = (page) => request('POST', '/api/pages', page)
export const getPage = (key) => request('GET', `/api/pages/${encodeURIComponent(key)}`)
export const updatePage = (key, page) => request('PUT', `/api/pages/${encodeURIComponent(key)}`, page)
export const deletePage = (key) => request('DELETE', `/api/pages/${encodeURIComponent(key)}`)
export const renderPreview = (page) => request('POST', '/api/render', page)

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
