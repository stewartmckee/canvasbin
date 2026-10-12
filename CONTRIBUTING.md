# Contributing

Issues and pull requests are welcome. For a security problem, use the process
in [SECURITY.md](SECURITY.md) instead of a public issue.

## Issues

Open an issue from the templates:

- [Bug report](https://github.com/stewartmckee/canvasbin/issues/new?template=bug_report.yml)
- [Feature request](https://github.com/stewartmckee/canvasbin/issues/new?template=feature_request.yml)

A useful report says what you expected, what happened, and how to reproduce
it (commands, image tag, and browser if the UI is involved). Search
[open issues](https://github.com/stewartmckee/canvasbin/issues) first so the
same report is not filed twice.

Questions about running your own copy belong in a bug report if something
failed, or a feature request if the project does not do it yet.

## Pull requests

1. Fork the repository and create a branch from `main`.
2. Keep the change focused. A bug fix and an unrelated rewrite are easier to
   review as two pull requests.
3. Run the checks below.
4. Describe why the change is needed in the pull request. Link the issue if
   there is one.

## Development setup

You need Go 1.27, Node.js 24, and Docker (for Postgres). Chromium or Chrome
is required only if you exercise PDF export outside the published image.

```bash
docker compose up postgres
```

Postgres is published on `localhost:5433`. In one terminal:

```bash
DATABASE_URL='postgres://canvasbin:canvasbin@localhost:5433/canvasbin?sslmode=disable' \
  HTTP_ADDR=:8090 \
  go run ./cmd/canvasbin
```

In another:

```bash
cd web
npm ci
npm run dev
```

The UI is at <http://localhost:5173> and proxies API calls to port 8090.

To run the whole stack in Docker, including a production-style build of this
checkout:

```bash
docker compose up --build
```

That serves the app at <http://localhost:8080>.

## Checks

```bash
go test ./...
cd web && npm ci && npm run build
```
