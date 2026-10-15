# Security

Canvasbin pages are unguessable links. A read key lets anyone view a page; a
write key is the secret that can change or delete it. Treat write URLs like
passwords.

With accounts on (as on canvasbin.dev), people sign in with email and password,
and MCP clients connect with OAuth 2.1 (PKCE, dynamic client registration).
Agents get bearer tokens scoped to `pages:read` / `pages:write`. Existing page
links keep working by key without signing in.

## Reporting a vulnerability

Please report security issues privately through
[GitHub Security Advisories](https://github.com/stewartmckee/canvasbin/security/advisories/new).

Do not open a public issue for a vulnerability. Include what you found, how
to reproduce it, and the impact if you know it.

## What to expect

You should get an acknowledgement within a few days. Fixes for confirmed
issues land in this repository before any public write-up.
