# Agent Guide

Read these first:

- `README.md`
- `docs/overview.md`
- `docs/architecture/index.md`
- `docs/architecture/core-boundaries.md`
- `docs/architecture/backend.md`
- `docs/architecture/frontend.md`
- `docs/development/design.md`
- `SECURITY.md`
- `docs/development/security.md`

Then read the pages for the change:

| Change | Read |
| --- | --- |
| Schema, Go API, native/Subsonic endpoints, shared types | Relevant sections of `docs/architecture/contract.md` |
| File mutations, scanning, media | `docs/architecture/library-management.md`, `docs/operations/reliability.md` |
| Go services and handlers | `docs/development/backend-guidelines.md` |
| React features and UI | `docs/development/frontend-guidelines.md` |
| Validation or Actions | `docs/development/testing.md`, `docs/development/ci.md` |
| Migrations or releases | `docs/development/migrations.md`, `docs/development/commit-and-release.md` |

`docs/architecture/contract.md` is the precise reference for the schema,
package APIs, endpoints, shared TypeScript types, and the design system. Keep
it and the code in sync: when they disagree, fix the code, or update the
contract in the same change and say so.

If `.codegraph/` exists, use `codegraph_explore` or `codegraph explore` before
searching or reading code to locate symbols and understand their callers. Use
direct reads for docs/configuration or details the index does not cover. Do
not create an index when the repository has none.

## Product Boundaries

Critical rule:

```text
The music library belongs to the user. Rainy only changes a file when a user
explicitly asks for that change, and every change stays reversible or logged.
```

Core boundary:

```text
The files on disk are the source of truth; the database is an index of them.
Management features change the files first and then rescan the affected paths.
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
- ffmpeg, ffprobe, and yt-dlp are invoked with fixed arguments and a context
  that bounds the run. Local inputs are absolute `file:` URLs; a validated
  link goes to yt-dlp after `--`. Never pass user input as a command-line
  option.
- Uploads stream to the data directory's tmp folder with per-file and
  per-request limits before being moved into a library; never overwrite an
  existing file.
- Mojibake repair is conservative by design: it must never change valid
  Western text (accented Latin, punctuation, symbols). Add adversarial tests
  for every heuristic change.

## Outbound Request Boundary

Rainy sends no outbound requests by default. Every outbound feature (online
metadata lookup, yt-dlp link downloads and updates, lx-music online music,
Last.fm and ListenBrainz scrobbling) sits behind an administrator setting that
is off by default, and runs only when a user starts it or, for scrobbling,
when a user links an account and then plays or stars songs. The full list of
destinations is in
[Runtime Topology](docs/architecture/index.md#runtime-topology) and
`PRIVACY.md`.

For every new or materially changed outbound HTTP path:

- Accept only HTTP(S) URLs and reject embedded credentials.
- Keep the destination fixed: a built-in public endpoint, or the host the
  feature validated. Do not follow arbitrary redirects; refuse them or check
  every hop against the same rule.
- Requests whose target comes from untrusted input, such as lx-music source
  scripts and the links they return, must use the guarded transport in
  `internal/lxmusic`, which checks the URL and every dialled address so DNS
  rebinding cannot reach loopback, private, or reserved addresses.
- Bound timeouts, response sizes, and streamed bytes. Large media streams to
  the data directory's tmp folder, never into memory.
- Keep credentials and full upstream URLs out of errors, logs, and API
  responses; name the host instead.
- Add tests for refused redirects, blocked addresses, response limits, and
  cancellation, and document the data flow in `PRIVACY.md`.

Each package builds its own client; there is no shared transport, so a new
path is not covered by another package's checks. A new built-in endpoint
belongs in `scripts/privacy-allowlist.json`.

## Code Organization

Dependencies point downward:

```text
cmd/rainy -> server -> api / subsonic -> app -> services -> auth / store / tags / events / nowplaying -> db / model / util / config / buildinfo
```

- `services` are `manage`, `scanner`, `artwork`, `transcode`, `lyrics`,
  `metasearch`, `ytdlp`, `lxmusic`, `scrobble`, `listening`, and `recommend`.
  A service may use a lower-level service (for example `manage` uses
  `scanner`), but never imports `app`, `api`, `subsonic`, or `server`.
- `app` only constructs and wires the services; HTTP packages reach them
  through `*app.App`.
- `db`, `model`, `util`, `config`, `buildinfo`, `tags`, `events`, and
  `nowplaying` import nothing internal.
- Keep HTTP handlers thin: parse, authorize, call a service or the store,
  encode. Do not grow unrelated orchestration inside handler files.
- Frontend features live in `web/src/features/<area>`. Shared code in
  `web/src/lib`, `components`, `hooks`, `layouts`, and `stores` must not
  import a feature; features talk through the small shared contracts in
  [Cross-feature contracts](docs/architecture/contract.md#92-cross-feature-contracts)
  (`useUI`, `usePlayer`, `TrackActionsMenu`, …).

## UI and Test Contracts

- Follow `docs/development/design.md`. Mobile first: check 375px width, iOS
  safe areas, and 44px touch targets. The player and its queue persist across
  navigation.
- Every user-visible string goes through i18next with both `zh` and `en`
  translations.
- Tests protect a user-visible behavior, public contract, state transition, or
  prior regression. Choose the lowest sufficient layer and do not repeat the
  same assertion at every layer.
- Test media is synthetic: generate it with `make testdata`
  (`scripts/gen-testdata.sh`) or build files in `t.TempDir()`. Never commit
  real music, real library paths, databases, or logs. Tests that need ffmpeg
  must skip when it is not installed.
- Prefer accessible roles, names, and labels in browser tests. Do not make
  utility classes or incidental DOM ancestry a public test contract.

## Documentation

Docs describe the current system and its current decisions, not how it got
there. Where each topic belongs is in
[Documentation Rules](docs/README.md#documentation-rules). Public docs are
written in English; the translated READMEs under `docs/readme/` are the only
translations.

- Write in present tense. Do not narrate change: no "previously", "no longer",
  "now", "as before", "was removed", "migration NNNN adds", version-dated
  asides, PR/issue/commit references, or before/after comparisons.
- When behavior changes, rewrite the affected section as if it had always been
  this way instead of appending a paragraph. Search for and remove every
  statement the change contradicts, including the contract, user pages,
  operations pages, `PRIVACY.md`, and the translated READMEs.
- Compatibility that still runs is current behavior. State it as a rule
  ("`frontend-docs` runs `docs-check`"), not as a story.
- History belongs only in `docs/history/` (release notes, with pending changes
  in `unreleased.md`) and in the context of ADRs in `docs/decisions/`. Release
  notes and `unreleased.md` fold each area's changes under its title; follow
  [Release Notes](docs/development/commit-and-release.md#release-notes).
- Reference docs contain no TODOs, plans, roadmaps, ownership assignments, or
  dated measurement logs.
- State each rule or fact in one place and link to it instead of restating it.
  The contract owns precise tables; other pages summarize and link to it.
- Code comments cite contract sections by number (`contract.md §9.4`). Keep
  section numbers stable when reorganizing it.
- When the English README changes, update the translated READMEs in the same
  change. If a translation cannot be updated, remove the stale passage rather
  than leave it contradicting the English page.

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
- During development, preview the app with the development environment:
  `make docker-up` builds the current checkout into its own Compose project
  (named after the checkout directory) and starts it with
  `RAINY_MODE=development`, which signs the web app in as `RAINY_DEV_USER`
  (default `dev`, created as an administrator), so no login is needed. Other
  worktrees run their own previews: give yours a free `RAINY_DEV_PORT` (and,
  when needed, `RAINY_DEV_DATA_PATH` / `RAINY_DEV_MUSIC_PATH`), never stop,
  remove, or rebuild another checkout's container, and run `make docker-down`
  from the same checkout when you are done. Do not start ad-hoc containers or
  images for previews.
- Use `make DOCKER_IMAGE=rainy:ci docker-build smoke` (or `make ci-production`)
  for Dockerfile, entrypoint, or runtime changes.
- `make ci-local` (alias `make ci`) runs the complete GitHub Actions
  validation sequence locally.
- Keep validation proportional. A narrow change does not require all phases;
  Actions selects affected jobs and keeps the required Core check stable.
- Before every commit, run `make sensitive-check` and review any findings. It
  scans tracked changes against `HEAD` plus untracked files, so run it before
  committing rather than after.
- Do not bypass the privacy scan with source comments. An approved public
  endpoint belongs in `scripts/privacy-allowlist.json` with its owner files and
  a reason, as described in
  [Before Committing](docs/development/testing.md#before-committing). Review
  every change to that list.

## Release and Handoff

Use the repository's normal signed-commit path. If the 1Password signing agent
requires approval or is unavailable, stop and ask the user. Do not disable
commit signing, pass a no-sign flag, replace the configured signer, or
otherwise bypass the agent. Do not commit unless the user asks.

Release and migration boundaries are derived from repository state:

- Read `VERSION` for the application version (`v<major>.<minor>.<patch>`); do
  not hard-code the current release in instructions or infer it from an old
  release note.
- Numbered migrations in `internal/db/migrations` are immutable once released.
  Add the next contiguous number after the highest existing migration for a
  schema change; never edit an applied migration. Details are in
  [Migrations](docs/development/migrations.md).
- A release tag must equal `VERSION`, point at a commit on `main` whose CI
  succeeded, and have release notes at `docs/history/<tag>.md`. The release
  workflow publishes multi-arch images to Docker Hub and GHCR, then the GitHub
  Release. Run `make release-check` before tagging; the steps are in
  [Commit and release](docs/development/commit-and-release.md).

Before handoff, run the smallest sufficient targets from
[Validation Commands](#validation-commands), then `make sensitive-check`.

Public tracked code and docs must use reserved domains (`example.com`,
`192.0.2.x`), generic paths (`/path/to/music`), and obviously synthetic names.
Never commit real library paths, server addresses, credentials, or personal
listening data. Ignore rules are not a secrecy boundary; keep real libraries
and deployment details outside the repository workspace whenever possible.
