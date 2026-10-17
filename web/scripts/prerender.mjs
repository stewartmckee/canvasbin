// Writes dist/prerender/<path>.html for each marketing page: the signed-out
// app HTML plus per-page title, description, social tags and structured data.
// The Go server serves these to signed-out visitors and crawlers.
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const dist = join(root, 'dist')
const ssrDir = join(root, 'dist-ssr')

// Keep in step with the production config (/api/config); the app re-renders
// with live values once it loads.
const config = { freeQuota: 5, proQuota: 100, teamQuota: 1000 }

const { render, marketingPaths } = await import(pathToFileURL(join(ssrDir, 'entry-server.js')).href)
const template = await readFile(join(dist, 'index.html'), 'utf8')
if (!/<!--seo:start-->[\s\S]*<!--seo:end-->/.test(template) || !template.includes('<div id="app"></div>')) {
  throw new Error('index.html is missing the seo markers or the #app container')
}

for (const path of marketingPaths) {
  const { html, head } = await render(path, config)
  const page = template
    .replace(/<!--seo:start-->[\s\S]*<!--seo:end-->/, head)
    .replace('<div id="app"></div>', `<div id="app">${html}</div>`)
  const file = join(dist, 'prerender', path === '/' ? 'index.html' : `${path.slice(1)}.html`)
  await mkdir(dirname(file), { recursive: true })
  await writeFile(file, page)
  console.log('prerendered', path)
}
await rm(ssrDir, { recursive: true, force: true })
