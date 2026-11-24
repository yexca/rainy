# Agent Guide

Rainy is a self-hosted music server: a Go service (`cmd/`, `internal/`) with a
Subsonic/OpenSubsonic API, embedding a React web player and library manager
(`web/`).

## Where To Read Next

Before changing files under a path, read its guide, then open only the doc
pages it names.

| Path | Guide |
| --- | --- |
| `cmd/`, `internal/` (services, handlers, schema, scanning, files, outbound requests) | [internal/AGENTS.md](internal/AGENTS.md) |
| `web/` (player, UI, i18n, shared types) | [web/AGENTS.md](web/AGENTS.md) |
| `docs/`, `README.md`, `PRIVACY.md`, any Markdown, agent guides and skills | [docs/AGENTS.md](docs/AGENTS.md) |
| `Dockerfile`, `docker/`, `deploy/`, `.env.example` | [Docker](docs/operations/docker.md), [Configuration](docs/operations/configuration.md), [Deployment security](docs/operations/security.md) |
| `Makefile`, `scripts/`, `.github/` | [CI and release automation](docs/development/ci.md) |
| A database migration or a release | the `migration` and `release` skills in [.claude/skills/](.claude/skills/) |

## Product Invariants

- Files on disk are the source of truth for music and media; SQLite is their
  index and stores user state. File mutations are explicit, hold the scanner
  lock, are reversible or logged, rescan affected paths, and publish a
  `library` event.
- Library paths are library-relative with `/` and resolve to host paths only
  through the checks in [internal/AGENTS.md](internal/AGENTS.md#files-and-paths).
- Track IDs are random and stable across moves, renames, and tag edits.
  Album, artist, and genre IDs are deterministic hashes. Scans preserve
  indexed tracks when a root or large subtree is unavailable.
- Public routes are limited to the app shell, health, authentication status,
  login, logout, and setup before users exist. Backend handlers enforce
  authentication and roles for protected operations.
- Never expose host paths, credentials, full untrusted URLs, stack traces, or
  raw filesystem errors in public responses. Hash session and API-key secrets;
  encrypt stored passwords and service credentials; keep secrets out of logs.
- Outbound features are disabled by default and start only from an explicit
  user action or linked scrobbling account. A new outbound path follows the
  [outbound checklist](internal/AGENTS.md#outbound-requests).
- Development authentication is local-only and explicit. Do not expose
  development mode or its automatically signed-in user to an untrusted network.
- English is the base locale and the fallback. The backend stores and returns
  codes or English text, never localized display strings.
- The [contract](docs/architecture/contract.md) changes with the code it
  describes: the native `/api` with `web/src/lib/api/types.ts`, and Subsonic
  responses with the golden files in `internal/subsonic/testdata`.

## Validation

The Makefile is the CI contract; run the smallest sufficient target.

| Change | Target |
| --- | --- |
| Docs and links | `make docs-check` |
| Formatting and static style | `make ci-style` |
| Go backend | `make ci-backend` (focused: `ci-backend-static`, `ci-backend-coverage`, `ci-backend-race`) |
| Frontend | `make ci-frontend` |
| Image and runtime | `make ci-production` |
| Everything | `make ci-local` or `make ci` |

- Toolchain versions come from `.nvmrc` and `go.mod` through
  `.github/actions/setup-node` and `.github/actions/setup-go`.
- Tests protect a user-visible behavior, public contract, state transition,
  security boundary, or regression at the lowest sufficient layer; see the
  [value gate](docs/development/testing.md#value-gate). Test code is evidence,
  not proof: inspect production paths independently and do not treat passing
  tests as a security guarantee.
- Use synthetic data, reserved domains and paths, and no real outbound
  requests.

## Commits and Handoff

- Begin with `git status --short`; never use destructive reset or checkout
  commands, and preserve unrelated user changes.
- Commit, amend, rebase, push, or publish PRs, issues, or releases only when
  the user explicitly asks. Follow the
  [commit format](docs/development/commit-and-release.md#commit-format) and
  [signed commits](docs/development/commit-and-release.md#signed-commits).
- Before a commit run `make sensitive-check`, review every finding and
  `git diff --cached`, and stage only the requested scope. Fix the source of a
  finding; never hide it.
- Never commit `.env`, credentials, private paths, personal media, logs,
  databases/WAL, runtime media, or generated indexes. Ignore rules are not a
  secrecy boundary.
- Never invent identifiers: the version comes from `VERSION`, migration
  numbers from `internal/db/migrations/`, and issue, PR, and tag names from
  the repository or the user.
- Before handoff, run proportional validation and `make sensitive-check`,
  remove generated artifacts, and review a final `git status --short` and
  diff. Do not claim a test result that was not run.

## Documentation

When behavior changes, update its docs in the same change. Docs describe the
current system; the rules are in [docs/AGENTS.md](docs/AGENTS.md).
