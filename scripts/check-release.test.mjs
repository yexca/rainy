import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { checkRelease } from "./check-release.mjs";

test("release metadata accepts CRLF and rejects mismatched tags and absent notes", (t) => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "rainy-release-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  fs.mkdirSync(path.join(root, "docs", "history"), { recursive: true });
  fs.writeFileSync(path.join(root, "VERSION"), "v1.2.3\r\n");
  const notes = path.join(root, "docs", "history", "v1.2.3.md");
  assert.throws(() => checkRelease(root, "v1.2.4"), /does not match/);
  assert.throws(() => checkRelease(root), /ENOENT/);
  fs.writeFileSync(notes, " \n");
  assert.throws(() => checkRelease(root), /nonempty/);
  fs.writeFileSync(notes, "Synthetic release notes\n");
  assert.equal(checkRelease(root, "v1.2.3"), "v1.2.3");
  assert.equal(checkRelease(root), "v1.2.3");
  for (const version of ["1.2.3", "v01.2.3", "v1.2", "v1.2.3-rc1", "../private"]) {
    fs.writeFileSync(path.join(root, "VERSION"), version);
    assert.throws(() => checkRelease(root), /VERSION must use/);
  }
});
