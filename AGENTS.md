# Agent Guide

## Start Here

Read these first:

- `README.md`
- `docs/overview.md`
- `docs/architecture/index.md`
- `docs/architecture/core-boundaries.md`
- `SECURITY.md`
- `docs/development/security.md`

Then read the pages for the change:

| Change | Read |
| --- | --- |
| Schema, Go API, native/Subsonic endpoints, shared types | Relevant sections of `docs/architecture/contract.md` |
| File mutations, scanning, media | `docs/architecture/library-management.md`, `docs/operations/reliability.md` |
| Go services and handlers | `docs/architecture/backend.md`, `docs/development/backend-guidelines.md` |
| React features and UI | `docs/architecture/frontend.md`, `docs/development/frontend-guidelines.md`, `docs/development/design.md` |
| Validation or Actions | `docs/development/testing.md`, `docs/development/ci.md` |
| Migrations or releases | `docs/development/migrations.md`, `docs/development/commit-and-release.md` |

Keep code and the precise contract in `docs/architecture/contract.md` in sync.
When they disagree, fix the code, or update the contract in the same change
and say so. Current workflow instructions live here and in development docs;
early implementation notes do not assign permanent file ownership.

If `.codegraph/` exists, use `codegraph_explore` or `codegraph explore` before
searching or reading code to locate symbols and understand their callers.
Use direct reads for docs/configuration or details the index does not cover.
Do not create an index when the repository has none.

## Product Boundaries

Critical rule:

```text
The music library belongs to the user. Rainy only changes a file when a user
explicitly asks for that change, and every change stays reversible or logged.
```

- A track's identity is its random, stable id. Renames, moves, and tag edits
  keep the id so stars, play counts, playlists, and bookmarks survive. Album,
  artist, and genre ids are deterministic hashes; do not introduce a second
  identity for the same file.
- Paths stored in the database and exposed by the API are library-relative
  with `/`. Build absolute paths only through `util.SafeJoin` (plus the
  symlink-aware `util.EnsureWithinRoot` for file mutations). Never read or write
  outside a library root or the data directory.
- Deleting from the app moves files to the trash under the data directory and
  marks tracks missing; only an explicit purge removes files. Every file
  mutation holds the scanner's library lock across the change and the
  following `RescanFiles`, writes an `edit_log` row, and publishes a `library`
  event. Read-only mounts must fail with the `readonly` error, never silently.
- A scan must never mass-mark a library missing because a NAS share is offline:
  a missing, unreadable, or suddenly empty library root or large folder aborts
  or is skipped instead.
- The Subsonic/OpenSubsonic API is a compatibility surface for third-party
  clients. Do not change response shapes, field names, or error codes without
  checking the OpenSubsonic specification and updating the golden files in
  `internal/subsonic/testdata`.
- The native API (`/api`) and `web/src/lib/api/types.ts` are one contract.
  Change both together.

## Filesystem and Media Boundary

- Tag reading and writing go through `internal/tags` (TagLib compiled to WASM,
  pure Go). Files TagLib cannot parse fall back to ffprobe/ffmpeg for reading
  only; do not add write paths that bypass TagLib.
- ffmpeg is invoked with an absolute `file:` input and fixed arguments, bounded
  by the request context. Never pass user input as an ffmpeg option.
- Uploads stream to the data directory's tmp folder with per-file and
  per-request limits before being moved into a library; never overwrite an
  existing file.
- Mojibake repair is conservative by design: it must never change valid
  Western text (accented Latin, punctuation, symbols). Add adversarial tests
  for every heuristic change.

## Code Organization

Dependencies point downward:

```text
cmd -> app -> api / subsonic / server -> manage / scanner / artwork / transcode / lyrics / metasearch -> store / tags / auth / events -> db / model / util / config
```

- `model`, `util`, `config`, and `buildinfo` import nothing internal. Services
  never import `api`, `subsonic`, `server`, or `app`.
- Keep HTTP handlers thin: parse, authorize, call a service or the store,
  encode. Do not grow unrelated orchestration inside handler files.
- Frontend features live in `web/src/features/<area>`. Shared code in
  `web/src/lib`, `components`, `hooks`, and `stores` must not import a
  feature; features talk through the small shared contracts documented in the
  contract (`useUI`, `usePlayer`, `TrackActionsMenu`, …).

## UI and Test Contracts

- Follow `docs/development/design.md`. Mobile first: check 375px width, iOS
  safe areas, and 44px touch targets. The player and its queue persist across
  navigation.
- Every user-visible string goes through i18next with both `zh` and `en`
  translations.
- Tests protect a user-visible behavior, public contract, state transition, or
  prior regression. Choose the lowest sufficient layer.
- Test media is synthetic: generate it with `make testdata`
  (`scripts/gen-testdata.sh`) or build files in `t.TempDir()`. Never commit
  real music, real library paths, databases, or logs. Tests that need ffmpeg
  must skip when it is not installed.

## Validation Commands

The `Makefile` is the canonical entry point for repository validation. Prefer
its targets over reconstructing CI commands by hand, so local checks and GitHub
Actions exercise the same commands. Use a direct command only when no target
exists for the required check. `make help` lists the common entry points.

- Use the smallest sufficient target: `make docs-check` (or its compatibility
  alias `frontend-docs`) for public documentation, `make ci-policy` for docs
  and validation-policy tests, `make ci-style` for web lint, `make ci-backend`
  for Go behavior, and `make ci-frontend` for web audit, typecheck, tests, and
  build. Go formatting is part of `ci-backend-static`.
- Build and run the application with Docker by default. Use
  `make DOCKER_IMAGE=rainy:dev docker-build smoke` to build the image and
  exercise it, and `make docker-up` (`RAINY_DEV_MUSIC_PATH` selects the
  library; `make docker-logs`, `make docker-down`) when a running instance is
  needed for manual or browser checks. Do not build or start local binaries
  (`make backend-build`, `make backend-run`, `go run ./cmd/rainy`,
  `pnpm build`) for this unless the user asks; host-side test and lint targets
  are fine. Remove any local build output you create (`bin/`, generated
  `web/dist` files, `coverage.out`).
- Use `make DOCKER_IMAGE=rainy:ci docker-build smoke` (or `make ci-production`)
  for Dockerfile, entrypoint, or runtime changes.
- `make ci-local` runs the complete GitHub Actions sequence locally.
- Keep validation proportional. A narrow change does not require all phases;
  Actions selects affected jobs and keeps the required Core check stable.
- Before every commit, run `make sensitive-check` against the actual working
  tree diff and review any findings. An approved public endpoint belongs in
  `scripts/privacy-allowlist.json` with its owner files and a reason; review
  every change to that list.
- Choose tests for observable behavior and use accessible roles and labels in
  browser checks. Do not make utility classes or incidental DOM ancestry a
  public test contract.

## Release and Handoff

Use the repository's normal signed-commit path. If the 1Password signing agent
requires approval or is unavailable, stop and ask the user. Do not disable
commit signing, pass a no-sign flag, replace the configured signer, or
otherwise bypass the agent. Do not commit unless the user asks.

Release and migration boundaries are derived from repository state:

- Read `VERSION` for the application version (`v<major>.<minor>.<patch>`); do
  not hard-code it elsewhere.
- Numbered migrations in `internal/db/migrations` are immutable once released.
  Add the next contiguous number for a schema change; never edit an applied
  migration.
- A release tag must equal `VERSION`, point at a commit on `main` whose CI
  succeeded, and have release notes at `docs/history/<tag>.md`. The release
  workflow publishes multi-arch images to Docker Hub and GHCR, then the GitHub
  Release. Run `make release-check` before tagging; see
  `docs/development/commit-and-release.md`.

Before handoff, run validation proportional to the change (see above) plus
`make sensitive-check`.

Public tracked code and docs must use reserved domains (`example.com`,
`192.0.2.x`), generic paths (`/path/to/music`), and obviously synthetic names.
Never commit real library paths, server addresses, credentials, or personal
listening data.
