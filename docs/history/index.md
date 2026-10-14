# History

This directory holds public release notes. Each tagged release has a file named
after its tag, which the release workflow publishes as the GitHub Release body;
see [Commit and release](../development/commit-and-release.md#release-notes).

Durable design decisions belong in [ADRs](../decisions/index.md). Current
behavior is documented in the user, architecture, operations, and development
docs; release notes are never its only description.

## What Belongs Here

- Release notes grouped by user-facing area.
- Upgrade notes for operators, such as migrations that call for a backup.
- [Unreleased changes](unreleased.md) collected for the next release.

Each release page opens with its upgrade notes and a short summary, then lists
its areas and folds the changes under them; the format is in
[Release Notes](../development/commit-and-release.md#release-notes).

## Releases

- [Unreleased changes](unreleased.md)
- [v0.1.2](v0.1.2.md)
- [v0.1.1](v0.1.1.md)
- [v0.1.0](v0.1.0.md)
