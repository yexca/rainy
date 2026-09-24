// Production image smoke test: runs the image in a disposable container with empty
// /data and /music mounts and exercises the public contract end to end.
//
//   node scripts/production-smoke.mjs <image>
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";

const image = process.argv[2];
if (!image) {
  console.error("usage: node scripts/production-smoke.mjs <image>");
  process.exit(2);
}

const docker = process.env.DOCKER || "docker";
const name = `rainy-smoke-${process.pid}`;
const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rainy-smoke-"));
const dataDir = path.join(workDir, "data");
const musicDir = path.join(workDir, "music");
fs.mkdirSync(dataDir);
fs.mkdirSync(musicDir);

function run(args, { allowFailure = false } = {}) {
  const result = spawnSync(docker, args, { encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0 && !allowFailure) {
    throw new Error(`${docker} ${args.join(" ")} failed:\n${result.stderr}`);
  }
  return result.stdout.trim();
}

function check(condition, message) {
  if (!condition) throw new Error(`check failed: ${message}`);
  console.log(`ok - ${message}`);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForHealth(base) {
  const deadline = Date.now() + 90_000;
  let lastError = "";
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`${base}/api/health`);
      if (res.ok) return;
      lastError = `HTTP ${res.status}`;
    } catch (error) {
      lastError = error instanceof Error ? error.message : String(error);
    }
    await sleep(1000);
  }
  throw new Error(`server did not become healthy: ${lastError}`);
}

async function main() {
  const ids =
    typeof process.getuid === "function"
      ? ["-e", `PUID=${process.getuid()}`, "-e", `PGID=${process.getgid()}`]
      : [];
  run([
    "run", "-d", "--name", name,
    "-p", "127.0.0.1::7650",
    ...ids,
    "-v", `${dataDir}:/data`,
    "-v", `${musicDir}:/music`,
    image,
  ]);
  const mapping = run(["port", name, "7650/tcp"]).split("\n")[0];
  const base = "http://127.0.0.1:" + mapping.slice(mapping.lastIndexOf(":") + 1);
  console.log(`container ${name} listening on ${base}`);

  await waitForHealth(base);
  check(true, "GET /api/health is healthy");

  const version = run(["exec", name, "rainy", "version"]);
  check(/rainy/i.test(version), `rainy version reports "${version.split("\n")[0]}"`);

  const processes = run(["top", name, "-o", "pid,user,args"])
    .split("\n")
    .slice(1)
    .map((line) => line.trim().split(/\s+/))
    .map(([pid, user, ...args]) => ({ pid, user, command: args.join(" ") }));
  const server = processes.find((p) => /(^|\/)rainy( serve)?$/.test(p.command));
  check(
    server !== undefined && server.user !== "root" && server.user !== "0",
    `server process runs as non-root (user ${server?.user ?? "?"})`,
  );

  const html = await fetch(`${base}/`).then((r) => r.text());
  check(html.includes('id="root"'), "GET / serves the web app");
  const manifest = await fetch(`${base}/manifest.webmanifest`);
  check(manifest.ok, "GET /manifest.webmanifest is served");

  const status = await fetch(`${base}/api/auth/status`).then((r) => r.json());
  check(status.initialized === false, "fresh instance reports initialized=false");

  const username = "smoke-admin";
  const password = `smoke-${process.pid}-${Date.now()}`;
  const setup = await fetch(`${base}/api/auth/setup`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  check(setup.status === 201, `POST /api/auth/setup creates the admin (HTTP ${setup.status})`);
  const cookie = (setup.headers.get("set-cookie") || "").split(";")[0];
  check(cookie.startsWith("rainy_session="), "setup sets the session cookie");

  const me = await fetch(`${base}/api/me`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(me.username === username && me.isAdmin === true, "GET /api/me returns the admin");

  const anonymous = await fetch(`${base}/api/home`);
  check(anonymous.status === 401, "GET /api/home without a session is 401");

  const hexPassword = Buffer.from(password, "utf8").toString("hex");
  const ping = await fetch(
    `${base}/rest/ping.view?u=${username}&p=enc:${hexPassword}&v=1.16.1&c=smoke&f=json`,
  ).then((r) => r.json());
  check(ping["subsonic-response"]?.status === "ok", "Subsonic ping authenticates");

  const wrong = await fetch(
    `${base}/rest/ping.view?u=${username}&p=wrong&v=1.16.1&c=smoke&f=json`,
  ).then((r) => r.json());
  check(wrong["subsonic-response"]?.error?.code === 40, "Subsonic rejects a wrong password with error 40");

  const scan = await fetch(`${base}/api/admin/scan`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(typeof scan.scanning === "boolean", "GET /api/admin/scan reports scanner status");

  console.log("Production smoke passed.");
}

let exitCode = 0;
try {
  await main();
} catch (error) {
  exitCode = 1;
  console.error(error instanceof Error ? error.message : String(error));
  console.error(run(["logs", "--tail", "80", name], { allowFailure: true }));
} finally {
  run(["rm", "-f", name], { allowFailure: true });
  fs.rmSync(workDir, { recursive: true, force: true });
}
process.exit(exitCode);
