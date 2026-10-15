import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router.js'
import { initAuth } from './auth.js'
import './style.css'

initAuth().finally(() => {
  createApp(App).use(router).mount('#app')
})
