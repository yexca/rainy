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
- Outbound requests leave the server only for the optional features described
  in [Outbound Requests](#outbound-requests).

## Outbound Requests

Rainy makes no outbound request unless an administrator turns on the setting
of the feature that needs it; every such setting is off by default. There is
no shared outbound transport or global egress policy: each package below
builds its own `http.Client` from a clone of Go's default transport, so the
environment's `HTTPS_PROXY`/`HTTP_PROXY` apply, and adds its own controls.

| Package | Admin setting | Destinations | Package-specific controls |
| --- | --- | --- | --- |
| `internal/metasearch` (online metadata in the tag editor; managers search) | `onlineMetadata` | Fixed API URLs of NetEase Cloud Music, QQ Music, Kugou, Kuwo, and the iTunes Search API | Redirects stay on the same host (at most three); covers are fetched only from the provider image hosts in `coverHosts` |
| `internal/ytdlp` (link downloads; managers submit links, administrators install yt-dlp) | `ytdlpEnabled` | YouTube and bilibili through yt-dlp; GitHub for the yt-dlp binary; `b23.tv` share links | See below |
| `internal/lxmusic` (online music; administrators import source scripts) | `lxSourcesEnabled` | Catalogue search APIs, script links, and whatever source scripts request | Public internet addresses only, checked on the URL, after DNS resolution on every dialled address, and on every redirect (at most five); a configured proxy is exempt from the address check |
| `internal/scrobble` (Last.fm and ListenBrainz) | `lastfmEnabled`, `listenBrainzEnabled` | The fixed Last.fm and ListenBrainz API URLs | Redirects are never followed |

Every outbound client keeps these rules:

- Bound each request: a dial timeout plus a request or response-header
  timeout, and read every response body through a size cap.
- Use fixed endpoint URLs and pass user input only as an encoded parameter.
  Validate provider and song ids before they reach a request.
- Report failures by host. Errors drop Go's `*url.Error` wrapper, whose
  message repeats the full URL and may carry tokens.
- Treat every response as untrusted text.
- Add any new outbound host to [PRIVACY.md](../../PRIVACY.md) and, when it
  appears in tracked files, to `scripts/privacy-allowlist.json`.

yt-dlp runs with a fixed argument list (`--ignore-config`,
`--no-plugin-dirs`, the generic extractor disabled, the link last after `--`),
only for links whose host exactly matches the YouTube and bilibili allow list,
and writes only into its work directory. Rainy resolves a `b23.tv` share link
itself without following the redirect and accepts it only when it points at a
bilibili video. The yt-dlp binary comes from the official GitHub release:
Rainy reads the latest tag from the GitHub API, builds the download URLs from
that validated tag instead of following a URL from the response, follows
redirects only over HTTPS to GitHub hosts, and verifies the binary against
`SHA2-256SUMS`. Sign-in cookies are credentials: keep them encrypted, out of
every API response, log line, and error message, and give each run a private
copy that is deleted with its work directory.

lx-music source scripts are third-party code imported by an administrator.
They run in a goja interpreter with only the lx-music API (no `require`, file
system, processes, or `fetch`); every run of script code is interrupted after
10 s and the interpreter discarded; and request bodies and responses are
bounded. Memory use is not bounded, which is why only administrators can
import scripts. No endpoint returns a script, and songs sent back by clients
are validated before their ids reach a script.

Scrobbling sends only the plays, "now playing" updates, and loved tracks of
users who linked an account. The browser, never the server, opens the Last.fm
sign-in page, and the returned token is accepted only with the random `state`
issued to the same user. Last.fm session keys, ListenBrainz tokens, and the
Last.fm shared secret are credentials: keep them encrypted with `secret.key`,
out of every API response, log line, and error message.

## Filesystem Operations

- Build every absolute path with `util.SafeJoin(root, rel)`. It rejects
  absolute input, `..`, NUL bytes, and anything outside the root. Use the
  symlink-aware helpers (`EnsureWithinRoot`, `IsWithin`) before reading or
  moving images and other files found by pattern.
- Never touch files outside a library root or the data directory. A library
  may not overlap the data directory or another library.
- Hold the scanner's library lock across a mutation and its rescan, write an
  `edit_log` row, and move deletions to the trash instead of removing files.
  See [Library management](../architecture/library-management.md).
- Treat a read-only mount as a normal condition and return the `readonly`
  error.
- Bound resource use: uploads stream to the data directory's `tmp` folder
  under per-file, per-request, and file-count limits; image decoding,
  resizing, and ffmpeg processes stop when the request is cancelled.
- Run ffmpeg and ffprobe with a fixed argument list and the input as an
  absolute `file:` URL, never through a shell and never with user input as an
  option.

## Authentication and Authorization

- Enforce permissions in the backend with the auth guards. A hidden button is
  not authorization.
- Session tokens and API keys are credentials. Store only their SHA-256 hash,
  and never place them in URLs (Subsonic query authentication is the protocol's
  own exception), logs, or error messages.
- Passwords are stored encrypted with `secret.key` because Subsonic token
  authentication needs them. Never log or return them, and never weaken the
  AES-GCM encryption or the key's file permissions.
- `RAINY_MODE=development` removes authentication from the native API. It is
  for the loopback-only development stack; never enable it in the production
  Compose file, the image defaults, or any example meant for operators.
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
- Keep CI commands in Makefile targets and follow the fail-closed result plan
  in [CI and release automation](ci.md). Only planned-out jobs may be skipped;
  release publication requires VERSION, release notes, main ancestry, and
  successful main CI for the exact commit.

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
