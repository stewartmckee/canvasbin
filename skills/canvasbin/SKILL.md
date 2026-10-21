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

1. If you are publishing into a team, call `get_style_guide` for that team first. When it returns a style guide, write the page in that style. Style guides are a Team plan feature; if the guide is empty or the plan is not active, write the page as usual. Personal pages have no style guide.
2. Write the page. HTML can be a fragment or a full document with `<html>`; scripts run in a sandbox. Use Markdown for plain documents.
3. Call `create_page` with the content, a title, and the format. Pass `team` when the page belongs in a team.
4. Give the user `html_url` (rendered, view-only). Offer `pdf_url` when they want a file.
5. Keep `write_url` / the write key to yourself unless the user asks for it: it can edit and delete the page.

## Changing pages

- `update_page` replaces the content (or changes the title or format) using the write key. Links stay the same.
- `get_page` reads a page's source by its read or write key or URL.
- `list_pages` shows the user's library. `share_page` / `unshare_page` give or remove access by email. `delete_page` removes a page permanently; confirm with the user first.
- `get_style_guide` reads a team's style guide. `set_style_guide` creates it when the team has none and updates it when one already exists. Team plan only; any member of the team can create or update it. Keep following the guide when you update a team page.
