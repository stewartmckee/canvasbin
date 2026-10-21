# Directory submissions

Listing copy and reviewer notes for the Claude Connectors Directory and the ChatGPT app directory. Keep this in step with the MCP server in `internal/mcpserver`.

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
| `create_page` | Create page | no | no | no | Publishes HTML or Markdown to the user's library or a team space. For a team page, the description tells the assistant to call `get_style_guide` first and follow it. |
| `get_page` | Get page | yes | no | no | Reads a page's title, format, content and links. |
| `update_page` | Update page | no | yes | no | Replaces a page's title or content. |
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
3. "Open my weekly sales dashboard on Canvasbin and add a row for Northern Ireland with £8,400 and 210 orders." Expect `get_page` then `update_page`, with the same link still working.
4. "Publish these meeting notes as Markdown to the Review Team space: …" Expect `list_teams` then `create_page` with the team.
5. "What comments are on the Team roadmap page? Reply that PDF export can move to October." Expect `list_comments` then `add_comment`.
6. "Set the Review Team style guide to short sentences and British English, then publish a one-page status update to that team." Expect `set_style_guide`, then `get_style_guide` before `create_page` with the team.

### Should not use Canvasbin

1. "What's the capital of Portugal?"
2. "Write a Python function that reverses a string." (Code only, nothing to publish.)
3. "Save this note to my Google Drive."

## OpenAI plugin package

OpenAI takes a plugin ZIP. The source is in `plugins/openai/` (manifest, MCP config, onboarding skill); `plugins/openai/build.sh [out.zip]` adds the shared `skills/canvasbin` skill and the icons, then zips it. Bump `version` in `plugin.json` for each upload. The demo video URL and reviewer credentials are entered in the dashboard, not the package.

## Before submitting

- Test every tool with MCP Inspector (`npx @modelcontextprotocol/inspector`) against `https://canvasbin.dev/mcp`.
- Add Canvasbin as a custom connector in Claude and in ChatGPT developer mode, sign in with the reviewer account and run the test prompts above.
- ChatGPT domain verification: copy the token from the OpenAI platform and set `OPENAI_APPS_CHALLENGE` in the production stack `.env`. It is served at `https://canvasbin.dev/.well-known/openai-apps-challenge`.
- Don't mention prices in listing descriptions.
