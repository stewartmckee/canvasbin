<script setup>
import PlanCards from '../components/PlanCards.vue'
import { auth } from '../auth.js'
import { site } from '../site.js'

const faqs = [
  ['What counts as a page?', 'Each page you own counts once, however many times you edit it. Pages shared with you, and team pages, don’t count toward your personal limit.'],
  ['How does team billing work?', `${site.teamBasePrice} a ${site.proInterval} for the team plus ${site.teamSeatPrice} for each member, including you. Adding or removing someone is prorated automatically.`],
  ['What’s the difference between Individual and Team?', 'Individual is for one person: more pages, shared by link. Team adds a shared space where everyone edits the same pages, comments, a timeline of who changed what, and sharing with people by email.'],
  ['Can I cancel any time?', 'Yes. Cancel from your account or team settings and the plan stays active until the end of the period you’ve paid for.'],
  [
    'What happens to my pages if I cancel?',
    `Nothing is deleted and every link keeps working. You just can’t create new pages while you own ${auth.freeQuota} or more.`,
  ],
  ['Do links expire?', 'No. A page’s links work until you delete the page.'],
  ['How do payments work?', `Individual is ${site.proPrice} a ${site.proInterval}, billed in GBP through Stripe. We never see your card details.`],
  ['Can my AI assistant use my account?', 'That’s the idea! Claude, ChatGPT and Cursor ask you to sign in once, then publish pages into your library.'],
]
</script>

<template>
  <div class="marketing">
    <header class="page-head">
      <h1>Pricing</h1>
      <p class="lead">Start free. Upgrade when you need more pages, or bring your team.</p>
    </header>
    <PlanCards />
    <section class="section">
      <h2 class="section-title left">Questions</h2>
      <dl class="faq">
        <template v-for="[q, a] in faqs" :key="q">
          <dt>{{ q }}</dt>
          <dd>{{ a }}</dd>
        </template>
      </dl>
    </section>
  </div>
</template>
