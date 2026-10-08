# Backend Guidelines

Read [Core boundaries](../architecture/core-boundaries.md) and the relevant
[backend contracts](../architecture/contract.md#5-backend-contracts) first.

## Responsibilities

- Compose dependencies in `internal/app`. Keep HTTP handlers focused on
  parsing, authorization, a service/store call, and encoding.
- Put file workflows in `internal/manage`, scanning in `internal/scanner`,
  tag access in `internal/tags`, and shared SQL in `internal/store`.
- Direct dependencies downward as described in [Backend](../architecture/backend.md).
  Services never import transport or app packages; primitives import nothing internal.
- Pass `context.Context` first, wrap errors with context, and log with `slog`.
  Public errors use stable codes and do not expose paths or credentials.
- Keep Go dependencies compatible with `CGO_ENABLED=0`.

## Changes That Need Contract Review

Keep Go JSON models and TypeScript types aligned. Preserve Subsonic error
codes and response shapes, and review its golden files when compatibility
changes. Keep timestamps in Unix milliseconds, durations in float seconds,
and stored paths relative with `/`.

For file effects, keep the lock, mutation, rescan, edit log, and library event
in one workflow. Cancellation, read-only media, unsafe paths, interrupted
operations, and offline mounts are normal cases to handle. Never expand the
ffmpeg read fallback into a tag write path.

## Validation

Use `make backend-test` for behavior and `make ci-backend` for the complete Go
phase. Use `dbtest.New(t)` for persistence and temporary synthetic media for
filesystem tests. External ffmpeg tests skip when the tool is unavailable.
See [Testing](testing.md), [Migrations](migrations.md), and
[Secure development](security.md).
