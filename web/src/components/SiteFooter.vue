<script setup>
import { RouterLink } from 'vue-router'
import { currentUser } from '../auth.js'
import { site } from '../site.js'

const year = new Date().getFullYear()
</script>

<template>
  <footer class="site-footer">
    <div class="footer-inner">
      <div class="footer-brand">
        <RouterLink to="/" class="brand">
          <img src="/favicon.svg" alt="" width="22" height="22" />
          <span>{{ site.name }}</span>
        </RouterLink>
        <p class="muted small">Pages you can share with a link.</p>
      </div>
      <nav class="footer-cols" aria-label="Footer">
        <div>
          <h4>Product</h4>
          <RouterLink to="/features">Features</RouterLink>
          <RouterLink to="/pricing">Pricing</RouterLink>
          <RouterLink v-if="!currentUser" to="/login">Sign in</RouterLink>
          <RouterLink v-else to="/account">Account</RouterLink>
        </div>
        <div>
          <h4>Company</h4>
          <RouterLink to="/contact">Contact</RouterLink>
          <RouterLink to="/privacy">Privacy</RouterLink>
          <RouterLink to="/terms">Terms</RouterLink>
        </div>
      </nav>
    </div>
    <p class="footer-legal muted small">© {{ year }} {{ site.operator }}</p>
  </footer>
</template>
