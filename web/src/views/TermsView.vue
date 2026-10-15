<script setup>
import { RouterLink } from 'vue-router'
import { site } from '../site.js'
</script>

<template>
  <div class="marketing legal">
    <h1>Terms of service</h1>
    <p class="muted small">Last updated 15 October 2026</p>
    <p>
      These terms apply when you use {{ site.name }}, operated by {{ site.operator }}. By creating an account or using
      the service you agree to them.
    </p>

    <h2>Your account</h2>
    <p>
      Keep your password safe and your email address up to date. You’re responsible for what happens under your
      account, including anything an AI assistant you connect does on your behalf.
    </p>

    <h2>Your content</h2>
    <p>
      You own the pages you create. You give us permission to store, render and serve them so the service works. You’re
      responsible for your pages and for who you share links with.
    </p>

    <h2>Acceptable use</h2>
    <p>Don’t use {{ site.name }} to publish or distribute:</p>
    <ul>
      <li>phishing, malware or anything designed to deceive or harm visitors;</li>
      <li>illegal content, or content that infringes someone else’s rights;</li>
      <li>spam, or material that harasses or threatens people.</li>
    </ul>
    <p>
      Don’t attempt to disrupt the service, get around plan limits, or access pages and accounts you haven’t been given.
      We may remove pages or suspend accounts that break these rules.
    </p>

    <h2>Plans and billing</h2>
    <p>
      The Free plan is free. Pro is {{ site.proPrice }} per {{ site.proInterval }}, billed in advance through Stripe and
      renewing automatically until you cancel. You can cancel any time from your
      <RouterLink to="/account">account</RouterLink>; Pro continues until the end of the paid period. Payments aren’t
      refunded for partial periods, except where the law requires. If prices change we’ll tell you in advance.
    </p>

    <h2>Availability</h2>
    <p>
      We work hard to keep {{ site.name }} running, but we can’t promise it will always be available or error-free. Keep
      your own copy of anything important.
    </p>

    <h2>Liability</h2>
    <p>
      The service is provided “as is”. To the extent the law allows, we aren’t liable for indirect or consequential
      losses, and our total liability is limited to the amount you paid us in the 12 months before the claim. Nothing in
      these terms limits liability that can’t be limited by law.
    </p>

    <h2>Ending your use</h2>
    <p>
      You can stop using {{ site.name }} at any time. To close your account, contact us at
      <a :href="`mailto:${site.contactEmail}`">{{ site.contactEmail }}</a>.
    </p>

    <h2>Changes</h2>
    <p>We may update these terms. We’ll update the date above and email you about significant changes.</p>
  </div>
</template>
