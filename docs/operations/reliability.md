# Reliability

## Storage and Scans

Music files are the source of truth; the database is their index plus user
state. A scan aborts when a library root is missing, unreadable, or not a
directory, and when the root contains no audio files while the library still
has live tracks. Inside a library, a subfolder or link that cannot be read
keeps its tracks unchanged, and a completely empty folder that held at least
20 live tracks is treated as an offline mount until the folder itself is
removed or anything is placed in it. These guards can also retain tracks after
an intentional emptying; review the scanner log and
[Troubleshooting](troubleshooting.md) before purging anything.

Scans and management changes share a library lock. A file change holds it
through the targeted rescan, with an edit log and a library event. Busy
libraries can return a conflict; retry after the scan completes. Read-only
mounts return `readonly` instead of silently ignoring writes.

Deleted media goes to the data-directory trash and stays there until a
manager empties it; nothing expires automatically. Explicit purge is
permanent.
Tag writes require TagLib even when reading used the ffprobe fallback.
See [Library management](../architecture/library-management.md).

## Process and Recovery Limits

Run one process per data directory. In-process locks do not coordinate
multiple containers, and SQLite is not a shared network database. Keep its
file on local storage. Rainy applies migrations before listening and stops
startup if one fails; restore a compatible backup before attempting recovery.

On `SIGTERM` or `SIGINT`, Rainy stops accepting connections, cancels a
running scan, and gives in-flight requests up to 10 seconds to finish; event
streams and long transfers are cut after 3 seconds. It then cancels running
link and online-music downloads, stops source scripts, the scrobble sender,
and a yt-dlp install, and closes the database. The Compose file allows
20 seconds (`stop_grace_period`) before Docker kills the process.

An edit log is an audit record, not a transaction journal capable of undoing
every interrupted operation. Downloads and other operations interrupted by a
shutdown or crash do not resume. Inspect the files, trash, and history before
retrying a failed management operation.

## Playback and Optional Services

If ffmpeg is unavailable, Rainy can stream originals but cannot provide the
required transcode. Browser codec support still determines what can play.
Optional online lookup, downloads, online music, and scrobbling have their
own upstream failure modes; local library use does not require enabling them.

The web player's queue persists across navigation and is saved locally and
to the server. iOS may suspend background web audio; use the
[client guide](../user/clients.md) when reliable long background sessions are needed.

## Operator Routine

Pin the image to a reviewed release or digest for reproducible upgrades.
Back up the full database together with `secret.key`, and back up music with
the NAS tools. Protect both as sensitive data. Restore procedures and WAL
handling are in [Database](database.md).

Check logs and library mount availability before deleting missing records.
Use [Deployment security](security.md) before exposing the server, and
[Troubleshooting](troubleshooting.md) when health, scans, or playback fail.
