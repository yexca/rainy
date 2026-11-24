import assert from "node:assert/strict";
import test from "node:test";
import {
  alwaysRun,
  auditKeys,
  createPlan,
  planKeys,
  plannedJobs,
  validateResults,
  validationKeys,
} from "./ci-plan.mjs";

const pr = (paths) => createPlan({ tier: "pr", paths });
const noAudit = { go_audit: false, web_audit: false };
const validates = (plan) => validationKeys.every((key) => plan[key]);
const resultsFor = (plan) => ({
  ...Object.fromEntries(alwaysRun.map((job) => [job, { result: "success" }])),
  ...Object.fromEntries(
    Object.entries(plannedJobs).map(([job, key]) => [
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
        "internal/AGENTS.md",
        "web/AGENTS.md",
        ".claude/skills/release/SKILL.md",
        ".github/pull_request_template.md",
      ]),
    ).every((value) => value === false),
  );
  for (const paths of [
    null,
    [],
    ["new-runtime/settings.json"],
    [".claude/settings.json"],
    [".claude/skills/release/check.sh"],
    ["VERSION"],
    ["Makefile"],
    [".github/workflows/validate.yml"],
    [".github/workflows/pr.yml"],
    [".github/actions/setup-go/action.yml"],
    ["scripts/ci-plan.mjs"],
    ["web/pnpm-lock.yaml"],
    ["web/package.json"],
    ["go.sum"],
  ]) {
    assert.ok(validates(pr(paths)), String(paths));
  }
});

test("PR plans select the affected areas and always rebuild the image", () => {
  assert.deepEqual(pr(["web/src/features/player/store.ts"]), {
    style: true,
    backend: false,
    frontend: true,
    production: true,
    ...noAudit,
  });
  assert.deepEqual(pr(["internal/scanner/scanner.go"]), {
    style: false,
    backend: true,
    frontend: false,
    production: true,
    ...noAudit,
  });
  assert.deepEqual(pr(["web/embed.go"]), {
    style: false,
    backend: true,
    frontend: false,
    production: true,
    ...noAudit,
  });
  assert.deepEqual(pr(["Dockerfile", "docker/entrypoint.sh"]), {
    style: false,
    backend: false,
    frontend: false,
    production: true,
    ...noAudit,
  });
  assert.deepEqual(pr(["cmd/rainy/main.go", "web/src/app.tsx"]), {
    style: true,
    backend: true,
    frontend: true,
    production: true,
    ...noAudit,
  });
});

test("dependency manifests plan their advisory audit, and nothing else does", () => {
  for (const path of ["go.mod", "go.sum"]) {
    assert.deepEqual(
      auditKeys.map((key) => pr([path])[key]),
      [true, false],
      path,
    );
  }
  for (const path of [
    "web/package.json",
    "web/pnpm-lock.yaml",
    "web/pnpm-workspace.yaml",
  ]) {
    assert.deepEqual(
      auditKeys.map((key) => pr([path])[key]),
      [false, true],
      path,
    );
  }
  assert.ok(auditKeys.every((key) => pr(["go.sum", "web/pnpm-lock.yaml"])[key]));
  // Global and unknown paths validate everything but query no advisory database.
  for (const paths of [
    ["Makefile"],
    [".github/workflows/validate.yml"],
    ["new-runtime/settings.json"],
    ["internal/scanner/scanner.go", "web/src/app.tsx"],
  ]) {
    assert.ok(auditKeys.every((key) => pr(paths)[key] === false), String(paths));
  }
  // Without a diff the planner cannot rule a manifest change out.
  for (const paths of [null, []]) {
    assert.ok(planKeys.every((key) => pr(paths)[key]), String(paths));
  }
});

test("the full tier ignores changed paths and plans every validation job but no audit", () => {
  const full = {
    ...Object.fromEntries(validationKeys.map((key) => [key, true])),
    ...noAudit,
  };
  assert.deepEqual(createPlan({ tier: "full", paths: ["go.sum"] }), full);
  assert.deepEqual(createPlan({ tier: "full" }), full);
});

test("an unknown or missing tier is rejected instead of defaulting", () => {
  for (const tier of [undefined, "", "main", "PR"]) {
    assert.throws(() => createPlan({ tier, paths: ["docs/a.md"] }), /Unknown CI tier/);
  }
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
  const dependencies = pr(["go.sum"]);
  assert.doesNotThrow(() => validateResults(dependencies, resultsFor(dependencies)));
  for (const result of ["failure", "skipped", "cancelled", undefined]) {
    assert.throws(
      () =>
        validateResults(dependencies, {
          ...resultsFor(dependencies),
          "go-audit": { result },
        }),
      /go-audit/,
    );
  }
  for (const job of alwaysRun) {
    for (const result of ["failure", "skipped", "cancelled", undefined]) {
      assert.throws(
        () => validateResults(plan, { ...resultsFor(plan), [job]: { result } }),
        new RegExp(job),
      );
    }
  }
  assert.throws(() => validateResults(null, resultsFor(plan)), /invalid CI plan/);
});

test("documentation-only results require policy success and permit planned skips", () => {
  const plan = pr(["docs/README.md"]);
  assert.doesNotThrow(() => validateResults(plan, resultsFor(plan)));
  for (const result of ["failure", "cancelled", undefined]) {
    assert.throws(
      () => validateResults(plan, { ...resultsFor(plan), style: { result } }),
      /style/,
    );
  }
  const frontend = pr(["web/src/app.tsx"]);
  assert.throws(
    () => validateResults(frontend, { ...resultsFor(frontend), style: { result: "skipped" } }),
    /style/,
  );
});
