// Public site details shown on the marketing and legal pages.
export const site = {
  name: 'Canvasbin',
  url: 'https://canvasbin.dev',
  operator: 'Active Information Design Limited',
  contactEmail: 'support@canvasbin.dev',
  billingRegion: 'uk',
  currency: 'GBP',
  proPrice: '£4.99',
  proInterval: 'month',
  teamBasePrice: '£15',
  teamSeatPrice: '£4',
  signalProduct: false,
}

function moneySymbol(currency) {
  return currency === 'USD' ? '$' : '£'
}

/** Apply /api/config branding (Foghorn vs Canvasbin) and regional pricing. */
export function applySiteConfig(cfg) {
  if (cfg?.product_name) site.name = cfg.product_name
  if (cfg?.public_url) site.url = String(cfg.public_url).replace(/\/$/, '')
  site.signalProduct = Boolean(cfg?.signal_product)
  const p = cfg?.pricing
  if (p) {
    site.billingRegion = p.region || cfg.billing_region || site.billingRegion
    site.currency = p.currency || cfg.currency || site.currency
    site.proPrice = p.pro_display || `${moneySymbol(site.currency)}${p.pro}`
    if (p.team_base != null && p.team_seat != null) {
      const sym = moneySymbol(site.currency)
      site.teamBasePrice = `${sym}${p.team_base}`
      site.teamSeatPrice = `${sym}${p.team_seat}`
    }
  }
}

const canvasbinPlanNames = { pro: 'Individual', signal: 'Signal' }

export const planName = (plan) => {
  if (site.signalProduct) {
    if (plan === 'pro') return site.name
    return 'Trial'
  }
  return canvasbinPlanNames[plan] || 'Free'
}
export const isPaidPlan = (plan) => plan === 'pro' || (!site.signalProduct && plan === 'signal')
