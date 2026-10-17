import { offers, pricingFaqs } from './pricing.js'
import { site } from './site.js'
import { useCases } from './usecases.js'

// Search and social metadata for the public marketing pages. The prerender
// build writes these into each page's <head>; the app updates them on navigation.
const pages = {
  '/': {
    title: 'Canvasbin — share HTML and Markdown pages with a link',
    description:
      'Publish HTML and Markdown pages with a shareable link, a PDF and a private edit link. Let Claude, ChatGPT or Cursor publish and update them for you, live.',
  },
  '/features': {
    title: 'Features · Canvasbin',
    description:
      'Visual and source editors, read-only and edit links, PDF downloads, live updates, email sharing and publishing from Claude, ChatGPT and Cursor.',
  },
  '/pricing': {
    title: 'Pricing · Canvasbin',
    description: `Free for up to 5 pages. Individual is ${site.proPrice} a month for 100 pages. Team is ${site.teamBasePrice} a month plus ${site.teamSeatPrice} per user, with shared pages, comments and a timeline.`,
  },
  '/use-cases': {
    title: 'Use cases: living pages built by AI agents · Canvasbin',
    description:
      'Dashboards, reports and diagrams that your AI agent builds and keeps up to date on a schedule. See live examples for developers, business and families.',
  },
  '/contact': {
    title: 'Contact · Canvasbin',
    description: 'Get in touch with the Canvasbin team about support, billing or partnerships.',
  },
  '/privacy': { title: 'Privacy policy · Canvasbin', description: 'How Canvasbin collects, uses and protects your data.' },
  '/terms': { title: 'Terms of service · Canvasbin', description: 'The terms for using Canvasbin.' },
}
for (const u of useCases) {
  pages[`/use-cases/${u.slug}`] = { title: `${u.seoTitle} · Canvasbin`, description: u.summary }
}

export const marketingPaths = Object.keys(pages)

const image = `${site.url}/og.png`

function meta(path) {
  const p = pages[path]
  return p && { ...p, url: site.url + (path === '/' ? '/' : path) }
}

function structuredData(path, quotas) {
  const app = {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name: site.name,
    url: `${site.url}/`,
    applicationCategory: 'BusinessApplication',
    operatingSystem: 'Web',
    description: pages['/'].description,
    image,
    offers: offers(quotas),
  }
  if (path === '/') {
    return [app, { '@context': 'https://schema.org', '@type': 'Organization', name: site.name, url: `${site.url}/`, logo: `${site.url}/favicon.svg` }]
  }
  if (path === '/pricing') {
    return [
      app,
      {
        '@context': 'https://schema.org',
        '@type': 'FAQPage',
        mainEntity: pricingFaqs(quotas.freeQuota).map(([q, a]) => ({
          '@type': 'Question',
          name: q,
          acceptedAnswer: { '@type': 'Answer', text: a },
        })),
      },
    ]
  }
  return []
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
