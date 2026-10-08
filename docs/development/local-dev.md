# Local Development

The [Makefile](../../Makefile) is the canonical entry point for building,
running, and validating Rainy. Run `make help` for common targets. Build and
run the application with Docker by default; host tools support focused tests
and lint.

## Prerequisites

- Docker with the Compose plugin.
- GNU Make; a Bash-compatible shell and ffmpeg for synthetic media generation.
- Node.js from [`.nvmrc`](../../.nvmrc) with Corepack enabled; use the pnpm
  version pinned in `web/package.json`.
- Go 1.26 or newer for host-side backend tests and static checks.

Documentation and policy checks need only Node, Git, and Make. Development
works on Windows (PowerShell or Git Bash), macOS, and Linux. Production runs
on Linux in Docker; use platform-aware filesystem paths as required by
[contract §3](../architecture/contract.md#3-conventions).

## Test Library

```sh
make testdata
```

This generates synthetic media in `testdata/music` (ignored by Git). It covers
MP3, FLAC, M4A, Ogg Vorbis, Opus, and WAV; CJK and English tags; multi-disc
albums and compilations; covers and lyrics; and ignored NAS folders.
Re-running it recreates the generated library. Never point generation at a
real collection.

## Docker

```sh
make testdata
make docker-up        # http://127.0.0.1:7650
make docker-status
make docker-logs
make docker-down
```

The development stack in
[`deploy/compose/dev.yml`](../../deploy/compose/dev.yml) builds from this
checkout. It publishes on loopback only (`RAINY_DEV_PORT`, default 7650),
keeps its database and application state in `testdata/dev-data` mounted at
`/config`, and mounts `testdata/music` at `/data` (override
with `RAINY_DEV_MUSIC_PATH`). It defaults to debug logs and disabled periodic
scans; environment overrides still apply. Keep development data separate from
the real library.

The root [`docker-compose.yml`](../../docker-compose.yml) is for operators and
pulls published images. To validate an isolated production container instead:

```sh
make DOCKER_IMAGE=rainy:dev docker-build smoke
```

Smoke runs an already-built image with temporary media and data and cleans up
its container afterwards. It does not use development mounts.

## Frontend

For Vite iteration against the Docker backend:

```sh
make frontend-dev
```

This installs the locked dependencies and serves on <http://localhost:5173>,
proxying `/api` and `/rest` to `http://localhost:7650`. Set `RAINY_BACKEND` if
the Docker backend uses another host port. The container serves the embedded
production UI; Vite reflects frontend edits directly.

## Backend

Run host-side checks with `make backend-test` or `make ci-backend`. Rebuild
the Docker stack with `make docker-up` after backend changes.

`backend-build`, `backend-run`, and `frontend-build` remain available for
explicit host-build workflows. The binary embeds `web/dist`, so a complete
host build needs `frontend-build` first. Host runtime reads `RAINY_*` variables
and defaults to `./data` and `./config`; choose synthetic paths deliberately.
Agent work uses Docker for running the app unless the user requests a host
runtime. The host build in `ci-frontend` is validation, not a running instance.

Remove build output created during a check (`bin/`, generated `web/dist`
files, and `coverage.out`); keep the tracked `web/dist/.gitkeep`.

## Common Checks

- [Testing](testing.md): target selection, fixtures, and browser checks.
- [CI and release automation](ci.md): job planning and local equivalents.
- [Backend guidelines](backend-guidelines.md) and
  [Frontend guidelines](frontend-guidelines.md): responsibilities.
- [Migrations](migrations.md), [Design](design.md), and
  [Secure development](security.md): change-specific rules.
- [Commit and release](commit-and-release.md): signed commits and publication.

## Useful Paths

| Path | Responsibility |
| --- | --- |
| `cmd/rainy` | Entry point and CLI |
| `internal/` | Backend packages |
| `internal/api`, `internal/subsonic` | Native and compatibility handlers |
| `internal/db/migrations` | Immutable released migration chain |
| `web/src` | Frontend source |
| `scripts/` | Validation policy and synthetic media tools |
| `docs/` | Public documentation |

## Related Docs

- [Development](index.md)
- [Architecture](../architecture/index.md)
- [Architecture contract](../architecture/contract.md)
- [Agent guide](../../AGENTS.md)
