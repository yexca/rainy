# Agent Guide

Read these first:

- `README.md`
- `docs/overview.md`
- `docs/architecture/index.md`
- `docs/architecture/contract.md` (the precise contract: schema, Go package APIs,
  native and Subsonic endpoints, TypeScript types, design system)
- `docs/architecture/library-management.md`
- `docs/development/design.md`
- `SECURITY.md`
- `docs/development/security.md`

Keep code and `docs/architecture/contract.md` in sync. When they disagree, fix
the code, or update the contract in the same change and say so.

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
cmd -> app -> api / subsonic / server -> manage / scanner / artwork / transcode / lyrics -> store / tags / auth / events -> db / model / util / config
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
Actions exercise the same commands.

- Use the smallest sufficient target: `make frontend-docs` for public
  documentation, `make ci-style` for formatting, lint, docs, and CI-policy
  checks, `make ci-backend` for Go behavior, and `make ci-frontend` for the web
  app.
- Use `make DOCKER_IMAGE=rainy:ci docker-build smoke` (or `make ci-production`)
  for Dockerfile, entrypoint, or runtime changes.
- `make ci-local` runs the complete GitHub Actions sequence locally.
- Before every commit, run `make sensitive-check` against the actual working
  tree diff and review any findings. An approved public endpoint belongs in
  `scripts/privacy-allowlist.json` with its owner files and a reason; review
  every change to that list.

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
  Release. See `docs/development/commit-and-release.md`.

Before handoff, run validation proportional to the change (see above) plus
`make sensitive-check`.

Public tracked code and docs must use reserved domains (`example.com`,
`192.0.2.x`), generic paths (`/path/to/music`), and obviously synthetic names.
Never commit real library paths, server addresses, credentials, or personal
listening data.
