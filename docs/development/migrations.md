# Migrations

Database migrations live in `internal/db/migrations/`. They are embedded into
the binary with `go:embed`, so a container never needs a migrations directory
beside the executable.

## Numbered Chain

Each migration is a plain SQL file named with a four-digit number and a short
description:

```text
0001_init.sql
0002_play_queue_index.sql
0003_<description>.sql
...
```

`db.Migrate` runs at every startup (and before CLI commands such as
`rainy user list`). It sorts the embedded files by name, skips versions already
recorded in `schema_migrations`, and applies each remaining file in its own
write transaction together with the row that records it. A failed migration
rolls back completely and stops startup with the file name in the error.

The version stored in `schema_migrations` is the file name without `.sql`.

## Rules

- **Released migrations are immutable.** A database that already applied a
  file never runs it again, so editing a released file silently diverges fresh
  installs from upgraded ones. Fix mistakes with a new migration.
- **Use the next contiguous number.** Inspect `internal/db/migrations/` and
  add the next number. Because files apply in lexical order, keep the
  four-digit zero padding and never reuse or skip a number. If two branches
  add the same number, renumber the unreleased one before merging.
- **Keep SQL deterministic.** Do not depend on the current time or on data that
  differs between installations, except through explicit defaults.
- **Make data backfills safe to retry** where an interrupted run could reach
  them.
- **Preserve user state.** Favorites, ratings, play history, playlists, and the
  edit log cannot be rebuilt by rescanning. Never drop or rewrite those tables
  without a migration that carries their data forward.
- **Follow the schema conventions** in
  [Data model](../architecture/data-model.md#conventions): millisecond
  timestamps, `0`/`1` booleans, `NOT NULL DEFAULT ''` text, and
  library-relative paths.

## Checklist

1. Add `internal/db/migrations/<next>_<description>.sql`.
2. Update the Go models, the store queries, and, when the JSON shape changes,
   `web/src/lib/api/types.ts`.
3. Update the [contract](../architecture/contract.md#51-database) and
   [Data model](../architecture/data-model.md) when schema meaning changes.
4. Add a store test that exercises the new column or table through
   `dbtest.New(t)`, which runs the complete chain.
5. Mention the migration in
   [`docs/history/unreleased.md`](../history/unreleased.md) so the release note
   can tell operators to back up `/data` first.

## Related Docs

- [Database operations](../operations/database.md)
- [Data model](../architecture/data-model.md)
- [Testing](testing.md)
