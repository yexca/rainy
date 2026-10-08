# Security

Rainy is a personal music server with accounts. Library data and media require
sign-in, and only managers and administrators can change files. Rainy is still
designed for home and small-group use: keep it on a trusted network or behind
HTTPS, and limit who can reach it.

This document is for operators. For vulnerability reporting, use the
[Security Policy](../../SECURITY.md); for implementation rules, use
[Secure development](../development/security.md).

## Network Exposure

The default Compose mapping publishes port 7650 on every host interface. To
reach Rainy only from the Docker host (for example, behind a local reverse
proxy), bind it to loopback in the Compose file:

```yaml
ports:
  - "127.0.0.1:7650:7650"
```

For remote access, prefer a VPN such as Tailscale or WireGuard, or a reverse
proxy with HTTPS; see [Reverse proxy](reverse-proxy.md). Do not forward port
7650 from your router over plain HTTP: passwords, session cookies, and Subsonic
credentials would cross the internet unencrypted.

## Accounts and Roles

- The first visitor to a new instance creates the administrator account. Finish
  setup before exposing the port to anyone else.
- Use strong, unique passwords. Rainy only enforces a minimum of 4
  characters, so choose much longer ones yourself.
- Give the **manager** permission only to people you trust with your files:
  managers can edit tags, rename, upload, and delete (to the trash).
- Downloads are governed by the per-user download permission and the global
  **Allow downloads** setting.
- An administrator can reset another user's password, which also signs that
  user out everywhere. The `rainy user reset-password` command does the same
  from the host.

## Sessions and Rate Limiting

Web sessions use an HttpOnly, `SameSite=Lax` cookie with a sliding lifetime of
`RAINY_SESSION_TTL` (default 30 days). The cookie is marked `Secure` when the
request arrived over HTTPS or with `X-Forwarded-Proto: https`.

Failed sign-ins are limited in memory: 10 failures within 10 minutes for one
client address or username lock it for 5 minutes. Subsonic authentication
allows 20 failures in the same window. Behind a reverse proxy, set
`RAINY_TRUST_PROXY=true` so the limit applies per real client instead of to the
proxy. Never enable it when clients can reach Rainy directly, because they
could then choose their own address.

## Subsonic Credentials

Subsonic clients send either the password or a salted MD5 token on every
request, often in the URL. Use HTTPS for any client outside your home network.
Prefer OpenSubsonic API keys where the client supports them: a key can be
revoked in **Settings → Subsonic apps** without changing the password.

## Stored Secrets

- `secret.key` encrypts stored passwords with AES-256-GCM. Passwords are
  reversible by design because Subsonic token authentication requires them.
  Anyone with both `rainy.db` and `secret.key` can recover every password.
- Session tokens and API keys are stored only as SHA-256 hashes.
- Restrict access to the data folder, and treat backups of it as sensitive.
  See [Database](database.md#backups).

## Filesystem and Container Boundaries

- Mount only your music at `/data` and a dedicated folder at `/config`. Never
  mount a host root, home directory, or the Docker socket.
- Every file operation is confined to the library roots, and libraries may not
  overlap the data directory.
- Mount the music read-only (`:ro`) if nobody should edit files through Rainy.
- The container drops root privileges to `PUID:PGID` after fixing `/config`
  ownership. Avoid `PUID=0`.
- Keep the image up to date; see [Docker](docker.md#upgrade).

## Known Limitations

- Cover images are checked for sign-in, not for per-playlist access. The cover
  mosaic of someone else's private playlist is visible to a signed-in user who
  knows its random ID.
- The service worker's cover cache survives sign-out on a shared device. Use a
  private browser profile on shared computers.

## Logs and Diagnostics

Logs may contain usernames, client addresses, library paths, and file names.
Redact them before sharing, and never post a database, `secret.key`, session
cookie, or API key in an issue. See [PRIVACY.md](../../PRIVACY.md).

## Related Docs

- [Configuration](configuration.md)
- [Reverse proxy](reverse-proxy.md)
- [Security Policy](../../SECURITY.md)
- [Security documentation map](../security/index.md)
