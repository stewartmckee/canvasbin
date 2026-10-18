import { createMemoryHistory, createRouter, createWebHistory } from 'vue-router'
import StartView from './views/StartView.vue'
import HomeView from './views/HomeView.vue'
import EditView from './views/EditView.vue'
import NotFoundView from './views/NotFoundView.vue'
import LoginView from './views/LoginView.vue'
import RegisterView from './views/RegisterView.vue'
import ForgotPasswordView from './views/ForgotPasswordView.vue'
import ResetPasswordView from './views/ResetPasswordView.vue'
import AccountView from './views/AccountView.vue'
import AdminView from './views/AdminView.vue'
import TeamView from './views/TeamView.vue'
import TeamNewView from './views/TeamNewView.vue'
import FeaturesView from './views/FeaturesView.vue'
import PricingView from './views/PricingView.vue'
import ContactView from './views/ContactView.vue'
import PrivacyView from './views/PrivacyView.vue'
import TermsView from './views/TermsView.vue'
import UseCasesView from './views/UseCasesView.vue'
import UseCaseView from './views/UseCaseView.vue'
import ConnectView from './views/ConnectView.vue'
import { auth, currentUser, initAuth, isAdmin } from './auth.js'
import { applySeo } from './seo.js'
import { pageview } from './analytics.js'

const marketing = (title) => ({ accounts: true, title })

const routes = [
  { path: '/', name: 'home', component: StartView },
  { path: '/new', name: 'new', component: HomeView, meta: { accounts: true, auth: true, title: 'New page' } },
  { path: '/features', name: 'features', component: FeaturesView, meta: marketing('Features') },
  { path: '/pricing', name: 'pricing', component: PricingView, meta: marketing('Pricing') },
  { path: '/use-cases', name: 'use-cases', component: UseCasesView, meta: marketing('Use cases') },
  {
    path: '/use-cases/:slug(developers|business|family|news-apps-games)',
    name: 'use-case',
    component: UseCaseView,
    props: true,
    meta: { accounts: true },
  },
  { path: '/connect', name: 'connect', component: ConnectView, meta: marketing('Connect your AI') },
  {
    path: '/connect/:slug(claude|chatgpt|cursor)',
    name: 'connect-assistant',
    component: ConnectView,
    props: true,
    meta: { accounts: true },
  },
  { path: '/contact', name: 'contact', component: ContactView, meta: marketing('Contact') },
  { path: '/privacy', name: 'privacy', component: PrivacyView, meta: marketing('Privacy policy') },
  { path: '/terms', name: 'terms', component: TermsView, meta: marketing('Terms of service') },
  { path: '/login', name: 'login', component: LoginView, meta: { accounts: true, guest: true, title: 'Sign in' } },
  {
    path: '/register',
    name: 'register',
    component: RegisterView,
    meta: { accounts: true, guest: true, title: 'Create an account' },
  },
  { path: '/forgot-password', name: 'forgot-password', component: ForgotPasswordView, meta: { accounts: true } },
  { path: '/reset-password', name: 'reset-password', component: ResetPasswordView, meta: { accounts: true } },
  { path: '/account', name: 'account', component: AccountView, meta: { accounts: true, auth: true, title: 'Account' } },
  { path: '/teams/new', name: 'team-new', component: TeamNewView, meta: { accounts: true, auth: true, title: 'Start a team' } },
  {
    path: '/teams/:id(\\d+)',
    name: 'team',
    component: TeamView,
    props: (route) => ({ teamId: route.params.id }),
    meta: { accounts: true, auth: true },
  },
  { path: '/admin', name: 'admin', component: AdminView, meta: { accounts: true, auth: true, admin: true } },
  {
    path: '/:key([A-Za-z0-9_-]{43})',
    name: 'page',
    component: EditView,
    props: (route) => ({ pageKey: route.params.key }),
  },
  { path: '/e/:key', redirect: (to) => ({ name: 'page', params: { key: to.params.key } }) },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
]

// The prerender build renders marketing routes with a memory history and no guards.
export function createAppRouter(history) {
  return createRouter({ history, routes, scrollBehavior: (to, from, saved) => saved || { top: 0 } })
}

export const router = createAppRouter(import.meta.env.SSR ? createMemoryHistory() : createWebHistory())

if (!import.meta.env.SSR) installGuards(router)

function installGuards(router) {
  router.beforeEach(async (to) => {
    if (!to.meta.accounts) return true
    await initAuth()
    if (!auth.accounts) return { name: 'home' }
    if (to.meta.auth && !currentUser.value) return { name: 'login', query: { next: to.fullPath } }
    if (to.meta.guest && currentUser.value) return { name: 'home' }
    if (to.meta.admin && !isAdmin.value) return { name: 'home' }
    return true
  })

  // Page views set their own title once the page loads, then count the view.
  router.afterEach((to) => {
    if (to.name === 'page') {
      pageview()
      return
    }
    if (!applySeo(to.path, auth.accounts && !currentUser.value)) {
      document.title = to.meta.title ? `${to.meta.title} · Canvasbin` : 'Canvasbin'
    }
    pageview()
  })
}
