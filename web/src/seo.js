import { assistants, connectHub } from './connect.js'
import { offers, pricingFaqs } from './pricing.js'
import { site } from './site.js'
import { useCases } from './usecases.js'
import { scratchpadHub, scratchpadPages } from './scratchpad.js'
import marketingPathsList from '../marketing-paths.json'

// Search and social metadata for the public marketing pages. The prerender
// build writes these into each page's <head>; the app updates them on navigation.
// `crumbs` (parent pages) feed the BreadcrumbList; `faqs` the FAQPage.
// Paths in web/marketing-paths.json must each have an entry here (and match Go sitemap).
const pages = {
  '/': {
    title: 'Canvasbin — publish HTML pages from your AI assistant',
    description:
      'Host and share HTML and Markdown pages at a link, with a PDF and an edit link. Connect the Canvasbin MCP server so your agent publishes and updates them — and use agent scratchpads to back up context and share notes across platforms.',
  },
  '/features': {
    title: 'Features: HTML and Markdown hosting, PDFs and MCP · Canvasbin',
    description:
      'Visual and source editors, shareable links, PDF export, live updates, MCP for your AI assistant, plus agent scratchpads to back up agent context and share project notes across platforms.',
    crumbs: [['Features', '/features']],
  },
  '/pricing': {
    title: 'Pricing: free HTML hosting, Individual and Team plans · Canvasbin',
    description:
      'Host up to 5 HTML or Markdown pages free. Individual and Team plans with monthly billing in GBP (UK) or USD (elsewhere), via Stripe.',
    crumbs: [['Pricing', '/pricing']],
  },
  '/use-cases': {
    title: 'Use cases: living pages built by AI agents · Canvasbin',
    description:
      'Dashboards, reports, news pages, apps and games that your AI agent builds and keeps up to date. See live examples for developers, business and families.',
    crumbs: [['Use cases', '/use-cases']],
  },
  '/connect': {
    title: `${connectHub.seoTitle} · Canvasbin`,
    description: connectHub.seoDescription,
    faqs: connectHub.faqs,
    crumbs: [['Connect your AI', '/connect']],
  },
  '/contact': {
    title: 'Contact · Canvasbin',
    description: 'Get in touch with the Canvasbin team about support, billing, partnerships or reporting a page.',
    crumbs: [['Contact', '/contact']],
  },
  '/privacy': {
    title: 'Privacy policy · Canvasbin',
    description:
      'What Canvasbin collects, why, who we share it with, how long we keep it, and the controls you have over your data.',
    crumbs: [['Privacy policy', '/privacy']],
  },
  '/terms': {
    title: 'Terms of service · Canvasbin',
    description: 'The terms for using Canvasbin: your account, your content, acceptable use, plans and billing.',
    crumbs: [['Terms of service', '/terms']],
  },
}
for (const u of useCases) {
  pages[`/use-cases/${u.slug}`] = {
    title: `${u.seoTitle} · Canvasbin`,
    description: u.seoDescription,
    crumbs: [['Use cases', '/use-cases'], [u.label]],
  }
}
for (const a of assistants) {
  const title = a.seoTitle.endsWith('Canvasbin') ? a.seoTitle : `${a.seoTitle} · Canvasbin`
  pages[`/connect/${a.slug}`] = {
    title,
    description: a.seoDescription,
    faqs: a.faqs,
    crumbs: [['Connect your AI', '/connect'], [a.label]],
  }
}
pages['/agent-scratchpad'] = {
  title: `${scratchpadHub.seoTitle} · Canvasbin`,
  description: scratchpadHub.seoDescription,
  crumbs: [['Agent scratchpad', '/agent-scratchpad']],
}
for (const p of scratchpadPages) {
  pages[`/agent-scratchpad/${p.slug}`] = {
    title: `${p.seoTitle} · Canvasbin`,
    description: p.seoDescription,
    faqs: p.faqs,
    crumbs: [['Agent scratchpad', '/agent-scratchpad'], [p.label]],
  }
}

for (const path of marketingPathsList) {
  if (!pages[path]) {
    throw new Error(`marketing-paths.json lists ${path} but seo.js has no metadata`)
  }
}
export const marketingPaths = marketingPathsList

const image = () => `${site.url}/og.png`

function meta(path) {
  const p = pages[path]
  if (!p) return null
  const url = site.url.replace(/\/$/, '') + (path === '/' ? '/' : path)
  return { ...p, url }
}

const ctx = 'https://schema.org'

const faqPage = (faqs) => ({
  '@context': ctx,
  '@type': 'FAQPage',
  mainEntity: faqs.map(([q, a]) => ({ '@type': 'Question', name: q, acceptedAnswer: { '@type': 'Answer', text: a } })),
})

function breadcrumbs(path, crumbs) {
  const items = [['Home', '/'], ...crumbs.slice(0, -1), [crumbs[crumbs.length - 1][0], path]]
  const base = site.url.replace(/\/$/, '')
  return {
    '@context': ctx,
    '@type': 'BreadcrumbList',
    itemListElement: items.map(([name, p], i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name,
      item: base + (p === '/' ? '/' : p),
    })),
  }
}

export function defaultQuotas(authState) {
  return {
    freeQuota: authState?.freeQuota ?? 5,
    proQuota: authState?.proQuota ?? 100,
    signalQuota: authState?.signalQuota ?? 100,
    teamQuota: authState?.teamQuota ?? 100,
  }
}

export function structuredData(path, quotas) {
  const p = pages[path]
  if (!p) return []
  const img = image()
  const app = {
    '@context': ctx,
    '@type': 'SoftwareApplication',
    name: site.name,
    url: `${site.url.replace(/\/$/, '')}/`,
    applicationCategory: 'BusinessApplication',
    operatingSystem: 'Web',
    description: pages['/'].description,
    image: img,
    offers: offers(quotas),
  }
  const out = []
  if (path === '/') {
    const base = site.url.replace(/\/$/, '')
    out.push(
      app,
      {
        '@context': ctx,
        '@type': 'Organization',
        name: site.name,
        url: `${base}/`,
        logo: `${base}/icon-512.png`,
      },
      {
        '@context': ctx,
        '@type': 'WebSite',
        name: site.name,
        url: `${base}/`,
        description: pages['/'].description,
      },
    )
  }
  if (path === '/pricing') out.push(app, faqPage(pricingFaqs(quotas.freeQuota)))
  if (p.faqs) out.push(faqPage(p.faqs))
  if (p.crumbs) out.push(breadcrumbs(path, p.crumbs))
  return out
}

const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

const ldJson = (data) =>
  `<script type="application/ld+json" data-canvasbin-seo="1">${JSON.stringify(data).replace(/</g, '\\u003c')}</script>`

// headTags returns the <head> markup for a marketing path.
export function headTags(path, quotas) {
  const m = meta(path)
  if (!m) throw new Error(`no SEO metadata for ${path}`)
  const img = image()
  const tags = [
    `<title>${esc(m.title)}</title>`,
    `<meta name="description" content="${esc(m.description)}" />`,
    `<meta name="robots" content="index, follow" />`,
    `<link rel="canonical" href="${esc(m.url)}" />`,
    `<meta property="og:type" content="website" />`,
    `<meta property="og:site_name" content="${esc(site.name)}" />`,
    `<meta property="og:locale" content="en_GB" />`,
    `<meta property="og:title" content="${esc(m.title)}" />`,
    `<meta property="og:description" content="${esc(m.description)}" />`,
    `<meta property="og:url" content="${esc(m.url)}" />`,
    `<meta property="og:image" content="${esc(img)}" />`,
    `<meta property="og:image:width" content="1200" />`,
    `<meta property="og:image:height" content="630" />`,
    `<meta property="og:image:alt" content="${esc(`${site.name} — share HTML and Markdown pages`)}" />`,
    `<meta name="twitter:card" content="summary_large_image" />`,
    `<meta name="twitter:title" content="${esc(m.title)}" />`,
    `<meta name="twitter:description" content="${esc(m.description)}" />`,
    `<meta name="twitter:image" content="${esc(img)}" />`,
  ]
  for (const d of structuredData(path, quotas)) {
    tags.push(ldJson(d))
  }
  return tags.join('\n    ')
}

function ensureMeta(selector, create) {
  let el = document.head.querySelector(selector)
  if (!el && create) {
    el = create()
    document.head.appendChild(el)
  }
  return el
}

function setMeta(selector, attr, value) {
  const el = document.head.querySelector(selector)
  if (el) el.setAttribute(attr, value)
}

function syncJsonLd(path, quotas) {
  document.querySelectorAll('script[data-canvasbin-seo]').forEach((el) => el.remove())
  for (const d of structuredData(path, quotas)) {
    const el = document.createElement('script')
    el.type = 'application/ld+json'
    el.dataset.canvasbinSeo = '1'
    el.textContent = JSON.stringify(d)
    document.head.appendChild(el)
  }
}

// applySeo updates the title and share metadata after client-side navigation.
// Returns false when path has no marketing metadata (or is the signed-in home).
export function applySeo(path, signedOut, quotas = defaultQuotas()) {
  const m = meta(path)
  if (!m || (path === '/' && !signedOut)) {
    applyNoIndex()
    return false
  }
  document.title = m.title
  setMeta('meta[name="description"]', 'content', m.description)
  setMeta('meta[name="robots"]', 'content', 'index, follow')
  const canonical = ensureMeta('link[rel="canonical"]', () => {
    const link = document.createElement('link')
    link.rel = 'canonical'
    return link
  })
  if (canonical) canonical.href = m.url
  setMeta('meta[property="og:title"]', 'content', m.title)
  setMeta('meta[property="og:description"]', 'content', m.description)
  setMeta('meta[property="og:url"]', 'content', m.url)
  setMeta('meta[property="og:image"]', 'content', image())
  setMeta('meta[name="twitter:title"]', 'content', m.title)
  setMeta('meta[name="twitter:description"]', 'content', m.description)
  setMeta('meta[name="twitter:image"]', 'content', image())
  syncJsonLd(path, quotas)
  return true
}

export function applyNoIndex() {
  setMeta('meta[name="robots"]', 'content', 'noindex, nofollow')
  document.querySelectorAll('script[data-canvasbin-seo]').forEach((el) => el.remove())
}
