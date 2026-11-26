# Backend Agent Guide

The root [AGENTS.md](../AGENTS.md) invariants apply to `internal/` and `cmd/`.
Open only the [contract](../docs/architecture/contract.md) section a change
needs.

## Layering

    cmd/rainy -> server -> api/subsonic -> app -> services -> store/auth/tags/events -> db/model/util/config

Handlers parse, authorize, call a service or store, and encode. Services never
import `app`, `api`, `subsonic`, or `server`; primitives import nothing
internal. `depguard` enforces this. Style:
[Backend guidelines](../docs/development/backend-guidelines.md).

## Files and Paths

- Resolve paths with `util.SafeJoin`, then `util.EnsureWithinRoot`, before
  serving or mutating a file.
- Tags go through `internal/tags` (TagLib); `ffprobe` and `ffmpeg` only read.
- Run `ffmpeg`, `ffprobe`, and `yt-dlp` with fixed arguments and bounded
  contexts.
- Libraries, uploads, and tool arguments:
  [Rules for every file operation](../docs/architecture/library-management.md#rules-for-every-file-operation),
  [Filesystem operations](../docs/development/security.md#filesystem-operations).

## Outbound Requests

- Build clients with `outbound.NewClient` and an explicit `outbound.Redirects`
  (copy `newHTTPClient` in `internal/ytdlp/update.go`) and read bodies with
  `outbound.ReadAll`. Accept only HTTP(S) links without credentials; untrusted
  URLs use `internal/lxmusic`'s guarded transport.
- Test redirects, private addresses, response limits, and cancellation for
  every new outbound path.
- Add a new destination to [PRIVACY.md](../PRIVACY.md) and, with a reason, to
  `scripts/privacy-allowlist.json`; listing it does not make it safe.
- Rules: [Outbound requests](../docs/development/security.md#outbound-requests).

## Schema and Contracts

- Released migrations are immutable; follow the `migration` skill and
  [Migrations](../docs/development/migrations.md).
- A schema, package API, endpoint, or Subsonic change updates its contract
  section in the same change
  ([contract review](../docs/development/backend-guidelines.md#changes-that-need-contract-review)).

## Tests

Use `dbtest.New(t)` and synthetic media from `make testdata` or `t.TempDir()`;
see [Backend tests](../docs/development/testing.md#backend).

## Which Page To Open

| Change | Page |
| --- | --- |
| Identity, file effects, user state, permissions | [Core boundaries](../docs/architecture/core-boundaries.md) |
| Packages, request flow, background work | [Backend](../docs/architecture/backend.md) |
| Tables and identifiers | [Data model](../docs/architecture/data-model.md), [contract §5.1](../docs/architecture/contract.md#51-database) |
| Models and store queries | [contract §5.2](../docs/architecture/contract.md#52-models-internalmodel), [§5.4](../docs/architecture/contract.md#54-store-internalstore) |
| Sessions, API keys, roles | [contract §5.5](../docs/architecture/contract.md#55-auth), [Authentication](../docs/development/security.md#authentication-and-authorization) |
| Scanner | [contract §5.8](../docs/architecture/contract.md#58-scanner), [Storage and scans](../docs/operations/reliability.md#storage-and-scans) |
| Tags, moves, downloads, uploads, trash | [Library management](../docs/architecture/library-management.md), [contract §5.11](../docs/architecture/contract.md#511-manage) |
| Metadata, yt-dlp, music sources, scrobbling | contract [§5.13](../docs/architecture/contract.md#513-metasearch)–[§5.16](../docs/architecture/contract.md#516-scrobble), [PRIVACY.md](../PRIVACY.md) |
| `/api` endpoints | [Native API](../docs/architecture/native-api.md), [contract §7](../docs/architecture/contract.md#7-native-api-api--json-contract) |
| `/rest` endpoints | [Subsonic](../docs/architecture/subsonic.md), [contract §6](../docs/architecture/contract.md#6-subsonic--opensubsonic-rest) |
| Configuration variables | [Configuration](../docs/operations/configuration.md) |
