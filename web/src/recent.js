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

export function forgetPage(writeKey) {
  recentPages.value = recentPages.value.filter((p) => p.writeKey !== writeKey)
  persist()
}
