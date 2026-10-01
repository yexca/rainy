# Native API

The native API under `/api` is the JSON API used by Rainy's web app. It is not a
stable public API for third-party clients; use the [Subsonic API](subsonic.md)
for those. Endpoint tables, request bodies, and response shapes are in
[contract §7](contract.md#7-native-api-api--json-contract), and the matching
TypeScript types are in
[contract §8](contract.md#8-typescript-contract-websrclibapitypests).

## Authentication

- The web app signs in with `POST /api/auth/login`, which sets the HttpOnly
  `rainy_session` cookie (`SameSite=Lax`, `Secure` when the request arrived
  over HTTPS or with `X-Forwarded-Proto: https`).
- Any request may instead send `Authorization: Bearer <token>`, where the token
  is a session token or the user's API key.
- Sessions use a sliding expiry controlled by `RAINY_SESSION_TTL`.
- `GET /api/auth/status` reports whether the server is initialized.
  `POST /api/auth/setup` creates the first administrator and is rejected once
  any user exists.
- Failed sign-ins are rate limited in memory: 10 failures within 10 minutes
  for one client IP or one username lock that key for 5 minutes.

## Authorization

Three guards protect the routes:

| Guard | Applies to |
| --- | --- |
| Signed-in user | Browsing, streaming, covers, lyrics, favorites, ratings, playlists, queue, events, their own listening report and history, their own mixes and daily mix, and their own scrobbling accounts |
| Manager (`canManage` or administrator) | `/api/manage/*`: tags, covers, lyrics, rename, upload, link and online music downloads, delete, trash, doctor, folders, and history |
| Administrator | `/api/admin/*`: users, libraries, scans, server settings, music sources, scrobbling, statistics, system information, and cache |

Downloads additionally require the user's download permission and the
`enableDownloads` server setting. Only a playlist's owner or an administrator
can modify it; someone else's private playlist answers `404`.

## Conventions

- Errors use an HTTP status plus
  `{"error":{"code":"<code>","message":"<text>"}}`. The `readonly` code (409)
  means the music files are not writable by the server.
- Lists accept `offset`, `limit` (default 50, maximum 1000), `sort`, and
  `order`, and return `{"items":[...],"total":N}`.
- Every state-changing endpoint uses a non-GET method.
- Timestamps are unix milliseconds, and JSON arrays are never `null`.
- Batch management endpoints return `200` with per-item errors, so one
  read-only file does not hide the successful items.

## Media Endpoints

- `/api/stream/{trackId}` serves the original file with HTTP range support, or a
  chunked transcode when a format or bitrate is requested.
- `/api/cover/{coverArtId}` serves resized covers. Versioned IDs are cached as
  immutable, so a changed cover gets a new URL instead of a stale cache entry.
- `/api/events` is a server-sent event stream (`scan`, `library`,
  `nowPlaying`) with a keep-alive comment every 25 seconds and
  `X-Accel-Buffering: no` for nginx-based proxies.

## Changing the API

Change the Go handler, `web/src/lib/api/types.ts`, `endpoints.ts`, and the
contract in the same change. `internal/model/contract_test.go` and
`internal/app/contract_test.go` lock the Go JSON field names to the field
lists in the contract, so a renamed field fails the backend tests.

## Related Docs

- [Subsonic](subsonic.md)
- [Library management](library-management.md)
- [Frontend](frontend.md)
