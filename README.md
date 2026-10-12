# Canvasbin

Canvasbin hosts HTML or Markdown pages at unguessable URLs. There are no
accounts. Whoever has the write link can edit or delete the page. Whoever has
the read link can view it.

A page URL's extension picks the view:

| URL | What it is |
| --- | --- |
| `/{key}` | The app. A write key opens the editor. A read key is view-only. |
| `/{key}.html` | The rendered page. HTML scripts run in a sandbox. |
| `/{key}.pdf` | A PDF download of the rendered page. |
| `/{key}.md` | Markdown source, for Markdown pages. |
| `/raw/{key}` | Raw stored source. |

`create_page` (HTTP or MCP) returns `html_url`, `pdf_url`, `read_url`, and
`write_url`. Share `html_url` when people only need to read. Keep `write_url`
for collaborators: the key in it grants edit and delete.

The public instance is [canvasbin.mckee.family](https://canvasbin.mckee.family).
This repository is how you run your own.

## Run the Docker Hub image

Image: [`stewartmckee/canvasbin`](https://hub.docker.com/r/stewartmckee/canvasbin)
(`linux/amd64`). It includes the UI and headless Chromium for PDF export.

Save this as `compose.yaml`:

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

volumes:
  pgdata: {}
```

```bash
export POSTGRES_PASSWORD='choose-a-password'
export PUBLIC_BASE_URL='https://canvasbin.example.com'   # optional; see below
docker compose up -d
```

Open <http://localhost:8080>. Put a reverse proxy with TLS in front of port
8080 for anything other than local use, and set `PUBLIC_BASE_URL` to the
public origin (no trailing slash). When `PUBLIC_BASE_URL` is unset, links are
built from the request, including `X-Forwarded-Proto` and `X-Forwarded-Host`.

Pin an image tag instead of `latest` when you want upgrades to be deliberate.

Build the image yourself with `docker compose up --build` in a checkout. That
compose file uses a local Postgres password and is meant for development.

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

Page content is limited to 1 MiB. Creating pages is rate limited (60 per 10
minutes per client IP). PDF renders are limited too. Behind a proxy, set
`X-Forwarded-For` so limits apply to the caller rather than the proxy.

## MCP

A Streamable HTTP MCP server is mounted at `/mcp`. Point a client at
`https://your-host/mcp`. Tools:

- `create_page` — publish HTML or Markdown
- `get_page` — read a page by read key, write key, or URL
- `update_page` — replace content, title, or format (write key)
- `delete_page` — delete a page (write key)

Tools that take a key accept the bare key or any Canvasbin URL. HTML may be a
fragment or a full document.

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
