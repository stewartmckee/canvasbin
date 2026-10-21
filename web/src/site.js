// Public site details shown on the marketing and legal pages.
export const site = {
  name: 'Canvasbin',
  url: 'https://canvasbin.dev',
  operator: 'Active Information Design Limited',
  contactEmail: 'support@canvasbin.dev',
  proPrice: '£2.99',
  signalPrice: '£4.99',
  proInterval: 'month',
  teamBasePrice: '£15',
  teamSeatPrice: '£4',
}

const planNames = { pro: 'Individual', signal: 'Signal' }
export const planName = (plan) => planNames[plan] || 'Free'
export const isPaidPlan = (plan) => Boolean(planNames[plan])
