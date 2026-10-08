import { appendFileSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

export function checkRelease(root, tag) {
  const version = readFileSync(path.join(root, "VERSION"), "utf8").trim();
  if (!/^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version))
    throw new Error("VERSION must use v<major>.<minor>.<patch>");
  if (tag !== undefined && tag !== version)
    throw new Error(`Release tag does not match VERSION ${version}`);
  const notes = path.join(root, "docs", "history", `${version}.md`);
  if (!statSync(notes).isFile() || !readFileSync(notes, "utf8").trim())
    throw new Error(`Release notes must be a nonempty file: docs/history/${version}.md`);
  return version;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const tag = process.env.GITHUB_REF_TYPE === "tag" ? process.env.GITHUB_REF_NAME : undefined;
  const version = checkRelease(repositoryRoot, tag);
  if (process.env.GITHUB_OUTPUT)
    appendFileSync(process.env.GITHUB_OUTPUT, `version_name=${version.slice(1)}\n`);
  console.log(`Release metadata verified for ${version}.`);
}
