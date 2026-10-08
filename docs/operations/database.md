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
| `rainy.db` (with `-wal` and `-shm`) | Users, sessions, playlists, favorites, ratings, play history, play queues, settings, the library index, trash entries, and the edit history | Yes |
| `secret.key` | 32-byte key that encrypts stored passwords | Yes. **Without it, every user must have their password reset.** |
| `trash/` | Music files deleted in Rainy | Recommended |
| `cache/` | Resized cover images | No; rebuilt on demand |
| `tmp/` | Upload staging | No |

The library index (tracks, albums, artists, genres) can be rebuilt by
rescanning, but favorites, ratings, playlists, play counts, and the edit
history cannot. Back up the whole database.

## Migrations on Startup

Every start applies any new embedded migrations before the server listens.
Each migration runs in its own transaction; a failure rolls it back and stops
startup with an error that names the migration. Restore your backup or run the
previous image in that case. Migration rules for contributors are in
[Migrations](../development/migrations.md).

## Connection Settings

Rainy opens a read-only reader pool and a single writer connection. Every
connection applies:

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

Stop Rainy before copying the database, so `rainy.db` and its WAL form a
consistent snapshot:

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

1. Stop the container: `docker compose stop rainy`.
2. Replace the data folder with the backup contents.
3. Start it again: `docker compose start rainy`.

The entrypoint fixes file ownership on startup. If the restored database is
older than the running image, its migrations run automatically.

## Maintenance

- **Clear the cover cache** under **Admin → System**, or delete
  `cache/` while Rainy is stopped. The same tab shows the database, cache, and
  trash sizes.
- **Empty the trash** from **Manage → Trash** to reclaim space.
- **Purge missing tracks** from **Manage → Doctor** after files are removed on
  purpose.

## Related Docs

- [Data model](../architecture/data-model.md)
- [Migrations](../development/migrations.md)
- [Configuration](configuration.md)
