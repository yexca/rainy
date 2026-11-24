import { appendFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

// Validation runs in one of two tiers. `pr` selects jobs from the changed
// paths and decides whether a pull request may merge. `full` runs every
// validation job and decides whether a commit on main may be released.
export const tiers = ["pr", "full"];

export const validationKeys = ["style", "backend", "frontend", "production"];
// Advisory audits query live databases, so only a pull request that changes
// the matching dependency manifest plans one. The daily Dependency audit
// workflow covers main, so the full tier never plans them and a new advisory
// cannot hold back a release.
export const auditKeys = ["go_audit", "web_audit"];
export const planKeys = [...validationKeys, ...auditKeys];

// The jobs every tier runs unconditionally, beside the planned ones.
export const alwaysRun = ["plan", "policy"];

// The planned jobs and the plan key that enables each of them.
export const plannedJobs = {
  style: "style",
  backend: "backend",
  frontend: "frontend",
  smoke: "production",
  "go-audit": "go_audit",
  "web-audit": "web_audit",
};

const docsPattern =
  /^(README(?:\.[\w-]+)?\.md|(?:[\w.-]+\/)*AGENTS\.md|\.claude\/skills\/.+\.md|CONTRIBUTING\.md|SECURITY\.md|PRIVACY\.md|LICENSE|\.github\/pull_request_template\.md)$/;
const globalPattern =
  /^(Makefile|VERSION|\.nvmrc|\.github\/|scripts\/|go\.(mod|sum)$|web\/(package\.json|pnpm-lock\.yaml|pnpm-workspace\.yaml)$)/;
const productionPattern =
  /^(Dockerfile|\.dockerignore|docker\/|docker-compose\.yml|\.env\.example|deploy\/)/;
const goManifestPattern = /^go\.(mod|sum)$/;
const webManifestPattern =
  /^web\/(package\.json|pnpm-lock\.yaml|pnpm-workspace\.yaml)$/;

const fill = (keys, value) =>
  Object.fromEntries(keys.map((key) => [key, value]));

export function createPlan({ tier, paths = null }) {
  if (!tiers.includes(tier)) throw new Error(`Unknown CI tier: ${tier}`);
  if (tier === "full")
    return { ...fill(validationKeys, true), ...fill(auditKeys, false) };
  // A missing/empty diff is not evidence that it is safe to skip validation.
  if (!paths?.length) return fill(planKeys, true);
  return {
    ...validationPlan(paths),
    go_audit: paths.some((path) => goManifestPattern.test(path)),
    web_audit: paths.some((path) => webManifestPattern.test(path)),
  };
}

function validationPlan(paths) {
  const full = fill(validationKeys, true);
  const plan = fill(validationKeys, false);
  for (const path of paths) {
    if (path.startsWith("docs/") || docsPattern.test(path)) continue;
    if (globalPattern.test(path)) return full;
    if (productionPattern.test(path)) {
      plan.production = true;
    } else if (
      path.startsWith("cmd/") ||
      path.startsWith("internal/") ||
      path === "web/embed.go"
    ) {
      Object.assign(plan, { backend: true, production: true });
    } else if (path.startsWith("web/")) {
      Object.assign(plan, { style: true, frontend: true, production: true });
    } else {
      return full;
    }
  }
  return plan;
}

export function validateResults(plan, results) {
  if (!plan || planKeys.some((key) => typeof plan[key] !== "boolean"))
    throw new Error("Missing or invalid CI plan");
  for (const job of alwaysRun) {
    const result = results[job]?.result;
    if (result !== "success")
      throw new Error(`CI job ${job} must succeed: ${result ?? "missing"}`);
  }
  for (const [job, key] of Object.entries(plannedJobs)) {
    const result = results[job]?.result;
    if (
      result !== "success" &&
      !(plan[key] === false && result === "skipped")
    ) {
      throw new Error(
        `CI job ${job} did not satisfy its plan: ${result ?? "missing"}`,
      );
    }
  }
}

function changedPaths(base) {
  if (!/^[a-f0-9]{40}$/.test(base || "")) return null;
  // Without rename detection a renamed file lists both its old and new path.
  const diff = spawnSync(
    "git",
    ["diff", "--name-only", "--no-renames", "-z", base, "HEAD"],
    { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 },
  );
  if (diff.status !== 0) return null;
  return diff.stdout.split("\0").filter(Boolean);
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  if (process.argv[2] === "check") {
    validateResults(
      JSON.parse(process.env.CI_PLAN || "null"),
      JSON.parse(process.env.CI_RESULTS || "{}"),
    );
    console.log("All planned CI jobs succeeded.");
  } else if (process.argv[2] === "plan") {
    const tier = process.env.CI_TIER;
    const paths = tier === "pr" ? changedPaths(process.env.CI_BASE_SHA) : null;
    const plan = createPlan({ tier, paths });
    const outputs =
      [
        `plan=${JSON.stringify(plan)}`,
        ...planKeys.map((key) => `${key}=${plan[key]}`),
      ].join("\n") + "\n";
    if (process.env.GITHUB_OUTPUT)
      appendFileSync(process.env.GITHUB_OUTPUT, outputs);
    console.log(outputs.trim());
  } else {
    throw new Error("usage: CI_TIER=<pr|full> node scripts/ci-plan.mjs <plan|check>");
  }
}
