Changes through v0.1.1 are summarized in [v0.1.1](v0.1.1.md).

Add user-facing changes for the next release here, grouped by area under `##`
headings. Start with an `[!IMPORTANT]` upgrade note when a change adds a
migration or needs operator action. Each area becomes a collapsed block in the
tagged release note; see
[Commit and release](../development/commit-and-release.md#release-notes).

## Library Management

- **Organize by tags** is now a server setting instead of a switch on the
  Upload tab, and it is on by default: files you pick or drag in, link
  downloads, and online music are all placed with the rename pattern, under the
  folder you chose. Administrators can turn it off with **Organize added music
  by tags** in **Admin → Settings**. The old per-browser switch is gone, so
  uploads that used to keep their names are now organized unless the setting is
  turned off.
- When added music has no artist or album tag, Rainy says so and offers **Fill
  in tags**, which opens the tag editor on those tracks. This covers uploads,
  link downloads, and online music.

## Look And Feel

- **Immersive** goes deeper: the cover's colours now fill the whole window as
  soft colour fields, and the sidebar, the header, the page, and cards are
  see-through over them instead of sitting on flat surfaces. From tablets up
  the fields drift slowly while music plays.
- Covers now colour the app only in **Immersive**. With it off, album,
  playlist, and artist pages keep the look of the style in use like every
  other page: no colours taken from the artwork and no blurred cover behind
  the header. The **Album colours** switch is gone from Settings → Appearance;
  turn Immersive on for the cover-coloured look.
- The sidebar is narrower (208 px instead of 240 px), leaving more room for
  the library.
