import { site } from './site.js'

// Shared by the pricing page and its FAQ structured data.
export function pricingFaqs(freeQuota) {
  return [
    ['What counts as a page?', 'Each page you own counts once, however many times you edit it. Pages shared with you, and team pages, don’t count toward your personal limit.'],
    ['How does team billing work?', `${site.teamBasePrice} a ${site.proInterval} for the team plus ${site.teamSeatPrice} for each member, including you. Adding or removing someone is prorated automatically.`],
    ['What’s the difference between Individual and Team?', 'Individual is for one person: more pages, shared by link. Team adds a shared space where everyone edits the same pages, comments, a timeline of who changed what, and sharing with people by email.'],
    ['Is there Canvasbin branding on my pages?', 'Pages on the Free plan show a small “Made with Canvasbin” badge in the corner. Individual and Team pages have no badge.'],
    ['Can I cancel any time?', 'Yes. Cancel from your account or team settings and the plan stays active until the end of the period you’ve paid for.'],
    [
      'What happens to my pages if I cancel?',
      `Nothing is deleted and every link keeps working. You just can’t create new pages while you own ${freeQuota} or more.`,
    ],
    ['Do links expire?', 'No. A page’s links work until you delete the page.'],
    ['How do payments work?', `Individual is ${site.proPrice} a ${site.proInterval}, billed in GBP through Stripe. We never see your card details.`],
    ['Can my AI assistant use my account?', 'That’s the idea! Claude, ChatGPT and Cursor ask you to sign in once, then publish pages into your library.'],
  ]
}

const amount = (price) => price.replace(/[^0-9.]/g, '')

// Offers for the SoftwareApplication structured data.
export function offers({ freeQuota, proQuota, teamQuota }) {
  return [
    { name: 'Free', price: '0', description: `Up to ${freeQuota} pages` },
    { name: 'Individual', price: amount(site.proPrice), description: `Up to ${proQuota} pages, no Canvasbin badge` },
    {
      name: 'Team',
      price: amount(site.teamBasePrice),
      description: `Up to ${teamQuota} shared team pages, plus ${site.teamSeatPrice} per user a ${site.proInterval}`,
    },
  ].map((o) => ({
    '@type': 'Offer',
    name: o.name,
    price: o.price,
    priceCurrency: 'GBP',
    description: o.description,
    url: `${site.url}/pricing`,
  }))
}
