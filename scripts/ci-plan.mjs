import { appendFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const planKeys = ["style", "backend", "frontend", "production"];

const docsPattern =
  /^(README(?:\.[\w-]+)?\.md|AGENTS\.md|CONTRIBUTING\.md|SECURITY\.md|PRIVACY\.md|LICENSE)$/;
const globalPattern =
  /^(Makefile|VERSION|\.nvmrc|\.github\/|scripts\/|go\.(mod|sum)$|web\/(package\.json|pnpm-lock\.yaml|pnpm-workspace\.yaml)$)/;
const productionPattern =
  /^(Dockerfile|\.dockerignore|docker\/|docker-compose\.yml|\.env\.example|deploy\/)/;

export function createPlan({
  eventName,
  ref,
  runBuilds = false,
  paths = null,
}) {
  const full = Object.fromEntries(planKeys.map((key) => [key, true]));
  if (ref === "refs/heads/main" || runBuilds) return full;
  if (eventName !== "pull_request") return full;
  // A missing/empty diff is not evidence that it is safe to skip validation.
  if (!paths?.length) return full;
  const plan = Object.fromEntries(planKeys.map((key) => [key, false]));
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
  if (results.policy?.result !== "success")
    throw new Error("Policy checks and CI planning must succeed");
  for (const [job, key] of Object.entries({
    style: "style",
    backend: "backend",
    frontend: "frontend",
    smoke: "production",
  })) {
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
    let paths = null;
    if (
      process.env.GITHUB_EVENT_NAME === "pull_request" &&
      /^[a-f0-9]{40}$/.test(process.env.CI_BASE_SHA || "")
    ) {
      const diff = spawnSync(
        "git",
        [
          "diff",
          "--name-only",
          "--no-renames",
          "-z",
          process.env.CI_BASE_SHA,
          "HEAD",
        ],
        { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 },
      );
      if (diff.status === 0) paths = diff.stdout.split("\0").filter(Boolean);
    }
    const plan = createPlan({
      eventName: process.env.GITHUB_EVENT_NAME,
      ref: process.env.GITHUB_REF,
      runBuilds: process.env.CI_RUN_BUILDS === "true",
      paths,
    });
    const outputs =
      [
        `plan=${JSON.stringify(plan)}`,
        ...planKeys.map((key) => `${key}=${plan[key]}`),
      ].join("\n") + "\n";
    if (process.env.GITHUB_OUTPUT)
      appendFileSync(process.env.GITHUB_OUTPUT, outputs);
    console.log(outputs.trim());
  } else {
    throw new Error("usage: node scripts/ci-plan.mjs <plan|check>");
  }
}
