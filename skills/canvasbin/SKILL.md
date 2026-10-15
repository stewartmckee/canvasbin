---
name: canvasbin
description: Publish HTML or Markdown from the agent as a shareable Canvasbin page. Use when the user asks to share, publish, or send a report, summary, chart, table, diagram, or other generated page, or wants a link or PDF of something you produced.
---

# Canvasbin

Canvasbin hosts HTML and Markdown pages at unguessable URLs. The `canvasbin` MCP server in this plugin publishes to https://canvasbin.dev. The first call asks the user to sign in; pages then appear in their library.

## When to publish

- The user asks for a link, a shareable page, or a PDF of something you produced.
- The output is visual or long (a report, dashboard, table, chart, timeline) and reads better as a page than as chat text.

Don't publish secrets, credentials, or personal data the user hasn't asked to share. Anyone with a view link can open the page.

## How

1. Write the page. HTML can be a fragment or a full document with `<html>`; scripts run in a sandbox. Use Markdown for plain documents.
2. Call `create_page` with the content, a title, and the format.
3. Give the user `html_url` (rendered, view-only). Offer `pdf_url` when they want a file.
4. Keep `write_url` / the write key to yourself unless the user asks for it: it can edit and delete the page.

## Changing pages

- `update_page` replaces the content (or changes the title or format) using the write key. Links stay the same.
- `get_page` reads a page's source by its read or write key or URL.
- `list_pages` shows the user's library. `share_page` / `unshare_page` give or remove access by email. `delete_page` removes a page permanently; confirm with the user first.
