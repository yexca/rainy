# Contributing

Thanks for helping improve Rainy. The project is still young and changes
quickly, so small, focused changes are easiest to review and keep stable.

## Before Changing Code

Read the [architecture contract](docs/architecture/contract.md) first. It is
the single source of truth for the schema, Go package APIs, REST and Subsonic
endpoints, TypeScript types, and the design system. When code and the contract
disagree, fix the code, or update the contract in the same change and explain
why.

Then read the public docs that match the area you are changing:

- Architecture: `docs/architecture/`
- User-visible behavior: `docs/user/`
- Runtime or deployment behavior: `docs/operations/`
- Development workflow: `docs/development/`
- Security-sensitive changes: `docs/security/` and
  `docs/development/security.md`
- Major design decisions: `docs/decisions/`

## Core Rules

- Build absolute file paths only with `util.SafeJoin` (plus the symlink-aware
  `util.EnsureWithinRoot` for file mutations), and never touch files outside a
  library root or the data directory.
- Keep the build pure Go (`CGO_ENABLED=0`).
- Store timestamps as unix milliseconds, durations as float seconds, and paths
  as library-relative with `/`.
- Put every user-visible string through i18next with both `en` and `zh`
  translations.
- Design mobile first: test at 375 px width, respect iOS safe areas, and keep
  hit targets at least 44 px.
- Treat Subsonic responses as a public compatibility surface.
- Do not commit runtime data, music, databases, `secret.key`, `.env` files,
  logs, or personal paths.
- Update the contract and public documentation when behavior changes.

## Commit Messages

Use:

```text
<type>(scope): <description>
```

Types, scopes, examples, and the
[signing rule](docs/development/commit-and-release.md#signed-commits) are in
[Commit and release](docs/development/commit-and-release.md).

## Security Reports

Do not disclose suspected vulnerabilities in public issues, discussions, or
pull requests. Follow the private reporting process in the
[Security Policy](SECURITY.md).

## Validation

The [Makefile](Makefile) is the canonical validation entry point. Use the
smallest target that covers the files you changed:

```sh
make docs-check          # public documentation links
make ci-policy           # docs and validation-policy tests
make ci-style            # frontend lint
make ci-backend          # backend format, lint, module, vulnerability, vet, coverage, and race checks
make ci-frontend         # frontend audit, typecheck, helper tests, and build
make ci-production       # production image build and smoke test
```

For a complete local check, run `make ci-local`. For UI changes, also check the
app in a browser at 375 px and 1280 px; see
[Testing](docs/development/testing.md#browser-checks).

Before every commit, run the privacy scan:

```sh
make sensitive-check
```

Build and run the app with Docker by default; see
[Local development](docs/development/local-dev.md). Use direct commands only
when the Makefile has no target for the required check. `make help` lists
common commands, and [CI and release automation](docs/development/ci.md)
explains how they map to Actions jobs.

## Documentation Rules

Public documentation lives under `docs/`. Where each topic belongs is listed
in [Documentation Rules](docs/README.md#documentation-rules), and the writing
rules (present tense, no change narration, one place per fact) are in the
[agent guide](AGENTS.md#documentation). In short:

- Describe the current system. Release notes in `docs/history/` are the only
  place for change history: record a user-facing change as a bullet in the
  folded block of its area in `docs/history/unreleased.md`; see
  [Release Notes](docs/development/commit-and-release.md#release-notes).
- Update the [architecture contract](docs/architecture/contract.md) in the
  same change as the code it describes.
- Update the translated READMEs in `docs/readme/` together with `README.md`.

## Sensitive Data Check

Before committing, check for:

- Credentials, API keys, session cookies, or `.env` files with real values.
- SQLite databases, `secret.key`, logs, or cover caches.
- Music files, tag dumps, or file listings from a personal library.
- Local filesystem paths that reveal private data.

Use reserved examples such as `example.com`, `192.0.2.10`, `/path/to/music`,
and `Example Artist` in code, tests, and docs. Review the actual staged diff,
not only the files you intended to change. See
[Secure development](docs/development/security.md).
