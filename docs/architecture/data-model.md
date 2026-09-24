# Data Model

Rainy stores its state in one SQLite database at `<data>/rainy.db`. The schema
is defined by the numbered SQL files in `internal/db/migrations/`, which are
part of the [contract](contract.md#51-database). The Go structs that mirror the
rows are in [contract §5.2](contract.md#52-models-internalmodel).

## Conventions

- Timestamps are `INTEGER` unix milliseconds (UTC). `0` means never or unknown.
- Durations are floating-point seconds.
- Booleans are `INTEGER` `0` or `1`.
- Paths are library-relative and use forward slashes. Only the `libraries`
  table stores an absolute OS path (the library root). Absolute paths are
  built at runtime with `util.SafeJoin`.
- Text columns are `NOT NULL DEFAULT ''` unless `NULL` carries meaning (for
  example, `starred_at` is `NULL` when an item is not starred).

## Identifiers

| Entity | ID form | Why |
| --- | --- | --- |
| Tracks, users, playlists, radio stations, trash entries | Random 22-character base62 (`util.NewID`) | Opaque and stable once assigned |
| Albums, artists, genres | Deterministic hash of normalized names (`util.HashID`) | Survive a full rescan or a database rebuild without breaking favorites or playlists |
| Libraries | Integer | Small, operator-managed list |

A track keeps its ID when the scanner detects that a file was moved (a new
file matching a vanished track by size, duration, and title), and when Rainy
itself renames the file. Favorites, ratings, playlists, and play history
therefore survive reorganizing a library.

## Tables

| Area | Tables | Notes |
| --- | --- | --- |
| Library index | `libraries`, `tracks`, `albums`, `artists`, `genres`, `track_genres` | Rebuilt from files by the scanner |
| Accounts | `users`, `sessions` | Passwords are stored encrypted (see below); sessions store a SHA-256 hash of the token |
| Per-user state | `annotations`, `play_history`, `playlists`, `playlist_tracks`, `play_queues`, `bookmarks` | Cascade-deleted with the user |
| Server state | `settings`, `radio_stations` | Settings are key/value rows over `model.Settings` defaults |
| Management | `edit_log`, `trash` | The history of file changes and the restorable trash |
| Migrations | `schema_migrations` | One row per applied migration file |

## Derived Data

Album, artist, and genre rows are aggregates of their non-missing tracks.
`RefreshAlbums`, `RefreshArtists`, and `RefreshGenres` recompute names, counts,
durations, sizes, and cover references after every scan or file change, and
delete aggregates that no longer have tracks. Do not write aggregate columns
directly.

`UpsertTracks` owns the derived track columns (`dir`, `filename`, `suffix`, the
joined `genre`, and `search_text`); callers set the source fields only.

## Missing Tracks

A track whose file is no longer seen is marked `missing` instead of being
deleted, so a temporarily unavailable disk or share does not destroy user
state. Missing tracks are hidden from browsing and from playlists (playlist
entries keep their position and reappear if the file returns). A manager can
purge them from the library doctor.

If a library root is missing, unreadable, or suddenly empty while the database
still holds tracks for it, the scanner aborts that library instead of marking
everything missing.

## Passwords and Keys

Subsonic token authentication sends `md5(password + salt)`, which requires the
server to know each plain password. Rainy therefore stores passwords encrypted
with AES-256-GCM using `<data>/secret.key`, not as one-way hashes. API keys and
session tokens are stored only as SHA-256 hashes. Protect the database and
`secret.key` together; see [Database](../operations/database.md#backups).

## Related Docs

- [Migrations](../development/migrations.md)
- [Database operations](../operations/database.md)
- [ADR-0003: SQLite first](../decisions/ADR-0003-sqlite-first.md)
