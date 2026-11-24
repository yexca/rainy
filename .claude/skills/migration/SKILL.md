---
name: migration
description: Add a Rainy database migration (a new numbered SQL file in internal/db/migrations/ with its models, store, contract, test, and upgrade note).
---

# Add a Migration

The rules are in [Migrations](../../../docs/development/migrations.md#rules);
the schema conventions are in
[Data model](../../../docs/architecture/data-model.md#conventions).

1. List `internal/db/migrations/` and take the next contiguous four-digit
   number. Never edit a released migration; fix mistakes with a new one.
2. Add `internal/db/migrations/<next>_<description>.sql` with deterministic
   SQL. Carry user state (favorites, ratings, play history, playlists, the
   edit log) forward; never drop or rewrite it.
3. Follow the [checklist](../../../docs/development/migrations.md#checklist):
   Go models, store queries, `web/src/lib/api/types.ts` when the JSON shape
   changes, the contract and data model when schema meaning changes, a store
   test through `dbtest.New(t)`, and an `[!IMPORTANT]` note in
   `docs/history/unreleased.md`.
4. Run `make ci-backend` (and `make ci-frontend` when `types.ts` changed),
   then `make sensitive-check`; review the findings and the diff.
