<script setup>
import { RouterLink } from 'vue-router'
import { currentUser } from '../auth.js'

const groups = [
  {
    id: 'write',
    title: 'Write',
    items: [
      ['Visual and source editors', 'Format text with a toolbar, or switch to source for full control of your HTML or Markdown.'],
      ['Live preview', 'See exactly what your readers will see before you save.'],
      ['Full HTML documents', 'Paste a fragment or a complete page with its own styles and scripts.'],
      ['Tables, images and code', 'Rich content renders cleanly on desktop and mobile.'],
    ],
  },
  {
    id: 'share',
    title: 'Share',
    items: [
      ['Read-only links', 'A clean, fast page for your audience. They never see the editor.'],
      ['Edit links', 'A private link that lets you, or anyone you trust, change the page.'],
      ['Live updates', 'When a page changes, it refreshes by itself in every viewer’s browser. No reloading needed.'],
      ['PDF downloads', 'Swap the link for a PDF version of any page with one click.'],
      ['Invite by email', 'Give people viewer or editor access. They get an email and the page shows up in their library.'],
    ],
  },
  {
    id: 'ai',
    title: 'Work with AI',
    items: [
      ['Claude, ChatGPT and Cursor', 'Connect once and your assistant can publish, update and share pages for you.'],
      ['Remote MCP server', 'Canvasbin is a remote MCP server over Streamable HTTP, so it works with any MCP client.'],
      ['Secure sign-in', 'Your AI asks you to sign in to Canvasbin before it publishes, and only acts on your account.'],
      ['Keeps pages current', 'Ask your assistant to refresh a report and the same link shows the new version.'],
      ['Runs on a schedule', 'Pair Canvasbin with Cursor Automations or scheduled tasks and dashboards update themselves.'],
      ['Rich, interactive pages', 'Agents can use charts, diagrams and scripts, anything a web page can do.'],
    ],
    links: [
      ['/connect', 'How to connect your AI'],
      ['/connect/claude', 'Canvasbin for Claude'],
      ['/connect/chatgpt', 'Canvasbin for ChatGPT'],
      ['/connect/cursor', 'Canvasbin for Cursor'],
      ['/use-cases', 'Live examples'],
    ],
  },
  {
    id: 'organise',
    title: 'Stay organised',
    items: [
      ['Your library', 'Every page you own and every page shared with you, in one list.'],
      ['Claim older pages', 'Have an edit link from before you signed up? Claim it into your library.'],
      ['Plans that grow with you', 'Start free, upgrade to Individual for more pages, or bring your team for shared pages, comments and a timeline.'],
    ],
    links: [['/pricing#plans', 'Compare plans']],
  },
  {
    id: 'safety',
    title: 'Safe by design',
    items: [
      ['Unguessable links', 'Pages are only reachable by people who have the link or are invited.'],
      ['Out of search engines', 'Your pages are marked not to be indexed.'],
      ['Sandboxed scripts', 'Scripts in a page run isolated from Canvasbin and from your account.'],
    ],
    links: [['/privacy', 'Privacy policy']],
  },
]
</script>

<template>
  <div class="marketing">
    <header class="page-head">
      <h1>HTML and Markdown publishing, built for AI</h1>
      <p class="lead">
        Everything you need to publish a page, share it with the right people and keep it up to date, by hand or from
        Claude, ChatGPT and Cursor.
      </p>
    </header>
    <section v-for="g in groups" :id="g.id" :key="g.title" class="section">
      <h2 class="section-title left">{{ g.title }}</h2>
      <div class="feature-grid">
        <article v-for="[title, body] in g.items" :key="title" class="feature">
          <h3>{{ title }}</h3>
          <p class="muted">{{ body }}</p>
        </article>
      </div>
      <p v-if="g.links" class="related-links">
        <RouterLink v-for="[to, label] in g.links" :key="to" :to="to">{{ label }}</RouterLink>
      </p>
    </section>
    <section class="section cta">
      <h2>Ready to publish?</h2>
      <RouterLink v-if="!currentUser" to="/register" class="btn primary large">Get started free</RouterLink>
      <RouterLink v-else to="/" class="btn primary large">Go to your pages</RouterLink>
    </section>
  </div>
</template>
