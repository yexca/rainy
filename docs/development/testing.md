# Testing

The goal is the smallest set of tests that gives strong confidence in a real
behavior or stable contract. Before adding a test, state the regression it is
meant to catch.

## Value Gate

A test is valuable when all of these are true:

1. It protects user-visible behavior, a documented contract, a security
   boundary, file safety, or a previous regression.
2. A realistic production-code defect would make it fail.
3. The behavior is not already covered at a more appropriate layer.
4. It survives an internal refactor that preserves behavior.

Path handling, authorization, file mutations, scanner decisions, Subsonic
responses, and bug fixes normally require coverage. A pass-through wrapper or a
purely visual adjustment normally does not.

## Choose the Lowest Sufficient Layer

| Behavior | Preferred layer |
| --- | --- |
| Pure helper, parser, or pattern (`util`, `lyrics`, rename patterns, `transcode.Decide`) | Table-driven unit test beside the source |
| SQL behavior or aggregates | Store test against a real migrated database (`dbtest.New(t)`) |
| HTTP authentication, validation, or response shape | Handler test through the real router |
| Scanner or management behavior on files | Test with a temporary library root and generated files |
| Responsive layout, player, and navigation | Browser check (see below) |
| Container, entrypoint, and runtime wiring | `make smoke` |

## Backend

```sh
make backend-test        # go test ./...
make backend-vet         # go vet ./...
make backend-race        # go test -race ./...
make backend-coverage    # coverage profile
make backend-format      # gofmt check
make backend-lint        # golangci-lint with the repository policy
make backend-verify      # go mod verify
make backend-vuln        # govulncheck
```

Conventions:

- Tests live beside the code as `*_test.go` and prefer table-driven cases.
- Use `dbtest.New(t)` for store-backed tests. It creates a migrated SQLite file
  in a temporary directory and closes it through `t.Cleanup`. Never check in a
  database or share a writable database between tests.
- Tests must not need the network. Tests that need ffmpeg must call `t.Skip`
  when it is unavailable, so `go test ./...` passes on a clean machine.
- Build file fixtures in `t.TempDir()`. Scanner, artwork, and manage tests
  that need real audio copy the generated `testdata/music` library into a
  temporary directory and call `t.Skip` when it has not been generated, so
  run `make testdata` first for full coverage. Never modify `testdata/music`
  in place.
- Subsonic responses are compared against golden files in
  `internal/subsonic/testdata/`. Review golden-file diffs as carefully as code:
  they are what third-party clients see.
- `internal/model` and `internal/app` contain contract tests that lock the
  JSON field names to the TypeScript contract. Update the contract and the
  test together when a field is intentionally added.

## Frontend

```sh
make frontend-install    # pnpm install with the pinned pnpm version
make frontend-typecheck  # tsc
make frontend-lint       # eslint
make frontend-test       # unit tests for pure helpers (node --test web/tests)
make frontend-audit      # dependency audit
make frontend-build      # production build into web/dist
make frontend-docs       # documentation link check
```

Pure, framework-free helpers such as the bilingual lyrics analyzer
(`src/lib/lyrics/bilingual.ts`) have unit tests in `web/tests/*.test.ts`, run
by Node's built-in test runner with type stripping. Keep such modules free of
path aliases and non-erasable TypeScript so the tests can import them directly.
Heuristics get adversarial cases: lines that must *not* be split.

## Browser Checks

Rainy is mobile-first. For any UI change, run the app against the generated
test library and check it in a browser at two widths:

- **375 px** (phone): tab bar, mini player, Now Playing sheet, safe-area
  padding, and hit targets of at least 44 px.
- **1280 px** (desktop): sidebar, player bar, side panels, and dialogs.

Check both light and dark themes and both languages when strings or layout
change. Confirm that nothing hides behind the player chrome and that
keyboard focus stays visible. The detailed contract is in
[Design](design.md).

## Test Data

Generate the local test library with `make testdata`
(`scripts/gen-testdata.sh`); see
[Local development](local-dev.md#test-library). Every tracked fixture is
public repository data:

- Use synthetic names such as `Example Artist` and `Example Album`.
- Use `example.com` or `.invalid` domains and documentation addresses such as
  `192.0.2.10`.
- Use obvious placeholders such as `synthetic-user` and `synthetic-password`
  for credentials.
- Never copy a real library path, database row, log, or tag dump from a
  personal collection into a fixture.

## Smoke Test

```sh
make docker-build smoke      # or: make ci-production
```

`make smoke` runs `DOCKER_IMAGE` (default `rainy:dev`) in a disposable
container and checks the health endpoint, the SPA shell, the Subsonic `ping`,
and first-run setup and sign-in. `make ci-local` and CI build and test the
image as `rainy:ci`.

## CI Targets

| Target | Scope |
| --- | --- |
| `make ci-style` | Go formatting, frontend lint, documentation links, and the privacy-scanner and CI-plan tests |
| `make ci-backend` | `ci-backend-static` (format, lint, module verification, vulnerabilities, vet), `ci-backend-coverage`, and `ci-backend-race` |
| `make ci-frontend` | Dependency audit, typecheck, and production build |
| `make ci-production` | `docker-build` plus `smoke` |
| `make ci-local` | Every CI validation phase above, using the image tag `rainy:ci` |
| `make ci` | Same as `ci-local` |

## Before Committing

- Run the tests that cover your change, or `make ci-local` for broad changes.
- Run `make frontend-docs` after changing any public documentation.
- Run `make sensitive-check`. It scans the working tree for secrets, private
  paths, runtime data, and non-reserved URLs before they reach a commit.
- Check `git status` and review the staged diff for databases, `.env` files,
  logs, media, and personal paths.
- Update the [contract](../architecture/contract.md) and public docs for
  behavior changes.

## Related Docs

- [Local development](local-dev.md)
- [Secure development](security.md)
- [Commit and release](commit-and-release.md)
