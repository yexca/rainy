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

- Build absolute file paths only with `util.SafeJoin`, and never touch files
  outside a library root or the data directory.
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

Examples:

```text
feat(player): add sleep timer
fix(scanner): keep track ids for moved files
docs(operations): describe Unraid permissions
```

See [Commit and release](docs/development/commit-and-release.md).

## Security Reports

Do not disclose suspected vulnerabilities in public issues, discussions, or
pull requests. Follow the private reporting process in the
[Security Policy](SECURITY.md).

## Validation

The [Makefile](Makefile) is the canonical validation entry point. Use the
smallest target that covers the files you changed:

```sh
make frontend-docs       # public documentation link check
make ci-style            # Go formatting, frontend lint, doc links, and script tests
make ci-backend          # backend format, lint, tests, vet, race, and vulnerability checks
make ci-frontend         # frontend audit, typecheck, and build
make ci-production       # production image build and smoke test
```

For a complete local check, run `make ci-local`. For UI changes, also check the
app in a browser at 375 px and 1280 px; see
[Testing](docs/development/testing.md#browser-checks).

Before every commit, run the privacy scan:

```sh
make sensitive-check
```

Direct commands are useful for focused iteration, but they do not replace the
Makefile targets:

```sh
go build ./... && go test ./... && go vet ./...
cd web && pnpm install && pnpm typecheck && pnpm lint && pnpm build
```

## Documentation Rules

- Put stable public documentation under `docs/`; the index is
  [docs/README.md](docs/README.md).
- Keep the [architecture contract](docs/architecture/contract.md) precise and
  current. Other architecture pages summarize and link to it.
- Put user-facing behavior in `docs/user/`.
- Put runtime setup, configuration, and troubleshooting in `docs/operations/`.
- Put local development and test instructions in `docs/development/`.
- Capture durable architectural decisions as ADRs in `docs/decisions/`.
- Record user-facing changes in `docs/history/unreleased.md`.
- Keep the translated READMEs in `docs/readme/` in step with `README.md`.

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
