# Database

Rainy stores all of its state in SQLite, using the pure-Go `modernc.org/sqlite`
driver. There is no separate database server to run; see
[ADR-0003](../decisions/ADR-0003-sqlite-first.md).

## Location

```text
/config/rainy.db        (plus rainy.db-wal and rainy.db-shm while running)
/config/secret.key
```

On the host, these live in the folder configured by `RAINY_DATA_PATH`
(default `./config`). Outside Docker, the data directory is `RAINY_DATA_DIR`
(default `./config`).

## What Is Stored

| File or folder | Contents | Back up? |
| --- | --- | --- |
| `rainy.db` (with `-wal` and `-shm`) | Users, sessions, libraries, playlists, favorites, ratings, play history, play queues, bookmarks, radio stations, daily and weekly mixes, linked scrobbling accounts and plays waiting to be sent, imported music source scripts, settings, the library index, trash entries, and the edit history | Yes |
| `secret.key` | 32-byte key that encrypts stored passwords, the Last.fm shared secret, linked scrobbling credentials, and yt-dlp sign-in cookies | Yes. **Without it, every user must have their password reset** (see below). |
| `trash/` | Music files deleted in Rainy | Recommended |
| `ytdlp/` | yt-dlp installed by Rainy, its cache, and encrypted sign-in cookies | Optional; Rainy can install yt-dlp again |
| `cache/` | Resized cover images | No; rebuilt on demand |
| `tmp/` | Upload staging and running downloads | No |

The library index (tracks, albums, artists, genres) can be rebuilt by
rescanning, but favorites, ratings, playlists, play counts, and the edit
history cannot. Back up the whole database.

When `secret.key` is missing, Rainy creates a new one at startup and nothing
encrypted with the old key can be read: nobody can sign in until an
administrator resets each password with `rainy user reset-password` (see
[Docker](docker.md#command-line-tools)), the Last.fm shared secret must be
entered again, users must reconnect their scrobbling accounts, and yt-dlp
sign-in cookies must be pasted again.

## Migrations on Startup

Every start applies any new embedded migrations before the server listens.
Each migration runs in its own transaction; a failure rolls it back and stops
startup with an error that names the migration. Restore your backup or run the
previous image in that case. Migration rules for contributors are in
[Migrations](../development/migrations.md).

## Connection Settings

Rainy opens a reader pool (`query_only`, at least four connections or twice
the CPU count) and a single writer connection whose transactions take the
write lock up front (`BEGIN IMMEDIATE`). Every connection applies:

| Pragma | Value | Purpose |
| --- | --- | --- |
| `journal_mode` | `WAL` | Readers continue while the writer commits |
| `busy_timeout` | `10000` ms | Wait briefly for a competing writer |
| `foreign_keys` | `ON` | Enforce declared references |
| `synchronous` | `NORMAL` | Sync at WAL checkpoints rather than on every commit |
| `cache_size` | `-20000` (about 20 MB) | Page cache per connection |

Closing the server checkpoints the WAL. Keep the data folder on a local disk:
SQLite over network file systems (NFS or SMB) risks locking problems and
corruption.

## Backups

The maintenance CLI can snapshot application state while Rainy is running:

```sh
docker compose exec rainy rainy backup create example-backup
docker compose exec rainy rainy backup verify example-backup
```

The snapshot is stored in `<data directory>/backups/example-backup/` with
`rainy.db`, `secret.key`, and `manifest.json`. The database is copied with
SQLite `VACUUM INTO`, including committed WAL data. The manifest records the
application version, migration names, file sizes, and SHA-256 hashes. The
command requires an existing database and key; it never creates a replacement
key or migrates the source. Choose a new name for every snapshot.

`verify` checks hashes, SQLite integrity and foreign keys, migration
compatibility, and whether the key decrypts stored credentials. It changes
neither the running installation nor the snapshot. Snapshots from a newer
schema require a compatible Rainy binary. Commands stop after ten minutes;
database files are limited to 16 GiB.

These snapshots contain database state and its key only. Back up music,
`trash/`, and any required `ytdlp/` files separately. A database snapshot
does not recreate a deleted music file or the files referenced by trash
entries. Copy completed snapshots to separate, protected storage; another
directory on the same disk is not protection against disk failure.

For a backup of the whole data directory, stop Rainy before copying it so
the database and filesystem state are consistent:

```sh
docker compose stop rainy
tar czf rainy-backup-$(date +%F).tar.gz \
  --exclude='./config/cache' --exclude='./config/tmp' ./config
docker compose start rainy
```

New files in the data folder belong to the configured runtime user (root by
default); nonzero PUID also adjusts existing state ownership. `secret.key` is readable
only by its owner. If `tar` reports permission errors, run it with `sudo`.

Treat backups as sensitive: together, `rainy.db` and `secret.key` reveal every
user's password. Store them encrypted and access-controlled.

Back up your music separately with your NAS tools (snapshots, Hyper Backup, or
similar), especially once managers can edit tags or rename files. Rainy's
trash and edit history help undo mistakes, but they are not a backup.

## Restore

For a CLI snapshot:

1. Place its complete directory under `<data directory>/backups/<name>/`
   and run `rainy backup verify <name>` with a compatible Rainy binary.
2. Stop the container: `docker compose stop rainy`.
3. Preserve the current database, any `rainy.db-wal`, `rainy.db-shm` or
   `rainy.db-journal` files, and `secret.key` in a separate recovery directory.
   Copy the snapshot's database and key into the data directory together;
   do not leave a journal from another database beside the restored file.
4. Restore music and trash from the corresponding filesystem backup when
   needed. Database library paths must still point to the intended mounts.
5. Start the container: `docker compose start rainy`.

For a whole-data-directory backup:

1. Stop the container: `docker compose stop rainy`.
2. Replace the data folder with the backup contents.
3. Start it again: `docker compose start rainy`.

With a nonzero `PUID`, the entrypoint fixes the data folder's ownership on
startup. If the restored database is older than the running image, its
migrations run automatically.

## Maintenance

- **Clear the cover cache** under **Admin → System**, or delete
  `cache/` while Rainy is stopped. The same tab shows the database, cache, and
  trash sizes.
- **Empty the trash** from **Library tools → Trash** to reclaim space.
- **Purge missing tracks** from **Library tools → Doctor** after files are removed on
  purpose.

## Related Docs

- [Data model](../architecture/data-model.md)
- [Migrations](../development/migrations.md)
- [Configuration](configuration.md)
