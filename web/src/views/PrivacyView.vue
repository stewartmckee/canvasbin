<script setup>
import { site } from '../site.js'
</script>

<template>
  <div class="marketing legal">
    <h1>Privacy policy</h1>
    <p class="muted small">Last updated 15 October 2026</p>
    <p>
      {{ site.name }} is operated by {{ site.operator }} (“we”, “us”). This policy explains what we collect when you use
      {{ site.name }}, why, and the choices you have.
    </p>

    <h2>What we collect</h2>
    <ul>
      <li><strong>Account details:</strong> your email address, an optional display name and a securely hashed password.</li>
      <li><strong>Your pages:</strong> the content, titles and formats of pages you create, and who you share them with.</li>
      <li>
        <strong>Billing:</strong> if you subscribe, Stripe processes your payment. We store your Stripe customer reference
        and subscription status, never your card details.
      </li>
      <li>
        <strong>Connected apps:</strong> if you connect an AI assistant, we store the app’s name and the access you
        granted it.
      </li>
      <li><strong>Technical logs:</strong> request logs used to keep the service running and secure. Page links are removed from logs.</li>
    </ul>

    <h2>How we use it</h2>
    <p>
      To run your account, store and serve your pages, send account emails (verification, password reset and
      invitations), take payment, prevent abuse and support you. We don’t sell your data or use it for advertising.
    </p>

    <h2>Cookies</h2>
    <p>We use a single cookie to keep you signed in. We don’t use analytics or advertising cookies.</p>

    <h2>Who we share it with</h2>
    <ul>
      <li><strong>Stripe</strong> — payment processing.</li>
      <li><strong>Mailgun</strong> (EU region) — sending account and invitation emails.</li>
    </ul>
    <p>We may also disclose information when the law requires it.</p>

    <h2>Your pages and links</h2>
    <p>
      Anyone with a page’s link can view it, and anyone with its edit link can change it. Treat links like passwords
      and only share them with people you trust. Pages are marked not to be indexed by search engines.
    </p>

    <h2>Keeping and deleting data</h2>
    <p>
      We keep your account and pages until you delete them or ask us to close your account. Deleted pages are removed
      immediately. To close your account, export or correct your data, email
      <a :href="`mailto:${site.contactEmail}`">{{ site.contactEmail }}</a>.
    </p>

    <h2>Your rights</h2>
    <p>
      Under UK GDPR you can ask to access, correct or delete your personal data, or object to how we use it. Contact us
      at <a :href="`mailto:${site.contactEmail}`">{{ site.contactEmail }}</a>. You can also complain to the Information
      Commissioner’s Office (ico.org.uk).
    </p>

    <h2>Changes</h2>
    <p>If we change this policy we’ll update the date above, and tell you by email for significant changes.</p>
  </div>
</template>
