console.log(`Rainy development and validation

  make docs-check          Check public documentation links
  make ci-policy           Check docs and validation/release policy tests
  make ci-style            Lint the web app
  make ci-backend          Check Go static analysis, coverage, and races
  make ci-frontend         Audit, typecheck, test, and build the web app
  make ci-production       Build and smoke-test the production image
  make ci-local            Run every Actions validation phase
  make sensitive-check     Scan the actual working-tree diff before handoff
  make release-check       Verify VERSION and matching release notes

  make testdata            Generate synthetic media (requires ffmpeg)
  make docker-up           Build and start the development Compose stack
  make docker-status       Show development containers
  make docker-logs         Read development logs
  make docker-down         Stop development containers
  make docker-build smoke  Build and test a disposable production container

Use DOCKER_IMAGE=rainy:ci to select an image tag.
See docs/development/index.md for prerequisites and focused targets.`);
