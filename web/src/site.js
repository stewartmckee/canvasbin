// Public site details shown on the marketing and legal pages.
export const site = {
  name: 'Canvasbin',
  url: 'https://canvasbin.dev',
  operator: 'Active Information Design Limited',
  contactEmail: 'support@canvasbin.dev',
  proPrice: '£2.99',
  proInterval: 'month',
  teamBasePrice: '£15',
  teamSeatPrice: '£4',
}

export const planName = (plan) => (plan === 'pro' ? 'Individual' : 'Free')
