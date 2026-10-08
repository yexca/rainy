# Core Boundaries

The music library belongs to the user. Rainy changes a file only when the user
asks, and every change stays reversible or logged. This page explains the
boundaries; the precise APIs and schema remain in the [contract](contract.md).

## Files and the Index

Music files, tags, covers, and lyrics on disk are the source of truth. SQLite
indexes them and stores the state that files cannot hold: accounts, playlists,
favorites, play counts, queues, settings, trash entries, and edit history.
Scanning reads files; it does not rewrite tags or reorganize a collection.
Management changes files first and rescans the affected paths.

## Identity and User State

A track has one random, stable ID. Moving, renaming, or editing the same file
preserves that ID and its listening state. Album, artist, and genre IDs are
deterministic hashes. A new location or edited title is not permission to
create another identity for the same track.

Favorites, playlists, queues, bookmarks, and listening history belong to users.
Refreshes of indexed metadata must preserve them. Destructive cleanup of
missing tracks is an explicit management operation.

## Filesystem Effects

Database and API file paths are library-relative with `/`. Resolve absolute
paths through `util.SafeJoin`; use symlink-aware `util.EnsureWithinRoot` for
mutations. File access stays inside a library root or the data directory.

Every mutation holds the scanner's library lock across the change and its
targeted rescan, records an `edit_log` entry, and publishes a `library` event.
Deletion moves files into the data-directory trash; only explicit purge
removes them. Read-only mounts return the `readonly` error.

Tag writes go through `internal/tags` and TagLib. The ffprobe/ffmpeg fallback
is for reading. Uploads stage under the data directory with byte limits and
never overwrite existing files. See [Library management](library-management.md).

## Visibility and Availability

A missing, unreadable, or suddenly empty library root is an unavailable
library, not evidence that the user deleted all tracks. Large empty folders
also have a scanner safety guard. Preserve the index where the scan cannot
observe files; see [Reliability](../operations/reliability.md).

## Transport and Permissions

The native `/api` JSON shapes and `web/src/lib/api/types.ts` form one contract.
The `/rest` Subsonic/OpenSubsonic API is a compatibility surface for third-party
clients, with its own response shapes and error codes. Changes require the
relevant contract and golden-file updates.

The backend enforces listener, manager, administrator, and download permissions.
UI visibility does not grant access. Optional metadata lookup, downloads,
online music, and scrobbling cross an outbound boundary only when enabled;
follow [Secure development](../development/security.md) and
[Privacy](../../PRIVACY.md) when changing them.

## Process Boundary

One process owns one data directory. SQLite writer serialization and scanner
locks coordinate that process; they do not make multiple Rainy replicas safe.
The web player and queue persist across route changes within the app shell.

## Related Docs

- [Data model](data-model.md)
- [Backend](backend.md) and [Frontend](frontend.md)
- [Architecture contract](contract.md)
