# Secure Development

Rainy has direct write access to an operator's music collection. Security work
must therefore protect both the usual web boundaries and the files themselves.
Read the repository [Security Policy](../../SECURITY.md) and
[Deployment security](../operations/security.md) before changing
authentication, file handling, or anything reachable without sign-in.

This document is for contributors changing the codebase. Operators should use
[Deployment security](../operations/security.md), and vulnerability reporters
should use the [Security Policy](../../SECURITY.md).

## Trust Boundaries

- Unauthenticated clients may reach only the app shell, `GET /api/health`,
  `GET /api/auth/status`, sign-in, sign-out, and `POST /api/auth/setup` while
  no user exists. Everything else requires an authenticated user.
- Listeners may read the library and change only their own state (favorites,
  ratings, playlists, queue, bookmarks, profile, password, and API key).
- Managers may change files through `/api/manage`. Administrators also manage
  users, libraries, scans, and server settings.
- Library roots and the data directory are trusted operator mounts. Paths from
  requests, tags, uploads, archive names, and the database remain untrusted.
- Outbound requests exist only in `internal/metasearch`, behind the
  admin-only `onlineMetadata` setting (off by default) and the manager role,
  and in `internal/ytdlp`, behind the admin-only `ytdlpEnabled` setting (off by
  default). yt-dlp runs with a fixed argument list (`--ignore-config`,
  `--no-plugin-dirs`, no generic extractor, the link last after `--`), only for
  links to the allow-listed YouTube and bilibili hosts, and writes only into
  its work directory; its binary comes from the official GitHub release,
  verified against `SHA2-256SUMS`, and never from a URL taken from an API
  response. Sign-in cookies are credentials: keep them encrypted, out of every
  API response, log line, and error message, and give each run a private copy
  that is deleted with its work directory.
  Use fixed endpoint URLs, pass user input only as an encoded parameter,
  validate provider ids before requesting, cap response sizes, and fetch covers
  only from the allow-listed provider image hosts. Responses from these
  services are untrusted text. Any new outbound host needs a
  [PRIVACY.md](../../PRIVACY.md) update and an entry in
  `scripts/privacy-allowlist.json`.

## Filesystem Operations

- Build every absolute path with `util.SafeJoin(root, rel)`. It rejects
  absolute input, `..`, NUL bytes, and anything outside the root. Use the
  symlink-aware helpers (`EnsureWithinRoot`, `IsWithin`) before reading or
  moving images and other files found by pattern.
- Never touch files outside a library root or the data directory. Libraries
  may not overlap the data directory.
- Hold the scanner's library lock across a mutation and its rescan, write an
  `edit_log` row, and move deletions to the trash instead of removing files.
  See [Library management](../architecture/library-management.md).
- Treat a read-only mount as a normal condition and return the `readonly`
  error.
- Bound resource use: uploads have a per-request cap, and image decoding,
  resizing, and ffmpeg processes must stop when the request is cancelled.
- Pass file paths to ffmpeg as absolute `file:` URLs and never through a shell.

## Authentication and Authorization

- Enforce permissions in the backend with the auth guards. A hidden button is
  not authorization.
- Session tokens and API keys are credentials. Store only their SHA-256 hash,
  and never place them in URLs (Subsonic query authentication is the protocol's
  own exception), logs, or error messages.
- Passwords are stored encrypted with `secret.key` because Subsonic token
  authentication needs them. Never log or return them, and never weaken the
  AES-GCM encryption or the key's file permissions.
- Keep the sign-in and Subsonic rate limits in place. Behind a proxy they
  depend on `RAINY_TRUST_PROXY`, which trusts only `X-Real-IP` or the last
  `X-Forwarded-For` hop.
- State-changing endpoints must use a non-GET method so the `SameSite=Lax`
  session cookie cannot be used for cross-site writes.

## Responses and Content

- Serve user-provided images only with raster image content types and
  `X-Content-Type-Options: nosniff`; anything else is served as
  `application/octet-stream`.
- Tags, lyrics, file names, and playlist names are untrusted text. Render them
  through React, never as raw HTML.
- API errors use stable codes and short messages. Log details server-side at
  an appropriate level; do not return stack traces or host paths to clients.

## Dependencies and Build Integrity

- Keep `CGO_ENABLED=0` and pure-Go dependencies; see
  [ADR-0001](../decisions/ADR-0001-pure-go-runtime.md).
- After changing Go modules, run `go mod tidy`, `make backend-verify`, the
  backend tests, and `make backend-vuln`.
- After changing the web lockfile, run `make frontend-audit`.
- Pin third-party GitHub Actions to full commit SHAs and give workflows the
  least permissions they need.

## Secrets and Public Data

- Never commit `.env` files with real values, databases, `secret.key`, logs,
  media, or personal paths. `.gitignore` is a convenience, not a secrecy
  boundary.
- Use reserved examples in code, tests, and docs: `example.com` or `.invalid`
  domains, `192.0.2.0/24` addresses, `/path/to/music`, and placeholder
  credentials. See [Testing](testing.md#test-data).
- Run `make sensitive-check` before committing and review the staged diff.

## Review Checklist

For a security-relevant change, record:

1. The asset and trust boundary affected.
2. Which roles may perform the action and which must be refused.
3. The failure behavior and whether it fails closed.
4. The smallest regression tests for the boundary.
5. Whether [SECURITY.md](../../SECURITY.md), deployment guidance, or
   [PRIVACY.md](../../PRIVACY.md) must change.

Do not publish exploit details for an undisclosed vulnerability in a normal
issue or pull request. Use GitHub Private Vulnerability Reporting as described
in the Security Policy.

## Related Documentation

- [Backend](../architecture/backend.md)
- [Testing](testing.md)
- [Privacy and data handling](../../PRIVACY.md)
