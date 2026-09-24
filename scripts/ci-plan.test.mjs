import assert from "node:assert/strict";
import test from "node:test";
import { createPlan, planKeys, validateResults } from "./ci-plan.mjs";

const pr = (paths) =>
  createPlan({ eventName: "pull_request", ref: "refs/pull/1/merge", paths });
const jobs = { backend: "backend", frontend: "frontend", smoke: "production" };
const resultsFor = (plan) => ({
  style: { result: "success" },
  ...Object.fromEntries(
    Object.entries(jobs).map(([job, key]) => [
      job,
      { result: plan[key] ? "success" : "skipped" },
    ]),
  ),
});

test("documentation-only PRs skip expensive jobs while unknown paths require full validation", () => {
  assert.ok(
    Object.values(
      pr([
        "docs/development/testing.md",
        "docs/readme/README.zh-Hans.md",
        "README.md",
        "AGENTS.md",
      ]),
    ).every((value) => value === false),
  );
  for (const paths of [
    null,
    [],
    ["new-runtime/settings.json"],
    ["VERSION"],
    ["Makefile"],
    [".github/workflows/ci.yml"],
    ["scripts/ci-plan.mjs"],
    ["web/pnpm-lock.yaml"],
    ["web/package.json"],
    ["go.sum"],
  ]) {
    assert.ok(Object.values(pr(paths)).every(Boolean), String(paths));
  }
});

test("PR plans select the affected areas and always rebuild the image", () => {
  assert.deepEqual(pr(["web/src/features/player/store.ts"]), {
    backend: false,
    frontend: true,
    production: true,
  });
  assert.deepEqual(pr(["internal/scanner/scanner.go"]), {
    backend: true,
    frontend: false,
    production: true,
  });
  assert.deepEqual(pr(["web/embed.go"]), {
    backend: true,
    frontend: false,
    production: true,
  });
  assert.deepEqual(pr(["Dockerfile", "docker/entrypoint.sh"]), {
    backend: false,
    frontend: false,
    production: true,
  });
  assert.deepEqual(pr(["cmd/rainy/main.go", "web/src/app.tsx"]), {
    backend: true,
    frontend: true,
    production: true,
  });
});

test("main, pushes, and reusable build calls run everything", () => {
  const full = Object.fromEntries(planKeys.map((key) => [key, true]));
  assert.deepEqual(
    createPlan({ eventName: "push", ref: "refs/heads/main", paths: ["docs/a.md"] }),
    full,
  );
  assert.deepEqual(
    createPlan({ eventName: "push", ref: "refs/heads/feature" }),
    full,
  );
  assert.deepEqual(
    createPlan({
      eventName: "pull_request",
      ref: "refs/pull/1/merge",
      runBuilds: true,
      paths: ["docs/a.md"],
    }),
    full,
  );
});

test("results must match the plan: only planned-out jobs may be skipped", () => {
  const plan = pr(["web/src/app.tsx"]);
  assert.doesNotThrow(() => validateResults(plan, resultsFor(plan)));
  assert.throws(
    () =>
      validateResults(plan, { ...resultsFor(plan), frontend: { result: "skipped" } }),
    /frontend/,
  );
  assert.throws(
    () =>
      validateResults(plan, { ...resultsFor(plan), smoke: { result: "failure" } }),
    /smoke/,
  );
  assert.throws(
    () => validateResults(plan, { ...resultsFor(plan), style: { result: "failure" } }),
    /Style/,
  );
  assert.throws(() => validateResults(null, resultsFor(plan)), /invalid CI plan/);
});
