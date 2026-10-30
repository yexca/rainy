# Agent Guide

## Start here

Read `README.md`, `docs/overview.md`, `docs/architecture/index.md`,
`docs/architecture/core-boundaries.md`, `SECURITY.md`, and
`docs/development/ci.md`. Read the area table below before changing code.

| Area | Read first |
| --- | --- |
| Schema, APIs, shared types | `docs/architecture/contract.md` |
| Files, scanning, and media | `docs/architecture/library-management.md`, `docs/operations/reliability.md` |
| Go services and handlers | `docs/architecture/backend.md`, `docs/development/backend-guidelines.md` |
| React and UI | `docs/architecture/frontend.md`, `docs/development/frontend-guidelines.md`, `docs/development/design.md` |
| Security and outbound requests | `SECURITY.md`, `docs/development/security.md`, `PRIVACY.md` |
| Tests | `docs/development/testing.md` |
| Migrations and releases | `docs/development/migrations.md`, `docs/development/commit-and-release.md` |

## CodeGraph

`.codegraph/` is a local CodeGraph index and is ignored by Git. When it exists,
use `codegraph explore "<question>"` (or the CodeGraph MCP tool) before
searching for symbols or reading implementation files. Use direct reads for
docs and configuration. Run `codegraph sync .` after substantial local edits;
run `codegraph init .` when the index is absent. Never commit the index or its
runtime files.

## Architecture boundaries

Rainy treats files on disk as the source of truth and the database as their
index. File changes are explicit, reversible or logged, hold the scanner lock,
rescan affected paths, and publish a `library` event. Paths are library-
relative with `/`; construct absolute paths only through `util.SafeJoin` and
`util.EnsureWithinRoot`. Read-only libraries return `readonly`.

Track IDs are random and stable across moves, renames, and tag edits. Album,
artist, and genre IDs are deterministic hashes. Scans must not mark a library
missing when its root or a large folder is unavailable.

The dependency direction is:

```text
cmd/rainy -> server -> api/subsonic -> app -> services -> store/auth/tags/events -> db/model/util/config
```

HTTP handlers parse, authorize, call a service or store, and encode. Services
do not import `app`, `api`, `subsonic`, or `server`. The native `/api` contract
and `web/src/lib/api/types.ts` change together; Subsonic response changes also
update the golden files in `internal/subsonic/testdata`.

## Filesystem, media, and network rules

- Tag reads and writes go through `internal/tags`; ffprobe/ffmpeg are read-only
  fallbacks. Do not add a write path that bypasses TagLib.
- Invoke ffmpeg, ffprobe, and yt-dlp with fixed arguments, bounded contexts,
  absolute local `file:` inputs, and `--` before validated links.
- Stream uploads through the data tmp directory with per-file and per-request
  limits. Never overwrite an existing library file.
- Outbound features are disabled by default and start only from an explicit
  user action or linked scrobbling account. Accept HTTP(S) without credentials,
  validate every redirect and dialled address, bound time and response size,
  and keep credentials, full URLs, and local paths out of public errors.
- Use `internal/lxmusic`'s guarded transport for untrusted source URLs. Add
  redirect, private-address, response-limit, and cancellation tests for every
  new outbound path, and update `PRIVACY.md` and the privacy allowlist when
  needed.

## UI and tests

Follow `docs/development/design.md`: mobile-first at 375px, safe-area padding,
44px touch targets, persistent player and queue, and visible keyboard focus.
Every user-visible string uses i18next with both `zh` and `en` translations.

Tests protect a user-visible behavior, public contract, state transition,
security boundary, or regression at the lowest sufficient layer. Use synthetic
media from `make testdata` or `t.TempDir()`, reserved domains and paths, and
accessible roles and labels in browser checks. Skip ffmpeg-dependent tests when
the tool is unavailable.

## Validation and Actions

The Makefile is the CI contract. Use the smallest sufficient target:

- docs: `make docs-check`
- style: `make ci-style`
- backend: `make ci-backend`
- frontend: `make ci-frontend`
- image/runtime: `make ci-production`
- complete local validation: `make ci-local` or `make ci`

GitHub Actions uses the same targets. Toolchain versions come from `.nvmrc` and
`go.mod` through `.github/actions/setup-node` and `.github/actions/setup-go`.
Run `make sensitive-check` before handoff or commit and review every finding.

## Documentation and releases

Documentation states current behavior in present tense. Keep one authoritative
statement for each rule; history belongs in `docs/history/` and ADRs. Update
the contract and translated user pages when public behavior changes.

Read `VERSION` for release identity. Numbered migrations are immutable after
release; add the next contiguous number. A release tag equals `VERSION`, points
to a commit on `main` with successful CI, and has
`docs/history/<tag>.md`. Follow the signed-commit workflow and do not bypass
the configured signer. Do not push or publish a PR, issue, or release without
the user's explicit approval.

Public tracked examples use reserved domains, generic paths, and synthetic
identifiers. Never commit credentials, private paths, personal media, logs, or
databases.
