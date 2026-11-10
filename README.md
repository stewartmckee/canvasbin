# Canvasbin

**Pastebin for the agentic age.**

Canvasbin was built for sharing what agents produce. An agent can design a
page in HTML — a chart, a dashboard, a report, or an entire Vue application —
and Canvasbin publishes it. HTML or Markdown in, a link out. No deploy, no
CMS. Agents publish; humans share.

Use the hosted service at [canvasbin.dev](https://canvasbin.dev), where you
sign in once and your pages live in a library you can share, or run your own
copy with accounts off and no sign-in at all.

The link is the permission. Share the read link. Keep the write link. Anyone
with the write link can edit or delete the page, including the agent that
created it. Ask again and the same URL updates in place.

`create_page` returns the write key to the agent, so that key is in the chat
history. Share `html_url` from the transcript. The write key stays there so
the agent can edit the page later; treat the transcript like the editor.

## What you get

| Link | What a person sees |
| --- | --- |
| `html_url` | The rendered page. This is the one to send. |
| `pdf_url` | The same page as a PDF. |
| `read_url` | The app, view-only: preview and source. |
| `write_url` | The editor. Returned to the agent, so it is stored in the chat history. |

The address itself picks the view:

| URL | What it is |
| --- | --- |
| `/{key}` | The app. A write key opens the editor. A read key is view-only. |
| `/{key}.html` | The rendered page. HTML scripts run in a sandbox. |
| `/{key}.pdf` | A PDF download. |
| `/{key}.md` | Markdown source, for Markdown pages. |
| `/raw/{key}` | The stored source. |

Pages can be a fragment or a full HTML document. Markdown is rendered for
you. Updates are live on the same URLs, so an agent can revise a page
without minting a new link.

### Agent scratchpads (`agentic_only`)

Agents can create **agentic-only** pages: external scratchpads for context shared
across MCP clients or backups of what matters from a chat. Set `agentic_only:
true` on `create_page` or a draft. They stay out of the web library unless
the user turns on “Show agent scratchpads”; `list_pages` always includes them
with the flag set. Passing `team` with `agentic_only: true` stores a personal
page, not a team page. See
[Share context across platforms](https://canvasbin.dev/agent-scratchpad/share-context-across-platforms)
and [Backup AI chats](https://canvasbin.dev/agent-scratchpad/backup-ai-chats).

## Hand it to an agent

Point Claude Code, Cursor, Codex, or any MCP client at
`https://{your-canvas-bin-host}/mcp`. There is no API key: a self-hosted copy
with accounts off needs no sign-in, and canvasbin.dev asks you to sign in once
through the client. In Cursor, install the Canvasbin plugin instead. Ask for a
page:

> Build a dashboard of this week's numbers and publish it on Canvasbin. Give me the link.

You get `html_url` to forward. The agent also receives `write_url` and
`write_key`, and those stay in the chat history for the next edit. Setup for
each client is below.

Examples use `https://{your-canvas-bin-host}`. Replace that with the origin
where you run Canvasbin. On your own machine that is `http://localhost:8080`.

## Run it

Self-hosting is the default. Accounts stay off: there is no sign-in, and a
page is created with its read and write links. You need Docker. Postgres
stores the pages. CircleCI publishes the app image on each push to `main`
as [`stewartmckee/canvasbin`](https://hub.docker.com/r/stewartmckee/canvasbin)
(`linux/amd64`). It includes the editor and headless Chromium for PDF
export. Each build also tags `0.1.<build number>`.

`ACCOUNTS` defaults to `off`. Leave it that way for a private bin.

### Docker Compose

This is the sample to run. The same file is
[`examples/compose.yaml`](examples/compose.yaml). It is not
`docker-compose.yml`, which is only for developing this repository.

```yaml
name: canvasbin

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: canvasbin
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD}
      POSTGRES_DB: canvasbin
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U canvasbin -d canvasbin"]
      interval: 5s
      timeout: 5s
      retries: 12

  canvasbin:
    image: stewartmckee/canvasbin:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://canvasbin:${POSTGRES_PASSWORD}@postgres:5432/canvasbin?sslmode=disable
      HTTP_ADDR: ":8080"
      PUBLIC_BASE_URL: ${PUBLIC_BASE_URL:-http://localhost:8080}
      ACCOUNTS: "off"
    # Chromium's sandbox needs namespaces Docker's default seccomp profile blocks.
    # Without this, PDF export fails instead of running the browser unsandboxed.
    security_opt:
      - seccomp:unconfined

volumes:
  pgdata: {}
```

```bash
export POSTGRES_PASSWORD='choose-a-password'
docker compose -f examples/compose.yaml up -d
```

Open <http://localhost:8080>.

### Docker Run

The same two containers without a compose file. `restart` brings the app
back up if Postgres is not ready on the first start.

```bash
docker network create canvasbin
docker volume create canvasbin-pg

docker run -d --name canvasbin-db --network canvasbin --restart unless-stopped \
  -e POSTGRES_USER=canvasbin \
  -e POSTGRES_PASSWORD=choose-a-password \
  -e POSTGRES_DB=canvasbin \
  -v canvasbin-pg:/var/lib/postgresql/data \
  postgres:16-alpine

docker run -d --name canvasbin --network canvasbin --restart unless-stopped \
  -p 8080:8080 \
  -e DATABASE_URL='postgres://canvasbin:choose-a-password@canvasbin-db:5432/canvasbin?sslmode=disable' \
  -e HTTP_ADDR=:8080 \
  -e PUBLIC_BASE_URL=http://localhost:8080 \
  -e ACCOUNTS=off \
  --security-opt seccomp=unconfined \
  stewartmckee/canvasbin:latest
```

Open <http://localhost:8080>.

For anything other than local use, put a TLS reverse proxy in front and set
`PUBLIC_BASE_URL` to that `https` origin with no trailing slash. Do not
publish port 8080 on a public interface; only the proxy should reach the
app. The proxy must append the caller to `X-Forwarded-For`. Canvasbin uses
the rightmost address, and only when the TCP peer is a private or loopback
address.

Pin `0.1.<build number>` instead of `latest` when you want upgrades to be deliberate.

To build the image from this checkout instead of pulling it:

```bash
docker build -t stewartmckee/canvasbin:local .
```

Use that name in the `docker run` above. `docker compose up --build` in the
repository root is the development stack, not this sample.

## Develop from source

Requirements: Go 1.27, Node.js 24, Docker.

```bash
git clone https://github.com/stewartmckee/canvasbin.git
cd canvasbin
docker compose up postgres
```

API (proxied by the Vite dev server):

```bash
DATABASE_URL='postgres://canvasbin:canvasbin@localhost:5433/canvasbin?sslmode=disable' \
  HTTP_ADDR=:8090 \
  go run ./cmd/canvasbin
```

UI:

```bash
cd web
npm ci
npm run dev
```

Open <http://localhost:5173>. PDF export looks for Chrome or Chromium on
`PATH`, or the binary in `CHROME_BIN`. The published image sets
`CHROME_BIN=/usr/bin/chromium-browser`.

```bash
go test ./...
cd web && npm run build
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://canvasbin:canvasbin@localhost:5432/canvasbin?sslmode=disable` | Postgres connection string. The schema is applied on startup. |
| `HTTP_ADDR` | `:8080` | Listen address. |
| `PUBLIC_BASE_URL` | request host | Origin used in returned links. |
| `CHROME_BIN` | Chrome/Chromium on `PATH` | Browser used to print PDFs. |
| `ACCOUNTS` | off | Leave `off` for a self-hosted bin with no sign-in. `on` adds email signup and a library, and then requires `MAILGUN_SENDING_KEY` and an `https` `PUBLIC_BASE_URL` (`http://localhost` is allowed). |
| `MAILGUN_SENDING_KEY` | empty | Required only when `ACCOUNTS=on`. Mailgun EU sending API key. |
| `MAILGUN_DOMAIN` | `canvasbin.dev` | Mailgun sending domain. Used only when accounts are on. |
| `MAIL_FROM` | `Canvasbin <noreply@canvasbin.dev>` | From address for verification and reset mail. |
| `ADMIN_EMAILS` | empty | Comma-separated emails allowed to use the admin user list. Accounts only. |

Copy [`.env.example`](.env.example) for a local env file. Never commit real secrets.

Page content is limited to 1 MiB. Creating pages is rate limited (60 per 10
minutes per client IP). PDF renders are limited too. Behind a proxy, append
the caller to `X-Forwarded-For` (`$proxy_add_x_forwarded_for`). Canvasbin
uses the rightmost address in that header, and only when the connection
comes from a private or loopback address, so a client cannot choose a fresh
rate-limit bucket by sending its own value. Do not publish the app port on
a public interface; the proxy should be the only thing that can reach it.

### Accounts (optional)

With `ACCOUNTS=off` (the default), Canvasbin stays authless: create a page, share
the read link, keep the write link.

With `ACCOUNTS=on`, people can register with email and password. Mailgun sends
the verification link, so `MAILGUN_SENDING_KEY` and `PUBLIC_BASE_URL` are
required. Signed-in users get a library and can claim a write-key page.
Creating a page then requires signing in; existing pages keep working by
their read and write links without an account. The Docker samples leave
accounts off.

## Connect an agent

Canvasbin speaks MCP over Streamable HTTP. The write key from `create_page` is
what lets an agent edit or delete that page later.

The endpoint is `https://{your-canvas-bin-host}/mcp`. On your own machine
that is `http://localhost:8080/mcp`. With accounts off, no API key or login is
required. With accounts on, every MCP request needs a signed-in account
(OAuth), so clients prompt for sign-in when they connect. Page links in the
browser keep working by key without signing in.

### Sign in from Claude, ChatGPT or Cursor

Point the connector at `https://canvasbin.dev/mcp` (or your hosted origin).
The first request returns HTTP 401 with a `WWW-Authenticate` challenge.
The client opens Canvasbin’s OAuth consent screen; after you approve, it
retries with a bearer token. Discovery documents live at
`/.well-known/oauth-protected-resource/mcp` and
`/.well-known/oauth-authorization-server`.

### Cursor plugin

This repository is also a Cursor plugin (`.cursor-plugin/plugin.json`). The
Marketplace install always connects Cursor to `https://canvasbin.dev/mcp`
and adds a `canvasbin` skill that tells the agent when to publish a page and
which link to hand back. Sign-in is OAuth on that host: Cursor opens the
Canvasbin consent screen the first time, and the plugin never sees the
account password. To use your own host instead, configure the server by hand
as shown under Other clients.

### Claude Code

Add it for every project on this machine:

```bash
claude mcp add --transport http canvasbin --scope user \
  https://{your-canvas-bin-host}/mcp
```

Or only the project you are in (the default scope):

```bash
claude mcp add --transport http canvasbin \
  https://{your-canvas-bin-host}/mcp
```

Check it with `claude mcp list`, then `/mcp` inside a session. Ask Claude to
publish a page. A typical request:

> Create a Canvasbin page that explains this change, and give me the link to share.

`create_page` returns four links, plus the keys. `html_url` is the one to
share. `write_url` and `write_key` come back to Claude and remain in the
chat history, which is what lets a later turn call `create_draft` and `publish_draft` without
opening the browser.

To share the server with everyone who clones a repository, commit a
`.mcp.json` in that repository:

```json
{
  "mcpServers": {
    "canvasbin": {
      "type": "http",
      "url": "https://{your-canvas-bin-host}/mcp"
    }
  }
}
```

Claude Code asks before it uses a project `.mcp.json`. `"type": "http"` is
required. `streamable-http` is accepted as an alias. This repository commits
that file at the root, pointed at `https://canvasbin.dev/mcp`, so
cursor.directory can find the server.

### Other clients

No headers are required.

**Cursor.** In `~/.cursor/mcp.json` (all projects) or `.cursor/mcp.json` (one
project):

```json
{
  "mcpServers": {
    "canvasbin": {
      "url": "https://{your-canvas-bin-host}/mcp"
    }
  }
}
```

**Codex.**

```bash
codex mcp add canvasbin --url https://{your-canvas-bin-host}/mcp
```

That writes this into `~/.codex/config.toml`:

```toml
[mcp_servers.canvasbin]
url = "https://{your-canvas-bin-host}/mcp"
```

**Claude Desktop.** Add a custom connector and set the URL to
`https://{your-canvas-bin-host}/mcp`. Desktop does not take a remote URL in
`claude_desktop_config.json`.

**VS Code.** In `.vscode/mcp.json`:

```json
{
  "servers": {
    "canvasbin": {
      "type": "http",
      "url": "https://{your-canvas-bin-host}/mcp"
    }
  }
}
```

### Tools

- `create_page` — publish HTML or Markdown. Returns `html_url`, `pdf_url`, `read_url`, `write_url`, and the keys. When publishing into a team, the agent is asked to read that team's style guide first.
- `get_page` — read a page by read key, write key, or URL.
- `create_draft` — store one unpublished draft (content, and title or format when those change). Needs the write key. Does not change the live page.
- `publish_draft` — copy that draft onto the live page when `expected_bytes` matches the stored draft. A draft under half the live page also needs `confirm_shrink`.
- `delete_page` — delete a page. Needs the write key.
- `get_style_guide` / `set_style_guide` — read, create, or update a team's style guide. These are registered only when accounts are on. Any member of that team can create the guide or replace it later. `create_page` tells the agent to call `get_style_guide` before writing a team page.

Tools that take a key accept the bare key or any Canvasbin URL. HTML may be a
fragment or a full document. Scripts in the rendered page run in a sandbox.

## Issues

Use the issue templates so reports are actionable:

- [Bug report](https://github.com/stewartmckee/canvasbin/issues/new?template=bug_report.yml) — what you expected, what happened, how to reproduce it, and the image tag or commit.
- [Feature request](https://github.com/stewartmckee/canvasbin/issues/new?template=feature_request.yml) — the job you are trying to do, and the behaviour you want.

Search [existing issues](https://github.com/stewartmckee/canvasbin/issues)
before opening a new one. Security reports go through
[private advisories](https://github.com/stewartmckee/canvasbin/security/advisories/new),
not a public issue. See [SECURITY.md](SECURITY.md) and
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE).
