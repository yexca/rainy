import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const repositoryRoot = path.resolve(scriptDirectory, "..");
const publicRootFiles = [
  "README.md",
  "AGENTS.md",
  "CONTRIBUTING.md",
  "SECURITY.md",
  "PRIVACY.md",
];
// Area agent guides outside docs/. docs/AGENTS.md is covered by the docs walk.
const agentGuideFiles = ["internal/AGENTS.md", "web/AGENTS.md"];
const skillsDirectory = ".claude/skills";
const markdownLinkPattern = /!?\[[^\]]*\]\(([^)]+)\)/g;
const htmlLinkPattern =
  /<(?:a|img)\b[^>]*?\b(?:href|src)\s*=\s*(["'])(.*?)\1/giu;
const headingPattern = /^ {0,3}#{1,6}[ \t]+(.*?)(?:[ \t]+#+)?[ \t]*$/u;
const explicitAnchorPattern = /<a\b[^>]*?\b(?:id|name)\s*=\s*(["'])(.*?)\1/giu;

function listMarkdownFiles(directory) {
  if (!fs.existsSync(directory)) return [];
  const files = [];
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    const fullPath = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...listMarkdownFiles(fullPath));
    } else if (entry.isFile() && entry.name.endsWith(".md")) {
      files.push(fullPath);
    }
  }
  return files;
}

function linkTarget(rawValue) {
  const value = rawValue.trim();
  if (value.startsWith("<")) {
    const end = value.indexOf(">");
    return end > 1 ? value.slice(1, end) : value;
  }
  return value.split(/\s+/u, 1)[0];
}

function isExternalOrSpecial(target) {
  return (
    target === "" ||
    target === "#" ||
    target.startsWith("//") ||
    /^[a-z][a-z0-9+.-]*:/iu.test(target) ||
    target.includes("<") ||
    target.includes(">")
  );
}

function resolveTarget(sourceFile, target, root) {
  const withoutFragment = target.split("#", 1)[0].split("?", 1)[0];
  if (withoutFragment === "") return sourceFile;
  let decoded = withoutFragment;
  try {
    decoded = decodeURIComponent(withoutFragment);
  } catch {
    // Keep malformed percent escapes visible as a missing path below.
  }
  const platformPath = decoded.split("/").join(path.sep);
  if (platformPath.startsWith(path.sep)) {
    return path.resolve(root, platformPath.slice(1));
  }
  return path.resolve(path.dirname(sourceFile), platformPath);
}

function fragmentOf(target) {
  const hash = target.indexOf("#");
  if (hash < 0) return "";
  const fragment = target.slice(hash + 1);
  try {
    return decodeURIComponent(fragment);
  } catch {
    return fragment;
  }
}

// Replace fenced code with empty lines so links and headings inside examples
// are ignored while line numbers stay accurate.
function visibleLines(text) {
  let fence = null;
  const visible = [];
  for (const line of text.split(/\r?\n/u)) {
    const fenceMatch = line.trimStart().match(/^(`{3,}|~{3,})/u);
    if (fenceMatch) {
      const marker = fenceMatch[1][0];
      fence = fence === null ? marker : fence === marker ? null : fence;
      visible.push("");
      continue;
    }
    visible.push(fence === null ? line : "");
  }
  return visible;
}

// GitHub's heading anchor: the rendered text, lowercased, with punctuation
// removed and each space turned into a hyphen.
export function headingSlug(heading) {
  return heading
    .replace(/!?\[([^\]]*)\]\([^)]*\)/gu, "$1")
    .replace(/<[^>]*>/gu, "")
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{M}\p{N}\p{Pc} -]/gu, "")
    .replace(/ /gu, "-");
}

const anchorCache = new Map();

export function anchorsOf(file) {
  if (anchorCache.has(file)) return anchorCache.get(file);
  const anchors = new Set();
  const seen = new Map();
  for (const line of visibleLines(fs.readFileSync(file, "utf8"))) {
    const heading = line.match(headingPattern);
    if (heading) {
      const base = headingSlug(heading[1]);
      const count = seen.get(base) ?? 0;
      seen.set(base, count + 1);
      anchors.add(count === 0 ? base : `${base}-${count}`);
    }
    for (const match of line.matchAll(explicitAnchorPattern)) {
      anchors.add(match[2]);
    }
  }
  anchorCache.set(file, anchors);
  return anchors;
}

export function checkFile(file, root = repositoryRoot) {
  const failures = [];
  // README language navigation and artwork use HTML, including multiline tags.
  const source = visibleLines(fs.readFileSync(file, "utf8")).join("\n");
  const links = [
    ...Array.from(source.matchAll(markdownLinkPattern), (match) => ({
      index: match.index,
      target: linkTarget(match[1]),
    })),
    ...Array.from(source.matchAll(htmlLinkPattern), (match) => ({
      index: match.index,
      target: match[2],
    })),
  ];
  for (const { index, target } of links) {
    if (isExternalOrSpecial(target)) continue;
    const resolved = resolveTarget(file, target, root);
    const line = source.slice(0, index).split("\n").length;
    if (!fs.existsSync(resolved)) {
      failures.push({ file, line, target, resolved });
      continue;
    }
    const fragment = fragmentOf(target);
    if (
      fragment !== "" &&
      resolved.endsWith(".md") &&
      fs.statSync(resolved).isFile() &&
      !anchorsOf(resolved).has(fragment)
    ) {
      failures.push({ file, line, target, resolved, missingAnchor: fragment });
    }
  }
  return failures.sort((a, b) => a.line - b.line);
}

function main() {
  const files = [
    ...[...publicRootFiles, ...agentGuideFiles]
      .map((file) => path.join(repositoryRoot, file))
      .filter(fs.existsSync),
    ...listMarkdownFiles(path.join(repositoryRoot, "docs")),
    ...listMarkdownFiles(path.join(repositoryRoot, skillsDirectory)),
  ];
  const failures = files.flatMap((file) => checkFile(file));

  if (failures.length > 0) {
    console.error(`Found ${failures.length} broken Markdown link(s):`);
    for (const failure of failures) {
      const relativeTarget = path.relative(repositoryRoot, failure.resolved);
      const detail = failure.missingAnchor
        ? `no heading or anchor "${failure.missingAnchor}" in ${relativeTarget}`
        : `resolved to ${relativeTarget}`;
      console.error(
        `- ${path.relative(repositoryRoot, failure.file)}:${failure.line} -> ${failure.target} (${detail})`,
      );
    }
    process.exitCode = 1;
  } else {
    console.log(
      `Checked ${files.length} Markdown files; all relative links and anchors resolve.`,
    );
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href)
  main();
