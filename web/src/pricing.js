import { site } from './site.js'

// Shared by the pricing page and its FAQ structured data.
export function pricingFaqs(freeQuota) {
  if (site.signalProduct) {
    return [
      ['What do I get on trial?', 'One hosted page (your website), unlimited posted updates, and email subscribers—no card required.'],
      ['What changes when I pay?', `Same single site and updates; ${site.proPrice} a ${site.proInterval} keeps billing active after trial.`],
      ['Can I have more than one page?', 'No—each account is one public site. Update it in place or delete it to start over.'],
      ['How do updates work?', 'You or your assistant posts a short update; it appears on your page and emails everyone subscribed.'],
      ['Can my AI assistant manage the site?', 'Yes. Connect via MCP (see Connect) and it can edit the page and post updates.'],
      ['Can I cancel any time?', 'Yes. Your site keeps working until the end of the period you have paid for.'],
    ]
  }
  return [
    ['What counts as a page?', 'Each page you own counts once, however many times you edit it. Pages shared with you, and team pages, don’t count toward your personal limit.'],
    ['How does team billing work?', `${site.teamBasePrice} a ${site.proInterval} for the team plus ${site.teamSeatPrice} for each member, including you. Adding or removing someone is prorated automatically.`],
    ['What’s the difference between Individual and Team?', 'Individual is for one person: more pages, shared by link. Team adds a shared space where everyone edits the same pages, comments, a timeline of who changed what, a style guide assistants follow when they write team pages, and sharing with people by email.'],
    ['Is there Canvasbin branding on my pages?', 'Pages on the Free plan show a small “Made with Canvasbin” badge in the corner. Individual and Team pages have no badge.'],
    ['Can I cancel any time?', 'Yes. Cancel from your account or team settings and the plan stays active until the end of the period you’ve paid for.'],
    [
      'What happens to my pages if I cancel?',
      `Nothing is deleted and every link keeps working. You just can’t create new pages while you own ${freeQuota} or more.`,
    ],
    ['Do links expire?', 'No. A page’s links work until you delete the page.'],
    [
      'How do payments work?',
      `Individual is ${site.proPrice} a ${site.proInterval}, billed in ${site.currency} through Stripe (UK visitors see GBP; everyone else USD). We never see your card details.`,
    ],
    ['Can my AI assistant use my account?', 'That’s the idea! Your agent asks you to sign in once, then publishes pages into your library over MCP.'],
  ]
}

const amount = (price) => price.replace(/[^0-9.]/g, '')

// Offers for the SoftwareApplication structured data.
export function offers({ freeQuota, proQuota, signalQuota = proQuota, teamQuota }) {
  if (site.signalProduct) {
    return [
      { name: 'Trial', price: '0', description: 'One site, updates and email subscribers' },
      { name: site.name, price: amount(site.proPrice), description: 'One site, unlimited updates, billed monthly' },
    ].map((o) => ({
      '@type': 'Offer',
      name: o.name,
      price: o.price,
      priceCurrency: site.currency || 'GBP',
      description: o.description,
      url: `${site.url}/pricing`,
    }))
  }
  return [
    { name: 'Free', price: '0', description: `Up to ${freeQuota} pages` },
    { name: 'Individual', price: amount(site.proPrice), description: `Up to ${proQuota} pages, no Canvasbin badge` },
    {
      name: 'Team',
      price: amount(site.teamBasePrice),
      description: `Up to ${teamQuota} shared team pages, a style guide, plus ${site.teamSeatPrice} per user a ${site.proInterval}`,
    },
  ].filter(Boolean).map((o) => ({
    '@type': 'Offer',
    name: o.name,
    price: o.price,
    priceCurrency: site.currency || 'GBP',
    description: o.description,
    url: `${site.url}/pricing`,
  }))
}
