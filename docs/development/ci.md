# CI and Release Automation

The [Makefile](../../Makefile) defines validation commands. Actions provisions
tools, selects the required phases, runs those targets, and records results.
The working-tree privacy scan (`make sensitive-check`) is a separate pre-commit
and handoff step that no CI job runs.

## Pull Requests

[CI](../../.github/workflows/ci.yml) always runs `make ci-policy` and computes
the plan with `make ci-plan`. Policy checks need only Node and Git: public
documentation links and the privacy-scanner, CI-plan, and release-check tests.

| Changed area | Additional jobs |
| --- | --- |
| Only public docs, root guides, or license | None |
| `cmd/`, `internal/`, or `web/embed.go` | Backend and production Smoke |
| Other `web/` files | Style, Frontend, and production Smoke |
| Dockerfile, entrypoint, Compose, deployment examples | Production Smoke |
| Makefile, scripts, workflows, versions, dependencies, unknown paths | All |

Combined changes select the union. An unavailable or empty diff selects all
jobs. Pushes to `main` and reusable calls with `run_builds: true` also run all
phases. Go formatting belongs to Backend static checks; frontend lint belongs
to Style. Independent phases start after Policy, and a newer push to the same
pull request or branch cancels the older run.

| Job | Runs |
| --- | --- |
| Policy | `make ci-policy`, then `make ci-plan` |
| Style | `make ci-style` |
| Backend (static), (coverage), (race) | `make ci-backend-<check>` as three parallel matrix jobs; coverage and race install ffmpeg so media tests run, and coverage uploads `coverage.out` as an artifact |
| Frontend | `make ci-frontend` |
| Smoke | `make docker-build` as `rainy:ci` with a Buildx layer cache, then `make smoke` |
| Core | `make ci-results` |

The stable required check is **Core**. It runs even after a failure and uses
`make ci-results` to reject failures, cancellations, missing results, and
unplanned skips. Only a job explicitly omitted by the plan may be skipped.
Configure branch protection to require Core rather than every conditional job.

## Local Equivalents

| Phase | Make target |
| --- | --- |
| Documentation and policy | `ci-policy` |
| Web style | `ci-style` |
| Go static checks | `ci-backend-static` |
| Go suite with coverage | `ci-backend-coverage` |
| Go suite with race instrumentation | `ci-backend-race` |
| Web audit, typecheck, helper tests, build | `ci-frontend` |
| Production image and HTTP smoke | `ci-production` |
| All phases, image `rainy:ci` | `ci-local` or `ci` |

Use `make docs-check` for a documentation change. `frontend-docs` is a
compatibility alias. `smoke` and `production-smoke` both exercise an already
built image in a disposable container. No local phase publishes an image or
release. Browser checks are manual; see [Testing](testing.md#browser-checks).

## Releases

[Release](../../.github/workflows/release.yml) runs on tags matching
`v*.*.*` in three jobs:

1. **Verify release source** runs `make release-check`, which requires a
   `VERSION` of the form `v<major>.<minor>.<patch>`, a tag equal to it, and a
   nonempty `docs/history/<tag>.md`. On a non-tag checkout the target
   validates `VERSION` and its release notes only. The job then requires the
   tagged commit to be an ancestor of `main` and waits for a successful CI run
   triggered by a push to `main` for that exact commit. It polls every minute,
   fails as soon as every such run has finished without success (failed or
   cancelled), and gives up after 30 minutes.
2. **Build and push image** builds `linux/amd64` and `linux/arm64` with Buildx
   (QEMU only for the final arm64 stage), passes the tag, commit, and build
   date as build arguments, and pushes to Docker Hub
   (`<repository owner>/rainy`, signed in as the repository owner with the
   `DOCKERHUB_TOKEN` secret) and to GHCR (`ghcr.io/<repository>`). Image tags
   drop the leading `v`: the full version, `<major>.<minor>`, and `latest`.
   Images carry OCI labels plus SBOM and provenance (`mode=max`)
   attestations.
3. **Publish GitHub release** creates the release, or updates an existing one,
   titled with the tag and with `docs/history/<tag>.md` as its body.

Concurrent attempts for the same tag are serialized without cancelling an
active publication. The release does not repeat the CI validation suite.

Third-party actions are pinned to full commit SHAs. Validation has read-only
repository permissions; registry and release write permissions are limited to
the jobs that publish. See [Commit and release](commit-and-release.md) for signing and
the release procedure.
