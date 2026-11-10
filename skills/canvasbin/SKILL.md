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

For data the user wants stored outside the chat (cross-platform context, chat backups, scratch notes) rather than shared as a polished page, use `agentic_only: true` on `create_page` or on a draft. Those pages are hidden from the human web library by default but appear in `list_pages` with `agentic_only` set. Passing `team` with `agentic_only: true` still creates a personal scratchpad.

## How

1. If you are publishing into a team, call `get_style_guide` for that team first. When it returns a style guide, write the page in that style. Style guides are a Team plan feature; if the guide is empty or the plan is not active, write the page as usual. Personal pages have no style guide.
2. Write the page. HTML can be a fragment or a full document with `<html>`; scripts run in a sandbox. Use Markdown for plain documents.
3. Call `create_page` with the content, a title, and the format. Pass `team` when the page belongs in a team. Gzip the UTF-8 source, standard-base64-encode it, and set `gzipped` to true so the call stays small. Omit `gzipped` to send plain HTML or Markdown. `create_page` publishes in one call.
4. Give the user `html_url` (rendered, view-only). Offer `pdf_url` when they want a file.
5. Keep `write_url` / the write key to yourself unless the user asks for it: it can edit and delete the page.

## Changing pages

- Change a page in two calls. `create_draft` stores one unpublished draft (content, and title or format when those change) and does not touch the live page. Gzip the source the same way as `create_page`. The result has `draft_bytes` and `live_bytes`, not the draft body. Count `expected_bytes` from the source you meant to save and compare it with `draft_bytes`. Do not copy `draft_bytes` into `expected_bytes`.
- `publish_draft` takes the write key, `draft_id`, and that `expected_bytes`. If they differ, nothing is published. If the draft is under half the live page, also pass `confirm_shrink: true`, and only when the shorter page is deliberate.
- `get_page` reads a page's source by its read or write key or URL.
- `list_pages` shows the user's library. `share_page` emails an invitation and `unshare_page` removes access; confirm with the user before either. `delete_page` removes a page permanently; confirm with the user first.
- `get_style_guide` reads a team's style guide. `set_style_guide` creates it when the team has none and updates it when one already exists. Team plan only; any member of the team can create or update it. Keep following the guide when you update a team page.
