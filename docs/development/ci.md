# CI and Release Automation

The [Makefile](../../Makefile) defines validation commands. Actions provisions
the Node and Go versions declared by `.nvmrc` and `go.mod` through the local
composite actions in `.github/actions/`, selects the required phases, runs those
targets, and records results.
The working-tree privacy scan (`make sensitive-check`) is a separate pre-commit
and handoff step that no CI job runs.

## Validation Tiers

Validation runs in two tiers that share one set of job definitions:

| Tier | Workflow | Runs on | Selects | Decides |
| --- | --- | --- | --- | --- |
| `pr` | [PR](../../.github/workflows/pr.yml) | Every pull request | Jobs for the changed paths | Whether the pull request may merge |
| `full` | [Full](../../.github/workflows/full.yml) | Every push to `main`, and any branch or tag on request | Every validation job, no advisory audit | Whether a `main` commit may be released |

[Validate](../../.github/workflows/validate.yml) holds the jobs and has no
trigger of its own. `pr.yml` and `full.yml` call it with an explicit `tier`,
so a tier never depends on the event or branch that started a run. A new push
to a pull request cancels its running PR validation. Full has no concurrency
group: every `main` commit keeps a concluded result, which a release of that
commit requires.

```text
Plan, Policy
Plan -> Style, Backend, Frontend, Smoke, Go audit, Web audit
Plan, Policy, and all planned jobs -> Gate
```

| Job | Runs |
| --- | --- |
| Plan | `make ci-plan` with the caller's tier |
| Policy | `make ci-policy`: documentation links, `actionlint` over the workflows (with shellcheck, which the runner provides), and the privacy-scanner, CI-plan, and release-check tests |
| Style | `make ci-style` |
| Backend (static), (coverage), (race) | `make ci-backend-<check>` as three parallel matrix jobs; coverage and race install ffmpeg so media tests run, and coverage uploads `coverage.out` as an artifact |
| Frontend | `make ci-frontend` |
| Smoke | `make docker-build` as `rainy:ci` with a Buildx layer cache, then `make smoke` |
| Go audit | `make backend-vuln` |
| Web audit | `make frontend-audit` |
| Gate | `make ci-results` |

Plan needs only Node and the diff, so the planned jobs start without waiting
for Policy, which runs beside them in both tiers. Go formatting belongs to
Backend static checks; frontend lint belongs to Style.

## Pull Request Plan

[`scripts/ci-plan.mjs`](../../scripts/ci-plan.mjs) owns the plan and the
Gate's verdict; its tests guard against accidental validation skips. In the
`pr` tier it compares the pull request base with the checked-out merge result,
listing both sides of renames and deleted paths:

| Changed area | Planned jobs beside Plan, Policy, and Gate |
| --- | --- |
| Only public docs, root and area agent guides, skills in `.claude/skills/`, the pull request template, or license | None |
| `cmd/`, `internal/`, or `web/embed.go` | Backend and Smoke |
| Other `web/` files | Style, Frontend, and Smoke |
| Dockerfile, entrypoint, Compose, deployment examples | Smoke |
| Makefile, scripts, workflows, composite actions, versions, dependencies, unknown paths | All |

Combined changes select the union. The advisory audits follow the dependency
manifests alone: `go.mod` or `go.sum` adds Go audit, and `web/package.json`,
`web/pnpm-lock.yaml`, or `web/pnpm-workspace.yaml` adds Web audit. Other
paths, including those that select every validation job, plan no audit, so a
new advisory for an unchanged dependency never fails an unrelated pull
request. An unavailable or empty diff selects all jobs, the audits included.

The `full` tier ignores changed paths and plans every validation job, so
whatever a pull request leaves out is validated on `main`. It plans no audit;
the daily [Dependency audit](#dependency-audit) covers `main`, so a new
advisory never holds back a release. An unknown or missing tier fails the
plan.

## Gate

Gate runs even after a failure and uses `make ci-results` to reject a missing
or invalid plan and failed, cancelled, missing, or unplanned skipped results.
Plan and Policy must succeed; only a job the plan disables may be skipped.
Each check is named after the calling job and the validation job, so the
ruleset for `main` requires the single check **PR / Gate**; adding, splitting,
or renaming a validation job never changes what the ruleset names. **Full /
Gate** reports the same verdict for a commit on `main`.

## Dependency Audit

[Dependency audit](../../.github/workflows/audit.yml) runs daily at 03:23 UTC
and on manual dispatch, in the upstream repository only, so forks do not
inherit the schedule. Its Go job runs `make backend-vuln`, which reports
vulnerabilities the code reaches in its modules and in the standard library of
the `go.mod` toolchain. Its web job runs `make frontend-audit` over the
production dependencies. A new advisory fails the run without changing the
repository. These targets query live advisory databases, so they stay outside
the validation phases; in CI they gate only a pull request that changes a
dependency manifest, through Go audit and Web audit. `make audit` runs both
locally.

Dependabot (`.github/dependabot.yml`) proposes weekly grouped updates for
GitHub Actions, Go modules, web packages, and Docker base images after a
seven-day cooldown.

## Local Equivalents

| Phase | Make target |
| --- | --- |
| Documentation, workflows, and policy | `ci-policy` |
| Workflow lint | `actionlint` |
| Web style | `ci-style` |
| Go static checks | `ci-backend-static` |
| Go suite with coverage | `ci-backend-coverage` |
| Go suite with race instrumentation | `ci-backend-race` |
| Web typecheck, helper tests, build | `ci-frontend` |
| Production image and HTTP smoke | `ci-production` |
| Go and web advisories | `audit` |
| All phases, image `rainy:ci` | `ci-local` or `ci` |

Use `make docs-check` for a documentation change. `frontend-docs` is a
compatibility alias. `smoke` and `production-smoke` both exercise an already
built image in a disposable container. Without shellcheck on the path,
`make actionlint` skips the shell snippets that CI checks. No local phase
publishes an image or release. Browser checks are manual; see
[Testing](testing.md#browser-checks).

## Releases

[Release](../../.github/workflows/release.yml) runs on tags matching
`v*.*.*` in three jobs:

1. **Verify release source** runs `make release-check`, which requires a
   `VERSION` of the form `v<major>.<minor>.<patch>`, a tag equal to it, and a
   nonempty `docs/history/<tag>.md`. On a non-tag checkout the target
   validates `VERSION` and its release notes only. The job then requires the
   tagged commit to be an ancestor of `main` and waits for a successful Full
   run whose `head_sha` is that exact commit, whether a push to `main` or a
   manual run on the tag started it. It polls every minute, fails as soon as
   every such run has finished without success (failed or cancelled), and
   gives up after 30 minutes. After a failure, start Full on the tag from the
   Actions page and rerun the release.
2. **Build and push image** builds `linux/amd64` and `linux/arm64` with Buildx
   (QEMU only for the final arm64 stage), passes the tag, commit, and build
   date as build arguments, and pushes to Docker Hub
   (`<repository owner>/rainy`, signed in as the repository owner with the
   `DOCKERHUB_TOKEN` secret) and to GHCR (`ghcr.io/<repository>`). Image tags
   drop the leading `v`: the full version, `<major>.<minor>`, and `latest`.
   Images carry OCI labels plus SBOM and provenance (`mode=max`)
   attestations.
3. **Publish GitHub release** creates the release from the existing tag, or
   updates an existing one, titled with the tag and with
   `docs/history/<tag>.md` as its body.

Concurrent attempts for the same tag are serialized without cancelling an
active publication. The release does not repeat the validation suite.

Third-party actions are pinned to full commit SHAs. Every job has a timeout,
and checkout does not persist credentials. Validation has read-only repository
permissions; registry and release write permissions are limited to the jobs
that publish. See [Commit and release](commit-and-release.md) for signing and
the release procedure.
