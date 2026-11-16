# Agent Guide

Rainy is a self-hosted music server with a Subsonic/OpenSubsonic API, an installable web player, and browser-based library management.

Read these first:

- `README.md`
- `docs/overview.md`
- `docs/architecture/index.md`
- `docs/architecture/core-boundaries.md`
- `SECURITY.md`
- `docs/development/ci.md`

Before changing an area, read its contract and implementation guidance.

| Area | Main paths | Read first |
| --- | --- | --- |
| Schema, persistence, and shared types | `internal/db/`, `internal/store/`, `internal/model/`, `web/src/lib/api/types.ts` | `docs/architecture/contract.md`, `docs/architecture/data-model.md`, `docs/development/migrations.md` |
| Files, scanning, media, and artwork | `internal/scanner/`, `internal/manage/`, `internal/artwork/`, `internal/tags/`, `internal/transcode/`, `internal/lyrics/` | `docs/architecture/library-management.md`, `docs/operations/reliability.md` |
| Go services and handlers | `cmd/`, `internal/app/`, `internal/api/`, `internal/subsonic/`, `internal/server/` | `docs/architecture/backend.md`, `docs/development/backend-guidelines.md` |
| React player and UI | `web/src/`, `web/public/`, `web/embed.go` | `docs/architecture/frontend.md`, `docs/development/frontend-guidelines.md`, `docs/development/design.md` |
| Authentication and outbound requests | `internal/auth/`, `internal/lxmusic/`, `internal/metasearch/`, `internal/ytdlp/`, `internal/scrobble/` | `SECURITY.md`, `docs/development/security.md`, `PRIVACY.md` |
| Deployment, configuration, and runtime | `Dockerfile`, `docker/`, `deploy/`, `.env.example` | `docs/operations/docker.md`, `docs/operations/configuration.md`, `docs/operations/security.md` |
| Tests and fixtures | `*_test.go`, `web/tests/`, `testdata/`, `scripts/` | `docs/development/testing.md` |
| Commits and releases | `VERSION`, `docs/history/`, `.github/workflows/` | `docs/development/commit-and-release.md`, `docs/development/ci.md` |

## CodeGraph

`.codegraph/` is a local CodeGraph index and is ignored by Git. When it exists, use `codegraph explore "<question>"` before searching for symbols or reading implementation files. Use direct reads for documentation and configuration. Run `codegraph sync .` after substantial code edits; run `codegraph init .` when the index is absent. Never commit the index or its runtime files.

## Product and architecture boundaries

- Files on disk are the source of truth for music and media; SQLite is their index and stores user state. File mutations are explicit, hold the scanner lock, are reversible or logged, rescan affected paths, and publish a `library` event.
- Library paths are library-relative with `/`. Resolve absolute paths only through `util.SafeJoin` and enforce symlink-aware containment with `util.EnsureWithinRoot` before serving or mutating files. Libraries are existing directories and cannot overlap the data directory or another library. The first-start default library uses `RAINY_MUSIC_DIR` only when it passes the same path checks.
- Track IDs are random and stable across moves, renames, and tag edits. Album, artist, and genre IDs are deterministic hashes. Scans preserve indexed tracks when a root or large subtree is unavailable.
- The dependency direction is `cmd/rainy -> server -> api/subsonic -> app -> services -> store/auth/tags/events -> db/model/util/config`. Handlers parse, authorize, call a service or store, and encode. Services do not import `app`, `api`, `subsonic`, or `server`.
- The native `/api` contract and `web/src/lib/api/types.ts` change together. Subsonic response changes also update golden files in `internal/subsonic/testdata`.

## Security, filesystem, and network boundaries

- Public routes are limited to the app shell, health, authentication status, login, logout, and setup before users exist. Backend handlers enforce authentication and roles for protected operations.
- Never expose host paths, credentials, full untrusted URLs, stack traces, or raw filesystem errors in public responses. Hash session and API-key secrets; encrypt stored passwords and service credentials; keep secrets out of logs.
- Tag reads and writes go through `internal/tags` and TagLib. `ffprobe` and `ffmpeg` are read-only fallbacks. Uploads stream through the data `tmp` directory with per-file and per-request limits and never overwrite an existing library file.
- Invoke `ffmpeg`, `ffprobe`, and `yt-dlp` with fixed arguments, bounded contexts, absolute local `file:` inputs, and `--` before validated links.
- Outbound features are disabled by default and start only from an explicit user action or linked scrobbling account. Accept HTTP(S) without credentials, validate every redirect and dialled address, bound time and response size, and use `internal/lxmusic`'s guarded transport for untrusted source URLs. Add redirect, private-address, response-limit, and cancellation coverage for every new outbound path. Update `PRIVACY.md` and the privacy allowlist for new destinations.
- Development authentication is local-only and explicit. Do not expose development mode or its automatically signed-in user to an untrusted network.

## UI and test contracts

- Follow `docs/development/design.md`: mobile-first at 375px, safe-area padding, 44px touch targets, a persistent player and queue, and visible keyboard focus. Every user-visible string uses i18next with both `zh` and `en` translations.
- Tests protect a user-visible behavior, public contract, state transition, security boundary, or regression at the lowest sufficient layer. Test code is evidence, not proof: inspect production paths independently and do not treat passing tests as a security guarantee.
- Use synthetic media from `make testdata` or `t.TempDir()`, reserved domains and paths, and accessible roles and labels in browser checks. Do not make real outbound requests. Skip `ffmpeg`-dependent tests when the tool is unavailable.

## Documentation

Docs describe the current system and current decisions in present tense. Do not narrate change with words such as “previously”, “no longer”, “now”, “as before”, “was removed”, or “retired”; do not add migration stories, before/after comparisons, plans, roadmaps, TODOs, dated measurement logs, or PR/issue/commit references to reference pages. Rewrite affected sections so each page states the current behavior directly.

History belongs in `docs/history/`. Durable design reasons belong in `docs/decisions/`. Keep one authoritative statement for each rule and link to it from summaries. User-visible behavior belongs in `docs/user/` and `README.md`; system boundaries belong in `docs/architecture/`; runtime instructions belong in `docs/operations/`; local developer workflow belongs in `docs/development/`.

English is canonical. Translated READMEs summarize the English README and link back to the English documentation. Use reserved domains, `192.0.2.0/24` addresses, `/path/to/music`, and synthetic identifiers in public examples. Keep `.env.example`, configuration docs, the Makefile, CI, and release docs consistent. Update `docs/architecture/contract.md` with schema, API, TypeScript, or Subsonic contract changes.

## Validation commands

The Makefile is the CI contract. Use the smallest sufficient target:

- `make docs-check` for documentation and links.
- `make ci-style` for formatting and static style checks.
- `make ci-backend-static`, `make ci-backend-coverage`, and `make ci-backend-race` for focused backend validation.
- `make ci-backend` for Go backend checks and tests.
- `make ci-frontend` for frontend type checks, build, and tests.
- `make ci-production` for image and runtime checks.
- `make ci-local` or `make ci` for complete local validation.

Toolchain versions come from `.nvmrc` and `go.mod` through `.github/actions/setup-node` and `.github/actions/setup-go`. Run `make sensitive-check` before handoff and review every finding. Remove generated artifacts and preserve unrelated user changes.

## Commits, releases, and handoff

- Begin with `git status --short`; do not use destructive reset or checkout commands. Preserve unrelated user changes.
- Do not commit `.env`, credentials, private paths, personal media, logs, databases/WAL, runtime media, or generated indexes. Ignore rules are not a secrecy boundary.
- Released migrations are immutable and use the next contiguous number. `VERSION` is the release identity; a release tag points to a commit on `main` with successful CI and has a matching `docs/history/<tag>.md` entry.
- Follow the signed-commit workflow. Agents create commits, amend/rebase, push, or publish PRs, issues, or releases only when the user explicitly requests it.
- Before handoff, run proportional validation, `make sensitive-check`, and a final `git status --short` plus diff review. Do not claim a test result that was not run.
