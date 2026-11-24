---
name: release
description: Prepare a Rainy release when the user asks (VERSION bump, release note from unreleased.md, release check, and validation).
---

# Prepare a Release

Run only when the user asks for a release and names the version, or approves
the one you propose from `VERSION`. Never invent a version or tag. The rules
are in [Commit and release](../../../docs/development/commit-and-release.md).

1. Run `git status --short` and stop if unrelated user changes would be
   affected.
2. Set `VERSION` to the release tag
   ([Version source](../../../docs/development/commit-and-release.md#version-source)).
3. Move the alerts and area blocks of `docs/history/unreleased.md` into
   `docs/history/<tag>.md`, add the summary above the blocks in the order and
   format of
   [Release notes](../../../docs/development/commit-and-release.md#release-notes),
   leave `unreleased.md` with its heading and a pointer to the new release,
   and add the page to `docs/history/index.md`.
4. Run `make release-check`, `make ci`, and `make sensitive-check`; review the
   findings and the diff.
5. Commit, push, and tag only when the user asks. The tag is exactly the
   value of `VERSION` on a `main` commit; the release workflow then waits for
   its Full run as described in
   [Releasing](../../../docs/development/commit-and-release.md#releasing).
