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
                    cache, trash, tmp)             (for example /data)
```

One process serves the embedded web app, both APIs, and background scans.
Server-sent events on `/api/events` push scan progress, library changes, and
"now playing" updates to open browsers. There are no external services, and
the server sends no outbound requests unless an administrator enables the
opt-in online metadata lookup (`internal/metasearch`), which only runs when a
manager searches from the tag editor, downloads from links
(`internal/ytdlp`), which contact YouTube or bilibili only when a manager starts
a download and GitHub only when an administrator checks for or installs a
yt-dlp update, or online music (`internal/lxmusic`), which searches the online
catalogues when a manager asks and runs administrator-imported lx-music source
scripts that contact the servers their authors chose, or scrobbling
(`internal/scrobble`), which sends the plays of users who linked a Last.fm or
ListenBrainz account to that service.

## Related Docs

- [Overview](../overview.md)
- [Migrations](../development/migrations.md)
- [Backend guidelines](../development/backend-guidelines.md) and
  [Frontend guidelines](../development/frontend-guidelines.md)
- [Reliability](../operations/reliability.md)
- [ADR index](../decisions/index.md)
