Changes through v0.1.0 are summarized in [v0.1.0](v0.1.0.md).

Add user-facing changes for the next release here, grouped by area. Start with
an `[!IMPORTANT]` upgrade note when a change adds a migration or needs operator
action.

## Look And Feel

- Songs and albums without a cover, artists without a photo, playlists without
  a cover, and radio stations now show the mascot instead of a grey icon, with
  three different covers so a grid of missing art doesn't repeat one picture.
  The lock screen and notifications show her too. Turn **Settings → Mascot →
  Illustrations** off to keep the plain placeholders.
- The player no longer keeps showing the previous song's cover when the next
  song has none.
- A new **About** page, opened from the sidebar, Settings → About, or the
  account menu, shows the version, an overview, the AI models Rainy was built
  with, the projects it learned from (Navidrome, lx-music, and Music Tag), the
  main technologies, and the license.

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
