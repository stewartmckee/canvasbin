import { assistants, connectHub } from './connect.js'
import { offers, pricingFaqs } from './pricing.js'
import { site } from './site.js'
import { useCases } from './usecases.js'

// Search and social metadata for the public marketing pages. The prerender
// build writes these into each page's <head>; the app updates them on navigation.
// `crumbs` (parent pages) feed the BreadcrumbList; `faqs` the FAQPage.
const pages = {
  '/': {
    title: 'Canvasbin — publish HTML pages from Claude, ChatGPT and Cursor',
    description:
      'Host and share HTML and Markdown pages at a link, with a PDF and an edit link. Connect the Canvasbin MCP server and your AI publishes and updates them for you.',
  },
  '/features': {
    title: 'Features: HTML and Markdown hosting, PDFs and MCP · Canvasbin',
    description:
      'Visual and source editors, shareable read-only and edit links, PDF export, live updates, email sharing, and an MCP server for Claude, ChatGPT and Cursor.',
  },
  '/pricing': {
    title: 'Pricing: free HTML hosting, Individual and Team plans · Canvasbin',
    description: `Host up to 5 HTML or Markdown pages free. Individual is ${site.proPrice} a month for 100 pages. Team is ${site.teamBasePrice} a month plus ${site.teamSeatPrice} per user, with shared pages and comments.`,
  },
  '/use-cases': {
    title: 'Use cases: living pages built by AI agents · Canvasbin',
    description:
      'Dashboards, reports and diagrams that your AI agent builds and keeps up to date on a schedule. See live examples for developers, business and families.',
  },
  '/connect': { title: connectHub.seoTitle, description: connectHub.seoDescription, faqs: connectHub.faqs },
  '/contact': {
    title: 'Contact · Canvasbin',
    description: 'Get in touch with the Canvasbin team about support, billing, partnerships or reporting a page.',
  },
  '/privacy': {
    title: 'Privacy policy · Canvasbin',
    description: 'What Canvasbin collects, why, who we share it with, how long we keep it, and the controls you have over your data.',
  },
  '/terms': {
    title: 'Terms of service · Canvasbin',
    description: 'The terms for using Canvasbin: your account, your content, acceptable use, plans and billing.',
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
  pages[`/connect/${a.slug}`] = {
    title: a.seoTitle,
    description: a.seoDescription,
    faqs: a.faqs,
    crumbs: [['Connect your AI', '/connect'], [a.label]],
  }
}

export const marketingPaths = Object.keys(pages)

const image = `${site.url}/og.png`

function meta(path) {
  const p = pages[path]
  return p && { ...p, url: site.url + (path === '/' ? '/' : path) }
}

const ctx = 'https://schema.org'

const faqPage = (faqs) => ({
  '@context': ctx,
  '@type': 'FAQPage',
  mainEntity: faqs.map(([q, a]) => ({ '@type': 'Question', name: q, acceptedAnswer: { '@type': 'Answer', text: a } })),
})

function breadcrumbs(path, crumbs) {
  const items = [['Home', '/'], ...crumbs.slice(0, -1), [crumbs[crumbs.length - 1][0], path]]
  return {
    '@context': ctx,
    '@type': 'BreadcrumbList',
    itemListElement: items.map(([name, p], i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name,
      item: site.url + (p === '/' ? '/' : p),
    })),
  }
}

function structuredData(path, quotas) {
  const p = pages[path]
  const app = {
    '@context': ctx,
    '@type': 'SoftwareApplication',
    name: site.name,
    url: `${site.url}/`,
    applicationCategory: 'BusinessApplication',
    operatingSystem: 'Web',
    description: pages['/'].description,
    image,
    offers: offers(quotas),
  }
  const out = []
  if (path === '/') {
    out.push(
      app,
      { '@context': ctx, '@type': 'Organization', name: site.name, url: `${site.url}/`, logo: `${site.url}/icon-512.png` },
      { '@context': ctx, '@type': 'WebSite', name: site.name, url: `${site.url}/` },
    )
  }
  if (path === '/pricing') out.push(app, faqPage(pricingFaqs(quotas.freeQuota)))
  if (p.faqs) out.push(faqPage(p.faqs))
  if (p.crumbs) out.push(breadcrumbs(path, p.crumbs))
  return out
}

const esc = (s) => String(s).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

// headTags returns the <head> markup for a marketing path.
export function headTags(path, quotas) {
  const m = meta(path)
  const tags = [
    `<title>${esc(m.title)}</title>`,
    `<meta name="description" content="${esc(m.description)}" />`,
    `<meta name="robots" content="index, follow" />`,
    `<link rel="canonical" href="${esc(m.url)}" />`,
    `<meta property="og:type" content="website" />`,
    `<meta property="og:site_name" content="${site.name}" />`,
    `<meta property="og:locale" content="en_GB" />`,
    `<meta property="og:title" content="${esc(m.title)}" />`,
    `<meta property="og:description" content="${esc(m.description)}" />`,
    `<meta property="og:url" content="${esc(m.url)}" />`,
    `<meta property="og:image" content="${image}" />`,
    `<meta property="og:image:width" content="1200" />`,
    `<meta property="og:image:height" content="630" />`,
    `<meta name="twitter:card" content="summary_large_image" />`,
    `<meta name="twitter:title" content="${esc(m.title)}" />`,
    `<meta name="twitter:description" content="${esc(m.description)}" />`,
    `<meta name="twitter:image" content="${image}" />`,
  ]
  for (const d of structuredData(path, quotas)) {
    tags.push(`<script type="application/ld+json">${JSON.stringify(d).replace(/</g, '\\u003c')}</script>`)
  }
  return tags.join('\n    ')
}

function setMeta(selector, attr, value) {
  const el = document.head.querySelector(selector)
  if (el) el.setAttribute(attr, value)
}

// applySeo updates the title and share metadata after client-side navigation.
// Returns false when path has no marketing metadata (or is the signed-in home).
export function applySeo(path, signedOut) {
  const m = meta(path)
  if (!m || (path === '/' && !signedOut)) return false
  document.title = m.title
  setMeta('meta[name="description"]', 'content', m.description)
  setMeta('link[rel="canonical"]', 'href', m.url)
  setMeta('meta[property="og:title"]', 'content', m.title)
  setMeta('meta[property="og:description"]', 'content', m.description)
  setMeta('meta[property="og:url"]', 'content', m.url)
  setMeta('meta[name="twitter:title"]', 'content', m.title)
  setMeta('meta[name="twitter:description"]', 'content', m.description)
  return true
}
