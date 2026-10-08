# CI and Release Automation

The [Makefile](../../Makefile) defines validation commands. Actions provisions
tools, selects the required phases, runs those targets, and records results.
The working-tree privacy scan remains a separate pre-commit and handoff step.

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
to Style. Independent phases start after Policy.

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
release. Browser checks remain manual; see [Testing](testing.md#browser-checks).

## Releases

[Release](../../.github/workflows/release.yml) runs on version tags. It first
uses `make release-check` to require a valid `VERSION`, an identical tag, and
nonempty `docs/history/<tag>.md`. Run that target locally before tagging; on
a non-tag checkout it validates VERSION and its release notes.

Actions also verifies that the commit belongs to `main` and waits for a
successful main CI run for that exact commit. Only then does it publish
amd64/arm64 images to Docker Hub and GHCR, followed by the GitHub Release.
Concurrent attempts for the same tag are serialized without cancelling an
active publication.

Actions remain pinned to full commit SHAs. Validation has read-only repository
permissions; registry and release write permissions are limited to the jobs
that publish. See [Commit and release](commit-and-release.md) for signing and
the release procedure.
