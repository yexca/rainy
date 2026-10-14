# Commit And Release

## Commit Format

Use:

```text
<type>(scope): <description>
```

Examples:

```text
feat(player): add sleep timer
fix(scanner): keep track ids for moved files
docs(readme): reorganize public docs
```

Common types are `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, and
`chore`. Scopes usually name an area such as `scanner`, `subsonic`, `manage`,
`player`, `api`, `web`, `docker`, or `docs`.

Before every commit, run `make sensitive-check`; see
[Testing](testing.md#before-committing).

## Signed Commits

Commits are signed with the committer's configured Git signing key. When the
signer is unavailable or waits for approval, stop and resolve it; never
disable signing, pass `--no-gpg-sign`, or switch to another signer to get a
commit through.

## Release Notes

A release note shows its area titles and folds the changes under each one, so
a reader sees what the release touches before expanding any detail. Its parts
appear in this order:

1. Upgrade notes as GitHub alerts (`> [!IMPORTANT]`, `> [!NOTE]`,
   `> [!WARNING]`), when the release has any. Start with an `[!IMPORTANT]`
   note when an upgrade needs operator attention, such as a backup before a
   migration or a changed default.
2. A short summary paragraph.
3. One folded block per user-facing area.
4. A `## Known Issue` section, when the release has one.

Only the area blocks are folded. Upgrade notes, the summary, and known issues
stay expanded because a reader must not miss them. Each area is a `<details>`
block whose summary is the area title, in place of a `##` heading:

```markdown
<details>
<summary><strong>Playback And The Web Player</strong></summary>

- One change per bullet.

</details>
```

Keep the blank line after `<summary>` and the one before `</details>`; GitHub
renders the list as literal text without them.

Group changes by user-facing area. The usual areas are:

- Playback and the web player.
- Library browsing and search.
- Library management.
- Subsonic clients.
- Operations and deployment.
- Development and docs.

Name an area after what the release changes when none of these fits.

[`docs/history/unreleased.md`](../history/unreleased.md) holds the pending
changes in the same area blocks under its heading. Add a pending change as a
bullet in the block of its area, and create the block when the area has none.
An upgrade note for a pending change, such as a new migration, goes above the
blocks as an alert.

Store each release note at `docs/history/<tag>.md`, for example
[`docs/history/v0.1.2.md`](../history/v0.1.2.md). The release workflow derives
this path from the tag, requires the file to exist, and uses it as the GitHub
Release body. The file starts directly with the release body and does not
repeat the tag as a level-one heading; the file name and the GitHub Release
title already identify the version.

## Version Source

[`VERSION`](../../VERSION) is the single source of the application version and
uses the `v<major>.<minor>.<patch>` format. The Makefile and the Docker build
inject it into `internal/buildinfo.Version` through `-ldflags` (release
builds pass the tag itself), and `rainy version`, the Subsonic
`serverVersion`, `GET /api/auth/status`, and the admin system page all report
it. Image tags drop the leading `v` (`v0.1.0` becomes `0.1.0`).

## Releasing

1. Set `VERSION` to the release tag in a normal change on `main`. In the same
   change, move the alerts and area blocks of `docs/history/unreleased.md`
   into `docs/history/<tag>.md`, add the summary above the blocks as described
   in [Release Notes](#release-notes), leave `unreleased.md` with its heading
   and a pointer to the new release, and add the new page to the
   [history index](../history/index.md).
2. Run `make release-check` to validate `VERSION` and its nonempty release
   note.
3. Land the change on `main`. The push to `main` starts the CI run the
   release waits for.
4. Tag that commit on `main` with exactly the value of `VERSION` and push the
   tag.

The release workflow re-runs the release check against the tag, requires the
tagged commit to belong to `main`, and waits up to 30 minutes for that
commit's `main` CI run to succeed; a failed or cancelled run stops the release
before anything is published. It then pushes `linux/amd64` and `linux/arm64`
images to Docker Hub (`yexca/rainy`) and GHCR and publishes the GitHub Release
with the tracked release note as its body. [CI and release
automation](ci.md#releases) describes each job.

The production [`docker-compose.yml`](../../docker-compose.yml) defaults to
`yexca/rainy:latest`, which every release moves. Operators who need
reproducible deployments pin `RAINY_IMAGE` to a release tag or digest; see
[Docker](../operations/docker.md#pin-the-image).

## Related Docs

- [Testing](testing.md)
- [Migrations](migrations.md)
- [History](../history/index.md)
- [CI and release automation](ci.md)
