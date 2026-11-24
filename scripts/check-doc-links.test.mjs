import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { checkFile } from "./check-doc-links.mjs";

test("relocated README navigation and images are checked alongside Markdown links", (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "rainy-doc-links-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  for (const file of [
    "README.md",
    "docs/development/design.md",
    "docs/assets/example image.svg",
  ]) {
    const target = path.join(root, file);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, "# Example fixture\n\n## Rules\n");
  }
  const readme = path.join(root, "docs/readme/README.zh-Hans.md");
  fs.mkdirSync(path.dirname(readme), { recursive: true });
  fs.writeFileSync(
    readme,
    [
      "[English](../../README.md)",
      '<a href="../../README.md">English</a>',
      "<a",
      '  href="../development/design.md#rules">Design</a>',
      '<img src="../assets/example%20image.svg" alt="Example">',
      '<a href="./missing.md">Missing translation</a>',
      '<img src="../assets/missing.svg" alt="Missing image">',
      "[Missing Markdown](../missing.md)",
      '<a href="https://example.invalid/README.md">External</a>',
      '<a href="#local">Section</a>',
      "```html",
      '<a href="ignored.md">Code example</a>',
      "```",
      "[Repository root](/README.md)",
      "## Local",
    ].join("\n"),
  );
  assert.deepEqual(
    checkFile(readme, root).map(({ line, target }) => ({ line, target })),
    [
      { line: 6, target: "./missing.md" },
      { line: 7, target: "../assets/missing.svg" },
      { line: 8, target: "../missing.md" },
    ],
  );
});

test("heading anchors follow GitHub slugs and missing anchors are reported", (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "rainy-doc-anchors-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const contract = path.join(root, "docs/contract.md");
  fs.mkdirSync(path.dirname(contract), { recursive: true });
  fs.writeFileSync(
    contract,
    [
      "# Contract",
      "## 7. Native API (`/api`) — JSON contract",
      "### 5.1 Database",
      "## Related Docs",
      "## Related Docs",
      '<a id="pinned"></a>',
      "```markdown",
      "## Inside a fence",
      "```",
      "## [Linked](https://example.com) title",
    ].join("\n"),
  );
  const guide = path.join(root, "AGENTS.md");
  fs.writeFileSync(
    guide,
    [
      "[API](docs/contract.md#7-native-api-api--json-contract)",
      "[Database](docs/contract.md#51-database)",
      "[Second related](docs/contract.md#related-docs-1)",
      "[Pinned](docs/contract.md#pinned)",
      "[Linked](docs/contract.md#linked-title)",
      "[Fenced](docs/contract.md#inside-a-fence)",
      "[Old section](docs/contract.md#code-organization)",
      "[Own section](#agent-guide)",
      "[Missing own section](#absent)",
      "# Agent Guide",
    ].join("\n"),
  );
  assert.deepEqual(
    checkFile(guide, root).map(({ line, missingAnchor }) => ({
      line,
      missingAnchor,
    })),
    [
      { line: 6, missingAnchor: "inside-a-fence" },
      { line: 7, missingAnchor: "code-organization" },
      { line: 9, missingAnchor: "absent" },
    ],
  );
});
