> [!IMPORTANT]
> The default mounts are now `./config:/config` for application state and
> `./data:/data` for music. Existing installations must preserve their database
> and `secret.key` and update the existing library path when changing mounts;
> see [Upgrading the mount layout](../operations/docker.md#upgrading-the-mount-layout).

Changes through v0.1.0 are summarized in [v0.1.0](v0.1.0.md).

Add user-facing changes for the next release here, grouped by area. Start with
an `[!IMPORTANT]` upgrade note when a change adds a migration or needs operator
action.

## Operations And Deployment

- Database, encryption key, caches, trash, uploads, and yt-dlp now default to
  `/config`; user music defaults to `/data`. The entrypoint fixes ownership
  only in the application-state directory. Existing path environment variables
  remain available for custom and legacy mounts.

## Look And Feel

- Songs and albums without a cover, artists without a photo, playlists without
  a cover, and radio stations now show the mascot instead of a grey icon, with
  three different covers so a grid of missing art doesn't repeat one picture.
  The lock screen and notifications show her too. Turn **Illustrations** off
  under **Settings → Appearance → Mascot** to keep the plain placeholders.
- The player no longer keeps showing the previous song's cover when the next
  song has none.
- A new **About** page, opened from the sidebar or the account menu, shows
  the version, the Subsonic compatibility, an overview, the AI models Rainy was built
  with, the projects it learned from (Navidrome, lx-music, and Music Tag), the
  main technologies, and the license.
- On tablets and desktops, the top-right corner of the header has a round tray
  with Search and an **Appearance** panel for theme, accent colour, and
  language, next to an account button that shows your name and role.
  **Settings** is pinned at the bottom of the sidebar. On phones, theme and
  language are in the account menu under **Appearance**.
- **Settings** is split into tabs: Account, Appearance, Playback, Apps, and
  Scrobbling. Older links to a settings section open its tab, **Log out** sits
  at the bottom of Account, and the version and compatibility moved to
  **About**.

## Development And Docs

- Documentation now has dedicated development and operations entry points,
  core-boundary guidance, backend/frontend contribution rules, and reliability
  and CI guides. Agent instructions select the relevant reading path.
- `make help`, `docs-check`, `ci-policy`, and `release-check` provide clear
  local entry points. Existing `frontend-docs` and `smoke` commands remain usable.
- Documentation-only pull requests run lightweight policy checks. Code changes
  select the affected validation phases, and the required Core check rejects
  unplanned skips. Releases also verify main ancestry and nonempty release notes.
- Update the transitive source-map-js dependency to its patched release so
  the frontend dependency audit passes.
