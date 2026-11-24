# Backend Agent Guide

The root [AGENTS.md](../AGENTS.md) invariants apply. This guide covers
`internal/` and `cmd/`. The [contract](../docs/architecture/contract.md) is
large; open only the section a change needs.

## Layering

    cmd/rainy -> server -> api/subsonic -> app -> services -> store/auth/tags/events -> db/model/util/config

- Handlers parse, authorize, call a service or store, and encode. Services do
  not import `app`, `api`, `subsonic`, or `server`.
- Responsibilities and error, context, and logging style:
  [Backend guidelines](../docs/development/backend-guidelines.md).

## Files and Paths

- Resolve absolute paths only through `util.SafeJoin` and enforce
  symlink-aware containment with `util.EnsureWithinRoot` before serving or
  mutating files. Libraries are existing directories and cannot overlap the
  data directory or another library. The first-start default library uses
  `RAINY_MUSIC_DIR` only when it passes the same path checks.
- Tag reads and writes go through `internal/tags` and TagLib. `ffprobe` and
  `ffmpeg` are read-only fallbacks.
- Uploads stream through the data `tmp` directory with per-file and
  per-request limits and never overwrite an existing library file.
- Invoke `ffmpeg`, `ffprobe`, and `yt-dlp` with fixed arguments, bounded
  contexts, absolute local `file:` inputs, and `--` before validated links.
- Details: [Rules for every file operation](../docs/architecture/library-management.md#rules-for-every-file-operation),
  [Filesystem operations](../docs/development/security.md#filesystem-operations).

## Outbound Requests

- Accept HTTP(S) without credentials, validate every redirect and dialled
  address, bound time and response size, and use `internal/lxmusic`'s guarded
  transport for untrusted source URLs.
- Add redirect, private-address, response-limit, and cancellation coverage for
  every new outbound path.
- Update [PRIVACY.md](../PRIVACY.md) and register a new destination with a
  reason in `scripts/privacy-allowlist.json`. Allowlisting alone does not make
  a destination safe.
- Details: [Outbound requests](../docs/development/security.md#outbound-requests).

## Schema and Contracts

- Migrations live in `internal/db/migrations/`. Released migrations are
  immutable and use the next contiguous number; follow the `migration` skill
  and [Migrations](../docs/development/migrations.md).
- A schema, package API, endpoint, or Subsonic change updates its contract
  section in the same change; see
  [Changes that need contract review](../docs/development/backend-guidelines.md#changes-that-need-contract-review).

## Tests

Use `dbtest.New(t)` for persistence and synthetic media from `make testdata`
or `t.TempDir()`. Skip `ffmpeg`-dependent tests when the tool is unavailable.
See [Backend tests](../docs/development/testing.md#backend).

## Which Page To Open

| Change | Page |
| --- | --- |
| Identity, file effects, user state, permissions | [Core boundaries](../docs/architecture/core-boundaries.md) |
| Packages, request flow, background work | [Backend](../docs/architecture/backend.md) |
| Tables and identifiers | [Data model](../docs/architecture/data-model.md), [contract §5.1](../docs/architecture/contract.md#51-database) |
| Models and store queries | [contract §5.2](../docs/architecture/contract.md#52-models-internalmodel), [§5.4](../docs/architecture/contract.md#54-store-internalstore) |
| Sessions, API keys, roles | [contract §5.5](../docs/architecture/contract.md#55-auth), [Authentication and authorization](../docs/development/security.md#authentication-and-authorization) |
| Scanner | [contract §5.8](../docs/architecture/contract.md#58-scanner), [Storage and scans](../docs/operations/reliability.md#storage-and-scans) |
| Tags, moves, downloads, uploads, trash | [Library management](../docs/architecture/library-management.md), [contract §5.11](../docs/architecture/contract.md#511-manage) |
| Metadata lookup, yt-dlp, music sources, scrobbling | contract [§5.13](../docs/architecture/contract.md#513-metasearch)–[§5.16](../docs/architecture/contract.md#516-scrobble), [PRIVACY.md](../PRIVACY.md) |
| `/api` endpoints | [Native API](../docs/architecture/native-api.md), [contract §7](../docs/architecture/contract.md#7-native-api-api--json-contract) |
| `/rest` endpoints | [Subsonic](../docs/architecture/subsonic.md), [contract §6](../docs/architecture/contract.md#6-subsonic--opensubsonic-rest) |
| Configuration variables | [Configuration](../docs/operations/configuration.md) |
| Tests | [Testing](../docs/development/testing.md) |
