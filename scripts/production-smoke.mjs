// Production image smoke test: runs the image with temporary application state
// and generated music mounts, exercising the public contract end to end.
//
//   node scripts/production-smoke.mjs <image> [root|nonroot]
import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import process from "node:process";

const image = process.argv[2];
const nonroot = process.argv[3] === "nonroot";
if (!image) {
  console.error("usage: node scripts/production-smoke.mjs <image>");
  process.exit(2);
}

const docker = process.env.DOCKER || "docker";
const name = `rainy-smoke-${process.pid}`;
const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "rainy-smoke-"));
const configDir = path.join(workDir, "config");
const musicDir = path.join(workDir, "data");
fs.mkdirSync(configDir);
fs.mkdirSync(musicDir);
// A small generated PCM WAV lets the smoke test verify real file-write rejection.
const audio = Buffer.alloc(44 + 4000);
audio.write("RIFF", 0);
audio.writeUInt32LE(audio.length - 8, 4);
audio.write("WAVEfmt ", 8);
audio.writeUInt32LE(16, 16);
audio.writeUInt16LE(1, 20);
audio.writeUInt16LE(1, 22);
audio.writeUInt32LE(8000, 24);
audio.writeUInt32LE(16000, 28);
audio.writeUInt16LE(2, 32);
audio.writeUInt16LE(16, 34);
audio.write("data", 36);
audio.writeUInt32LE(4000, 40);
const audioPath = path.join(musicDir, "Synthetic Smoke Track.wav");
fs.writeFileSync(audioPath, audio);

function run(args, { allowFailure = false } = {}) {
  const result = spawnSync(docker, args, { encoding: "utf8" });
  if (result.error) throw result.error;
  if (result.status !== 0 && !allowFailure) {
    throw new Error(`${docker} ${args.join(" ")} failed:\n${result.stderr}`);
  }
  return (args[0] === "logs" ? result.stdout + result.stderr : result.stdout).trim();
}

function check(condition, message) {
  if (!condition) throw new Error(`check failed: ${message}`);
  console.log(`ok - ${message}`);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function containerURL() {
  const mapping = run(["port", name, "7650/tcp"]).split("\n")[0];
  return "http://127.0.0.1:" + mapping.slice(mapping.lastIndexOf(":") + 1);
}

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
  const ids = nonroot ? ["-e", "PUID=1000", "-e", "PGID=1000"] : [];
  run([
    "run", "-d", "--name", name,
    "-p", "127.0.0.1::7650",
    ...ids,
    "-v", `${configDir}:/config`,
    "-v", `${musicDir}:/data:ro`,
    image,
  ]);
  let base = containerURL();
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
    server !== undefined && (nonroot ? server.user === "1000" || server.user === "rainy" : server.user === "root" || server.user === "0"),
    `server process uses ${nonroot ? "explicit PUID=1000" : "root by default"} (user ${server?.user ?? "?"})`,
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

  const system = await fetch(`${base}/api/admin/system`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(system.dataDir === "/config", "application state is stored in /config");
  const libraries = await fetch(`${base}/api/admin/libraries`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(libraries.length === 1 && libraries[0].path === "/data", "default music library is /data");
  check(
    fs.existsSync(path.join(configDir, "rainy.db")) && fs.existsSync(path.join(configDir, "secret.key")),
    "database and encryption key persist in the config bind mount",
  );
  check(fs.readdirSync(musicDir).length === 1, "read-only music mount receives no application state");

  let track;
  const scanDeadline = Date.now() + 30000;
  while (Date.now() < scanDeadline && !track) {
    const page = await fetch(`${base}/api/tracks`, { headers: { Cookie: cookie } }).then((r) => r.json());
    track = page.items?.find((item) => item.path === "Synthetic Smoke Track.wav");
    if (!track) await sleep(250);
  }
  check(!!track, "synthetic WAV is scanned from the read-only music mount");
  const tags = await fetch(`${base}/api/manage/tracks/${track.id}/tags`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(tags.writable === false, "read-only music is reported as unwritable by the runtime user");
  const edit = await fetch(`${base}/api/manage/tags`, {
    method: "POST",
    headers: { Cookie: cookie, "Content-Type": "application/json" },
    body: JSON.stringify({ edits: [{ trackId: track.id, tags: { TITLE: ["Synthetic Edited Smoke Track"] } }] }),
  });
  const rejected = await edit.json();
  check(edit.status === 409 && rejected.error?.code === "readonly", "tag writes to a read-only mount fail with readonly");
  const deletion = await fetch(`${base}/api/manage/delete`, {
    method: "POST",
    headers: { Cookie: cookie, "Content-Type": "application/json" },
    body: JSON.stringify({ trackIds: [track.id] }),
  });
  const deleteRejected = await deletion.json();
  check(deletion.status === 409 && deleteRejected.error?.code === "readonly", "deletion from a read-only mount fails with readonly");
  check(fs.readFileSync(audioPath).equals(audio), "rejected file mutations leave the original synthetic file unchanged");

  run(["restart", name]);
  // Docker may assign a new ephemeral host port after a restart.
  base = containerURL();
  await waitForHealth(base);
  const restored = await fetch(`${base}/api/me`, { headers: { Cookie: cookie } }).then((r) => r.json());
  check(restored.username === username && restored.isAdmin === true, "account and session survive a container restart");

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
