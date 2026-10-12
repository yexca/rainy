## File Editing And Deployment

- Run Docker as root by default (`PUID=0`, `PGID=0`), with nonzero IDs and explicit
  container users still supported. Existing environment overrides keep their user.
- Require two confirmation stages for music-file edits and deletion, including
  tag/cover/lyric saves, rename, encoding repair, tag rebuild, delete-to-trash, and
  permanent trash purge. Save shortcuts use the same flow; canceling preserves files.

Changes through v0.1.0 are summarized in [v0.1.0](v0.1.0.md).

Add user-facing changes for the next release here, grouped by area. Start with
an `[!IMPORTANT]` upgrade note when a change adds a migration or needs operator
action.

## Look And Feel

- **Settings → Appearance** has four styles: **Anthropic** (the new default:
  warm paper tones, a terracotta accent, and serif titles), **OpenAI**,
  **Apple**, and **Google MD**. Each changes the colours, corner radius, and
  fonts in light and dark mode. The new **Original** accent, now the default,
  uses the style's own colour; the earlier accent colours still override it,
  and an accent you already picked stays as it was. The style is also in the
  header's Appearance panel.
- A style also sets the layout. On tablets and desktops **Apple** drops the bar
  across the top: the logo moves to the top of the sidebar, and your account
  (with Settings, About, and Log out), **Quick actions**, and the
  **Appearance** button sit in its bottom-left corner. Switching styles slides
  the bar away and brings the account up, and back again. Phones keep the
  header in every style.
- **Immersive** (the switch beside the accent colour, in Settings → Appearance
  and the Appearance panel) colours the whole app — sidebar, header, player,
  and menus — with the album, playlist, or artist you are viewing, and with
  the playing song's cover on other pages. While it is on, the first accent
  swatch takes the highlight from the cover as well, and the other swatches
  keep a highlight colour you choose. It is on by default in **Apple** and
  off in the other styles.
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
  with **Quick actions** and an **Appearance** panel for theme, accent colour,
  and language, next to an account button that shows your name and role.
  **Settings** is pinned at the bottom of the sidebar. Phones get the header
  too, as a glass bar with the logo, **Quick actions**, the **Appearance**
  button, and your avatar for the account menu; page titles and back buttons
  sit just below it.
- **Quick actions** (the lightning button in the header, or Ctrl+K / ⌘K
  anywhere) opens a searchable list: play, pause, and skip while music is
  queued, every library page and your playlists, the manager and admin pages
  you can open, and your settings. Typed text that matches nothing searches the
  library. Phones open it as a bottom sheet.
- **Settings** is split into tabs: Account, Appearance, Playback, Apps, and
  Scrobbling. Older links to a settings section open its tab, **Log out** sits
  at the bottom of Account, and the version and compatibility moved to
  **About**.
- Settings, Server settings, and the Upload, Online, and Links tabs now share
  one left-aligned column under the page title, while lists and tables such as
  Tracks, Users, and Libraries stay full width. Switching tabs no longer moves
  the content sideways.

## Development And Docs

- Documentation now has dedicated development and operations entry points,
  core-boundary guidance, backend/frontend contribution rules, and reliability
  and CI guides. Agent instructions select the relevant reading path.
- `make help`, `docs-check`, `ci-policy`, and `release-check` provide clear
  local entry points. Existing `frontend-docs` and `smoke` commands remain usable.
- `RAINY_MODE=development` signs the web app in as `RAINY_DEV_USER` (default
  `dev`, created as an administrator when missing) so local previews need no
  login; the default `production` mode is unchanged. The development Compose
  stack turns it on, publishes on loopback only, and runs one container per
  checkout, so Git worktrees preview side by side with their own
  `RAINY_DEV_PORT`, `RAINY_DEV_DATA_PATH`, and `RAINY_DEV_MUSIC_PATH`.
- Documentation-only pull requests run lightweight policy checks. Code changes
  select the affected validation phases, and the required Core check rejects
  unplanned skips. Releases also verify main ancestry and nonempty release notes.
- Update the transitive source-map-js dependency to its patched release so
  the frontend dependency audit passes.
