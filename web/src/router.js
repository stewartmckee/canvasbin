import { createRouter, createWebHistory } from 'vue-router'
import HomeView from './views/HomeView.vue'
import EditView from './views/EditView.vue'
import NotFoundView from './views/NotFoundView.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    {
      path: '/:key([A-Za-z0-9_-]{43})',
      name: 'page',
      component: EditView,
      props: (route) => ({ pageKey: route.params.key }),
    },
    { path: '/e/:key', redirect: (to) => ({ name: 'page', params: { key: to.params.key } }) },
    { path: '/:pathMatch(.*)*', name: 'not-found', component: NotFoundView },
  ],
})
