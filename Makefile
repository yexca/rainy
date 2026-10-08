.PHONY: help docs-check frontend-docs
.PHONY: backend-format backend-lint backend-verify backend-vuln backend-test backend-coverage backend-vet backend-race backend-build backend-run
.PHONY: frontend-install frontend-dev frontend-typecheck frontend-lint frontend-test frontend-audit frontend-build
.PHONY: testdata docker-build docker-up docker-down docker-status docker-logs smoke production-smoke
.PHONY: sensitive-check sensitive-check-test privacy-check release-check release-check-test
.PHONY: ci-plan ci-plan-test ci-results ci-policy ci-style ci-backend-static ci-backend-coverage ci-backend-race ci-backend ci-frontend ci-production ci-local ci

.DEFAULT_GOAL := help

GO ?= go
PNPM ?= pnpm
NODE ?= node
DOCKER ?= docker
DOCKER_BUILD ?= $(DOCKER) build
DOCKER_BUILD_ARGS ?=
DOCKER_IMAGE ?= rainy:dev
GOLANGCI_LINT_VERSION ?= v2.13.1
GOLANGCI_LINT_TIMEOUT ?= 5m
GOLANGCI_LINT_PACKAGE := github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOLANGCI_LINT_CONFIG := scripts/golangci-lint.yml
GOVULNCHECK_PACKAGE := golang.org/x/vuln/cmd/govulncheck@v1.7.0
BUILDINFO := rainy/internal/buildinfo
# Keep build contexts, mounts, and the project name rooted here.
DOCKER_COMPOSE_DEV = $(DOCKER) compose --project-directory "$(CURDIR)" -f deploy/compose/dev.yml

ifeq ($(OS),Windows_NT)
APP_VERSION := $(shell powershell -NoProfile -Command "(Get-Content -Raw VERSION).Trim()")
BINARY := bin/rainy.exe
else
APP_VERSION := $(shell tr -d '\r\n' < VERSION)
BINARY := bin/rainy
endif
LDFLAGS := -s -w -X $(BUILDINFO).Version=$(APP_VERSION)

help:
	@$(NODE) scripts/make-help.mjs

# ---------------------------------------------------------------------------- backend

backend-format:
	$(GO) run scripts/check-go-format.go .

backend-lint:
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run --config=$(GOLANGCI_LINT_CONFIG) --timeout=$(GOLANGCI_LINT_TIMEOUT) --color=never ./...

backend-verify:
	$(GO) mod verify

backend-vuln:
	$(GO) run $(GOVULNCHECK_PACKAGE) ./...

backend-test:
	$(GO) test ./...

backend-coverage:
	$(GO) test -count=1 -covermode atomic -coverpkg ./... -coverprofile coverage.out ./...
	$(GO) tool cover -func coverage.out

backend-vet:
	$(GO) vet ./...

backend-race:
	$(GO) test -race ./...

# The binary embeds web/dist; run frontend-build first for a complete UI.
backend-build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/rainy

backend-run:
	$(GO) run -ldflags "$(LDFLAGS)" ./cmd/rainy

# --------------------------------------------------------------------------- frontend

frontend-install:
	cd web && $(PNPM) install --frozen-lockfile

frontend-dev: frontend-install
	cd web && $(PNPM) dev

frontend-typecheck: frontend-install
	cd web && $(PNPM) typecheck

frontend-lint: frontend-install
	cd web && $(PNPM) lint

frontend-test: frontend-install
	cd web && $(PNPM) test

frontend-audit: frontend-install
	cd web && $(PNPM) audit --audit-level=moderate --prod

frontend-build: frontend-install
	cd web && $(PNPM) build

docs-check:
	$(NODE) --test scripts/check-doc-links.test.mjs
	$(NODE) scripts/check-doc-links.mjs

# Compatibility with existing contributor commands.
frontend-docs: docs-check

# ----------------------------------------------------------------------- data, docker

# Generates a small synthetic tagged library into testdata/music (needs ffmpeg).
testdata:
	bash scripts/gen-testdata.sh

docker-build:
	$(DOCKER_BUILD) $(DOCKER_BUILD_ARGS) --build-arg VERSION=$(APP_VERSION) -t $(DOCKER_IMAGE) .

docker-up:
	$(DOCKER_COMPOSE_DEV) up -d --build

docker-down:
	$(DOCKER_COMPOSE_DEV) down

docker-status:
	$(DOCKER_COMPOSE_DEV) ps

docker-logs:
	$(DOCKER_COMPOSE_DEV) logs --no-color rainy

# Runs DOCKER_IMAGE in a disposable container and exercises the public contract.
production-smoke:
	$(NODE) scripts/production-smoke.mjs $(DOCKER_IMAGE)
	$(NODE) scripts/production-smoke.mjs $(DOCKER_IMAGE) nonroot

smoke: production-smoke

# ---------------------------------------------------------------------------- privacy

sensitive-check:
	$(NODE) scripts/check-sensitive.mjs

sensitive-check-test:
	$(NODE) --test scripts/check-sensitive.test.mjs

privacy-check: sensitive-check

# --------------------------------------------------------------------------------- ci

ci-plan:
	$(NODE) scripts/ci-plan.mjs plan

ci-results:
	$(NODE) scripts/ci-plan.mjs check

ci-plan-test:
	$(NODE) --test scripts/ci-plan.test.mjs

# Policy needs only Node and Git; documentation-only PRs avoid Go/pnpm setup.
ci-policy: docs-check sensitive-check-test ci-plan-test release-check-test

ci-style: frontend-lint

ci-backend-static: backend-format backend-lint backend-verify backend-vuln backend-vet

# Coverage executes the full suite; keep the separate race-instrumented run.
ci-backend-coverage: backend-coverage

ci-backend-race: backend-race

ci-backend: ci-backend-static ci-backend-coverage ci-backend-race

ci-frontend: frontend-audit frontend-typecheck frontend-test frontend-build

ci-production: docker-build smoke

release-check:
	$(NODE) scripts/check-release.mjs

release-check-test:
	$(NODE) --test scripts/check-release.test.mjs

# ci-local follows every GitHub Actions validation phase.
ci-local: DOCKER_IMAGE := rainy:ci
ci-local: ci-policy ci-style ci-backend ci-frontend ci-production

ci: ci-local
