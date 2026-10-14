# Architecture

Rainy is organized around a single rule:

```text
The files on disk are the source of truth; the database is an index of them.
```

Tags, covers, and lyrics live in the music files and their sidecars. The
database mirrors them for fast browsing and adds what files cannot hold: users,
playlists, favorites, play counts, the play queue, settings, the trash, and the
edit history. Management features change the files first and then rescan the
affected paths, so the index always follows the disk.

## Architecture Topics

- [Core boundaries](core-boundaries.md): file ownership, stable identity,
  user state, filesystem effects, and process boundaries.
- [Contract](contract.md): the precise, section-numbered contract for the
  schema, Go package APIs, REST and Subsonic endpoints, TypeScript types, and
  the design system. It is the single source of truth.
- [Backend](backend.md): Go packages, dependency rules, and concurrency.
- [Frontend](frontend.md): the React app, routing, state, and PWA.
- [Data model](data-model.md): tables, identifiers, and derived data.
- [Native API](native-api.md): the `/api` JSON API used by the web app.
- [Subsonic](subsonic.md): the `/rest` Subsonic and OpenSubsonic API.
- [Library management](library-management.md): how file mutations stay safe.

## When To Read What

- Read [Core boundaries](core-boundaries.md) before changing identity,
  file effects, scanner decisions, user state, or permissions.
- Read the [contract](contract.md) before changing a schema, a package API, an
  endpoint, or a shared TypeScript type, and update it in the same change.
- Read [Data model](data-model.md) and [Migrations](../development/migrations.md)
  before editing SQL.
- Read [Library management](library-management.md) before adding any code that
  writes, moves, or deletes files.
- Read [Subsonic](subsonic.md) before changing anything a third-party client
  can observe.
- Read [Backend](backend.md) and [Frontend](frontend.md) before changing
  package layout or API/UI responsibilities.

## Runtime Topology

```text
Browser (web app / PWA)            Subsonic and OpenSubsonic clients
        |  /api (JSON, SSE)                 |  /rest (XML / JSON)
        +----------------+------------------+
                         |
                  Go HTTP server (chi)
                         |
   store (SQLite: reader pool + single writer)   scanner, manage, artwork,
                         |                        transcode (ffmpeg), lyrics
                         |                                |
                    /config (DB, secret.key,         library roots
                    cache, trash, tmp, ytdlp)      (for example /data)
```

One process serves the embedded web app, both APIs, and background scans.
Server-sent events on `/api/events` push scan progress, library changes, and
"now playing" updates to open browsers. There are no external services.

The server sends outbound requests only from four opt-in features, each off
until an administrator enables its server setting:

- Online metadata lookup (`internal/metasearch`, `onlineMetadata`) searches
  the catalogues and fetches lyrics and covers (from the providers' image
  hosts only) when a manager searches from the tag editor or picks a result.
- Link downloads (`internal/ytdlp`, `ytdlpEnabled`) contact YouTube or
  bilibili when a manager starts a download (through the yt-dlp process, plus
  resolving bilibili short links), and GitHub when an administrator checks for
  or installs a yt-dlp update.
- Online music (`internal/lxmusic`, `lxSourcesEnabled`) searches the online
  catalogues, fetches lyrics and covers, and downloads audio when a manager
  asks; it downloads source scripts when an administrator imports or refreshes
  one from a link, and runs administrator-imported lx-music source scripts that
  contact the servers their authors chose (public addresses only).
- Scrobbling (`internal/scrobble`, `lastfmEnabled` and `listenBrainzEnabled`)
  contacts Last.fm or ListenBrainz when a user links an account, and then
  sends that user's plays and "now playing" updates to the linked services and
  mirrors their favorites to Last.fm loved tracks.

## Related Docs

- [Overview](../overview.md)
- [Migrations](../development/migrations.md)
- [Backend guidelines](../development/backend-guidelines.md) and
  [Frontend guidelines](../development/frontend-guidelines.md)
- [Reliability](../operations/reliability.md)
- [ADR index](../decisions/index.md)
