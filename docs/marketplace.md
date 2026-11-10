# Directory submissions

Listing copy and reviewer notes for the Claude Connectors Directory, the ChatGPT app directory, and the Grok Bot form. Keep this in step with the MCP server in `internal/mcpserver`. The Grok status page is https://canvasbin.dev/azIMyPccXaueDqQ8bs098a9BBifdSjEih6aKEGvuaic.html.

## Listing

| Field | Value |
|---|---|
| Name | Canvasbin |
| Tagline | Publish HTML and Markdown pages at shareable links. |
| Category | Productivity |
| MCP server URL | `https://canvasbin.dev/mcp` (Streamable HTTP) |
| Authentication | OAuth 2.1 with PKCE. Dynamic client registration and client ID metadata documents are both supported. Scopes: `pages:read`, `pages:write`, `offline_access`. |
| Website | https://canvasbin.dev |
| Setup guides | https://canvasbin.dev/connect, https://canvasbin.dev/connect/claude, https://canvasbin.dev/connect/chatgpt |
| Privacy policy | https://canvasbin.dev/privacy |
| Terms | https://canvasbin.dev/terms |
| Support | support@canvasbin.dev, https://canvasbin.dev/contact |
| Logo | https://canvasbin.dev/icon-512.png (512×512 PNG), https://canvasbin.dev/favicon.svg |

### Short description

Publish HTML and Markdown from your conversation as a page anyone can open with a link, and keep it up to date.

### Long description

Canvasbin turns HTML and Markdown into hosted pages with shareable links. Ask your assistant to build a dashboard, report, diagram, meeting notes or a one-page site, and it publishes the result to your Canvasbin library. Each page gets a view link, a PDF download and a private edit link.

Your assistant can update a page in place, so the link people already have always shows the latest version. That makes it easy to keep weekly reports, project status pages and family dashboards current without sending a new file every time.

With a team you can publish into a shared team space, invite people to individual pages by email and discuss pages in comments. On the Team plan, a style guide tells the assistant how that team's pages should be written; it reads the guide before creating a team page.

Pages are served from unguessable links, are marked not to be indexed by search engines, and run any scripts in a sandbox.

## Tools

Every tool has a title and explicit `readOnlyHint`, `destructiveHint` and `openWorldHint` values (`internal/mcpserver/annotations_test.go` enforces this).

| Tool | Title | Read only | Destructive | Open world | What it does |
|---|---|---|---|---|---|
| `create_page` | Create page | no | no | yes | Publishes HTML or Markdown to the user's library or a team space. For a team page, the description tells the assistant to call `get_style_guide` first and follow it. |
| `get_page` | Get page | yes | no | no | Reads a page's title, format, content and links. |
| `create_draft` | Create draft | no | no | no | Stores one unpublished draft. The live page stays as it is. |
| `publish_draft` | Publish draft | no | yes | yes | Copies a draft onto the live page when expected_bytes matches. |
| `delete_page` | Delete page | no | yes | no | Deletes a page. |
| `list_pages` | List my pages | yes | no | no | Lists pages the user owns, was shared, or can reach through a team. |
| `claim_page` | Claim page | no | no | no | Moves an anonymous page into the user's library. |
| `share_page` | Share page | no | yes | yes | Gives someone access by email and sends them an invitation. |
| `unshare_page` | Unshare page | no | yes | no | Removes someone's access. |
| `list_teams` | List my teams | yes | no | no | Lists the user's teams. |
| `list_comments` | List comments | yes | no | no | Reads comments on a team page. |
| `add_comment` | Add comment | no | no | no | Posts a comment on a team page. |
| `get_style_guide` | Get style guide | yes | no | no | Reads a team's style guide. Team plan only. |
| `set_style_guide` | Create or update style guide | no | yes | no | Creates a team's style guide, or replaces it if one already exists. Team plan only; any member. |

## Reviewer account

A verified account with a complimentary active plan, three personal pages, an active team with a team page, a comment and a style guide, so every tool can be exercised. No MFA.

- Email: `review@canvasbin.dev`
- Password: supplied in the submission form only. It is not stored in this repository.

## Test prompts

### Should use Canvasbin

1. "List my Canvasbin pages." Expect `list_pages` to return the dashboard, kickoff notes, release checklist and team roadmap.
2. "Make an HTML page comparing three laptops for a student and publish it to Canvasbin." Expect `create_page` to return a view link and a PDF link.
3. "Open my weekly sales dashboard on Canvasbin and add a row for Northern Ireland with £8,400 and 210 orders." Expect `get_page`, then `create_draft`, then `publish_draft`, with the same link still working.
4. "Publish these meeting notes as Markdown to the Review Team space: …" Expect `list_teams` then `create_page` with the team.
5. "What comments are on the Team roadmap page? Reply that PDF export can move to October." Expect `list_comments` then `add_comment`.

OpenAI's review form accepts exactly these five positive cases. Do not add a sixth to the plugin ZIP.

### Should not use Canvasbin

1. "What's the capital of Portugal?"
2. "Write a Python function that reverses a string." (Code only, nothing to publish.)
3. "Save this note to my Google Drive."

## OpenAI plugin package

OpenAI takes a plugin ZIP. The source is in `plugins/openai/` (manifest, MCP config, onboarding skill); `plugins/openai/build.sh [out.zip]` adds the shared `skills/canvasbin` skill and the icons, then zips it. Bump `version` in `plugin.json` for each upload. The demo video URL lives in the package at `extensions.com.openai.review.demo_recording_url`. Reviewer credentials are entered in the dashboard, not the package.

## Grok Bot form

Submit at https://x.ai/bot/marketplace/submit-a-plugin. Checked 4 November 2026: the form is open, Grok itself is still marked coming soon, and Canvasbin is not on https://x.ai/bot/marketplace. Tick Grok Bot and Grok (coming soon). Leave Cursor unticked; that checkbox is a separate review from the Cursor Marketplace submission.

| Field | Value |
|---|---|
| Name | Stewart McKee |
| Work email | support@canvasbin.dev |
| Organisation | Active Information Design Limited |
| Organisation handle | `active-information-design` |
| Website | https://canvasbin.dev |
| Logo | https://canvasbin.dev/favicon.svg (square, indigo plate). Outer corners are transparent; fill them with `#4f46e5` first if review rejects that. |
| Server | `https://canvasbin.dev/mcp` |

Brief description (347 characters):

> Canvasbin publishes HTML and Markdown from a chat as a hosted page with a shareable link, a PDF and a private edit link. The assistant can update that same page later, so a weekly report or dashboard stays at the link people already have. Sign-in is OAuth. Pages use unguessable addresses, stay out of search engines, and run scripts in a sandbox.

Testing instructions (575 characters). The reviewer password is the one already supplied for Claude and ChatGPT. It is not stored here.

> Reviewer: review@canvasbin.dev. Use the password already supplied for the Claude and ChatGPT reviews. In Grok Bot, add the remote server https://canvasbin.dev/mcp and finish Canvasbin sign-in. 1. List my Canvasbin pages. Expect Weekly sales dashboard, Project kickoff notes, Release checklist and Team roadmap. 2. Make an HTML page comparing three laptops for a student and publish it to Canvasbin. Expect a view link and a PDF link that open. 3. Open my weekly sales dashboard and add Northern Ireland, £8,400 revenue, 210 orders. The same view link should show the new row.

Do not tick "tested locally and in-app" until those three prompts have been run in Grok Bot. Submitting accepts the Cursor Marketplace Publisher Terms. Section 3.1 requires the plugin to be usable at no cost, so the listing must not depend on a paid Canvasbin plan. The connector works on Free. The Grok Build catalog (`xai-org/plugin-marketplace`) is a separate pull-request index for repo plugins and is not this submission.

## Before submitting

- Test every tool with MCP Inspector (`npx @modelcontextprotocol/inspector`) against `https://canvasbin.dev/mcp`.
- Add Canvasbin as a custom connector in Claude and in ChatGPT developer mode, sign in with the reviewer account and run the test prompts above.
- ChatGPT domain verification: copy the token from the OpenAI platform and set `OPENAI_APPS_CHALLENGE` in the production stack `.env`. It is served at `https://canvasbin.dev/.well-known/openai-apps-challenge`.
- Don't mention prices in listing descriptions.
