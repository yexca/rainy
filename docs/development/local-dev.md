# Local Development

The [Makefile](../../Makefile) is the canonical entry point for building,
running, and validating Rainy. Direct commands are shown for focused
iteration.

## Prerequisites

- Go 1.26 or newer.
- Node.js (the version in [`.nvmrc`](../../.nvmrc)) with Corepack enabled, so
  the pnpm version pinned in `web/package.json` is used.
- ffmpeg, for transcoding and for generating the test library.
- Docker with the Compose plugin, for container builds and the smoke test.

Development works on Windows (Git Bash or PowerShell), macOS, and Linux.
Production runs on Linux in Docker, so never hard-code `/` or `\` for OS paths;
see [contract §3](../architecture/contract.md#3-conventions).

## Test Library

Generate a small, fully tagged library in `testdata/music` (ignored by Git):

```sh
make testdata
# or: bash scripts/gen-testdata.sh
```

It covers MP3, FLAC, M4A, Ogg Vorbis, Opus, and WAV; Chinese, Japanese, and
English metadata; a multi-disc album; a compilation; embedded and folder
covers; synced and embedded lyrics; an untagged file; and hidden and `@eaDir`
folders that the scanner must ignore. Re-running it recreates the directory.

## Backend

```sh
make backend-run
```

This runs `go run ./cmd/rainy` with the version from `VERSION` and serves on
port 7650. Like the binary, it reads `RAINY_*` variables from your environment
and otherwise uses `./music` and `./data`. To use the test library with the
Vite dev server:

```sh
RAINY_MUSIC_DIR=./testdata/music RAINY_DATA_DIR=./testdata/data \
RAINY_DEV_CORS=http://localhost:5173 make backend-run
```

The server embeds `web/dist`, so build the frontend once (`make frontend-build`)
if you want to use the app on port 7650 directly.

## Frontend

```sh
make frontend-install
make frontend-dev
# or: cd web && pnpm install && pnpm dev
```

The Vite dev server listens on <http://localhost:5173> and proxies `/api` and
`/rest` to `http://localhost:7650`. Set `RAINY_BACKEND` to proxy elsewhere.

## Docker

Build a local image (tagged `rainy:dev`) and run the development Compose stack,
which builds from source and mounts the generated test library:

```sh
make docker-build
make docker-up        # http://127.0.0.1:7650
make docker-status
make docker-logs
make docker-down
```

The development stack is defined in
[`deploy/compose/dev.yml`](../../deploy/compose/dev.yml). It publishes the
port on loopback only (`RAINY_DEV_PORT`, default 7650), keeps its data in
`testdata/dev-data`, mounts `testdata/music` (or `RAINY_DEV_MUSIC_PATH`), logs
at debug level, and disables periodic scans. The root
[`docker-compose.yml`](../../docker-compose.yml) is the production file for
operators and only pulls published images.

## Common Checks

- Tests and pre-commit checks: [Testing](testing.md)
- Database changes: [Migrations](migrations.md)
- UI changes: [Design](design.md)
- Security-sensitive changes: [Secure development](security.md)
- Commit format and releases: [Commit and release](commit-and-release.md)

## Useful Paths

- Entry point and CLI: `cmd/rainy`
- Backend packages: `internal/`
- Native API handlers: `internal/api`
- Subsonic handlers: `internal/subsonic`
- Migrations: `internal/db/migrations`
- Frontend source: `web/src`
- Public docs: `docs`

## Related Docs

- [Architecture](../architecture/index.md)
- [Architecture contract](../architecture/contract.md)
- [Agent guide](../../AGENTS.md)
