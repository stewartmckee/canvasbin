import { createSSRApp } from 'vue'
import { createMemoryHistory } from 'vue-router'
import { renderToString } from 'vue/server-renderer'
import App from './App.vue'
import { auth } from './auth.js'
import { createAppRouter } from './router.js'
import { headTags, marketingPaths } from './seo.js'

export { marketingPaths }

// render returns the signed-out HTML and <head> tags for a marketing path.
export async function render(path, config) {
  Object.assign(auth, {
    ready: true,
    accounts: true,
    billingEnabled: true,
    teamBillingEnabled: true,
    signalBillingEnabled: true,
    freeQuota: config.freeQuota,
    proQuota: config.proQuota,
    signalQuota: config.signalQuota,
    teamQuota: config.teamQuota,
    me: null,
  })
  const router = createAppRouter(createMemoryHistory())
  const app = createSSRApp(App).use(router)
  await router.push(path)
  await router.isReady()
  const html = await renderToString(app)
  return { html, head: headTags(path, config) }
}
