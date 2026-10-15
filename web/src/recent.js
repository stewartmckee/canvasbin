import { ref } from 'vue'

// Pages this browser created or opened with a write link. Stored only locally.
const STORAGE_KEY = 'canvasbin.recent'
const LIMIT = 30

function load() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY)) || []
  } catch {
    return []
  }
}

export const recentPages = ref(load())

function persist() {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(recentPages.value))
}

export function rememberPage(page) {
  if (!page?.write_key) return
  const entry = {
    writeKey: page.write_key,
    readKey: page.read_key,
    title: page.title || 'Untitled',
    format: page.format,
    updatedAt: page.updated_at,
  }
  recentPages.value = [entry, ...recentPages.value.filter((p) => p.writeKey !== entry.writeKey)].slice(0, LIMIT)
  persist()
}

// forgetPage drops a page by either of its keys.
export function forgetPage(key) {
  recentPages.value = recentPages.value.filter((p) => p.writeKey !== key && p.readKey !== key)
  persist()
}

let pruned = false

// pruneRecent drops pages that were deleted elsewhere (another browser, MCP)
// and refreshes titles. Runs once per app load.
export async function pruneRecent(fetchPage) {
  if (pruned) return
  pruned = true
  const queue = [...recentPages.value]
  const worker = async () => {
    for (let p = queue.shift(); p; p = queue.shift()) {
      try {
        const page = await fetchPage(p.writeKey)
        const entry = recentPages.value.find((e) => e.writeKey === p.writeKey)
        if (entry && page) {
          entry.title = page.title || 'Untitled'
          entry.updatedAt = page.updated_at || entry.updatedAt
        }
      } catch (e) {
        if (e.status === 404) forgetPage(p.writeKey)
      }
    }
  }
  await Promise.all([worker(), worker(), worker()])
  persist()
}
