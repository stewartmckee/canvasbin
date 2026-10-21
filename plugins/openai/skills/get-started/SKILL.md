---
name: get-started
description: Set up Canvasbin after installation. Confirms the connection works and helps the user publish their first page.
---

# Get started with Canvasbin

1. Call `list_pages` to confirm the user is signed in. If they already have pages, show the five most recent with their view links.
2. Explain in one or two sentences what Canvasbin does: it publishes HTML or Markdown as a page anyone can open with a link, and pages can be updated later without the link changing.
3. Offer to publish a first page. Suggest something useful from the conversation, or a short welcome page if there's nothing yet. Only publish once the user agrees. If the page is for a team, call `get_style_guide` first and follow the guide when one is set. Style guides are a Team plan feature.
4. After publishing, give the user the `html_url`, and mention that `pdf_url` downloads the page as a PDF.
