Changes through v0.1.0 are summarized in [v0.1.0](v0.1.0.md).

Add user-facing changes for the next release here, grouped by area. Start with
an `[!IMPORTANT]` upgrade note when a change adds a migration or needs operator
action.

## Playback and the Web Player

- Bilingual lyrics show the translation in smaller text under each original
  line, both for lines that share a timestamp and for `original 中文` lines. A
  translation button beside the lyrics button (and a setting under Playback)
  hides or shows translations.
- Japanese and Korean lyric lines use matching Japanese or Korean fonts.

## Library Management

- Metadata editing is now a standalone entry in the sidebar and its own tab on
  phones. The other library tools and the admin pages are folded away until
  opened.
- The lyrics editor recognizes bilingual lyrics written as `original 中文`,
  previews the split, and on request rewrites timed lines as standard
  same-timestamp bilingual LRC. Stamping a line keeps its paired translation on
  the same time.

## Development and Docs

- `make frontend-test` runs unit tests for pure web helpers with Node's
  built-in test runner; `ci-frontend` includes it.
