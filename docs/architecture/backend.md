# Backend

The backend is a single Go module named `rainy`, built with `CGO_ENABLED=0`
(see [ADR-0001](../decisions/ADR-0001-pure-go-runtime.md)). The entry point is
`cmd/rainy`; everything else lives under `internal/`.

The precise package APIs are in [contract §5](contract.md#5-backend-contracts).
This page explains how the pieces fit together.

## Command Line

`cmd/rainy` starts the server by default and offers a few maintenance commands:

```text
rainy [serve]                                   start the server
rainy version                                   print version information
rainy user list                                 list users
rainy user reset-password <username> <password> set a new password
```

`reset-password` also signs the user out everywhere. Configuration comes only
from `RAINY_*` environment variables; see
[Configuration](../operations/configuration.md).

## Packages

| Package | Responsibility |
| --- | --- |
| `buildinfo` | Version, commit, and build date injected through `-ldflags` |
| `config` | Loads `RAINY_*` environment variables and derives data-directory paths |
| `db` | Opens SQLite, applies embedded migrations; `db/dbtest` gives tests a migrated temporary database |
| `model` | Shared structs whose JSON form is the native API shape |
| `util` | IDs, time, search normalization, cover-art IDs, MIME types, and `SafeJoin` |
| `store` | All shared SQL (the repository layer) |
| `auth` | Password encryption, sessions, API keys, middleware, and login rate limiting |
| `events`, `nowplaying` | In-process pub/sub for server-sent events, and the in-memory "now playing" list |
| `tags` | TagLib wrapper for reading and writing tags and pictures, plus mojibake repair |
| `scanner` | Library walks, scheduling, targeted rescans, and the library lock |
| `artwork` | Cover resolution, resizing, and the disk cache |
| `transcode`, `lyrics` | ffmpeg streaming and LRC parsing |
| `manage` | Tag editing, covers, lyrics, rename, upload, trash, the library doctor, and the edit log |
| `metasearch` | Opt-in online metadata lookup for the tag editor |
| `ytdlp` | Opt-in downloads from YouTube and bilibili: the yt-dlp binary (install, update), encrypted sign-in cookies, and yt-dlp runs |
| `lxmusic` | Opt-in online music: song search in the five lx-music catalogues and lx-music custom source scripts run in a sandboxed goja interpreter |
| `listening` | Listening reports from the play history: totals, the timeline, the hour-of-week clock, and top lists |
| `recommend` | Songs picked from the user's own library and plays: infinite-mode mixes and the stored daily mix |
| `scrobble` | Opt-in scrobbling to Last.fm and ListenBrainz: account linking, "now playing", loves, and a retrying queue of plays sent in the background |
| `app` | The dependency container that wires every service |
| `api`, `subsonic` | The native `/api` and the Subsonic `/rest` HTTP handlers |
| `server` | Router assembly, middleware, SPA serving, and graceful shutdown |

## Dependency Rules

- `model`, `util`, `config`, and `buildinfo` import nothing internal.
- `store` imports only `db`, `model`, and `util`.
- Services import `store` but never `api`, `subsonic`, `server`, or `app`.
- Only `app` knows every service; handlers receive the `App` container.

The full conventions (IDs, timestamps, paths, logging, errors, context) are in
[contract §3](contract.md#3-conventions).

## Request Flow

The router in `server` mounts three areas:

- `/api/*`: the native API. The auth middleware resolves the session cookie or
  bearer token (session token or API key) into a user; per-route guards then
  require a user, a manager, or an administrator.
- `/rest/*`: the Subsonic API with its own authentication and error codes.
- `/*`: the embedded single-page app, with long-lived caching for hashed
  assets and `no-cache` for the shell, manifest, and service worker.

`GET /api/health` is public and used by the Docker health check. Compressible
responses are gzip-encoded; audio, images, and event streams never are.

## Database Access

`db.Open` returns two pools on the same SQLite file: a read-only reader pool and
a single-connection writer that begins transactions immediately. Funnelling all
writes through one connection avoids `SQLITE_BUSY` storms, and WAL mode lets
reads continue during writes. See [Database](../operations/database.md) for the
pragmas.

## Scanning and Background Work

The scanner walks each library with a worker pool that reads tags in parallel
and a single writer that upserts tracks in batches. After a scan it refreshes
album, artist, and genre aggregates, resolves folder covers, and publishes
`scan` and `library` events. `Schedule` runs periodic quick scans using the
interval stored in settings.

A library lock serializes scans and file mutations. Management operations hold
it across the file change and the targeted rescan that follows, so a scan never
observes a half-finished rename. Details are in
[contract §5.8](contract.md#58-scanner-owner-scanner-agent) and
[§13](contract.md#13-wave-2-implementation-notes).

## Shutdown

On `SIGINT` or `SIGTERM`, the server stops accepting connections, cancels
request contexts shortly after shutdown begins (ending event streams), stops
the scan scheduler, and closes the database last so the WAL is checkpointed.

## Related Docs

- [Frontend](frontend.md)
- [Data model](data-model.md)
- [Testing](../development/testing.md)
- [Secure development](../development/security.md)
