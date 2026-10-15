<script setup>
import { RouterLink } from 'vue-router'
import { site } from '../site.js'
</script>

<template>
  <div class="marketing legal">
    <h1>Contact</h1>
    <p class="lead">Questions, feedback or help with your account — we’d love to hear from you.</p>
    <p>
      Email <a :href="`mailto:${site.contactEmail}`">{{ site.contactEmail }}</a> and we’ll get back to you as soon as we
      can, usually within one working day.
    </p>
    <h2>Billing</h2>
    <p>
      You can upgrade, change your card, download invoices or cancel from your
      <RouterLink to="/account">account page</RouterLink>.
    </p>
    <h2>Reporting a page</h2>
    <p>
      If a Canvasbin page breaks our <RouterLink to="/terms">terms</RouterLink> — for example phishing, malware or
      illegal content — email us the link and we’ll look into it.
    </p>
  </div>
</template>
