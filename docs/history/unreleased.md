# Unreleased

Changes through v0.2.0 are summarized in [v0.2.0](v0.2.0.md).

> [!NOTE]
> **Apple is the default style.** Browsers that never picked a style switch to
> Apple, with **Immersive** on, after the upgrade. Pick another style or turn
> Immersive off in **Settings → Appearance**; the choice is kept per browser.

<details>
<summary><strong>Appearance</strong></summary>

- New browsers start in the Apple style with Immersive on.
- In immersive mode, the next cover's colours shift smoothly across the app
  instead of cross-fading a still picture of the old screen, so scrolling
  while the colours change no longer leaves a ghost of the previous page.
- Opening another album, playlist, or artist keeps the current colours while
  its page loads and then changes them once, instead of briefly showing the
  playing song's colours or the plain theme. Album colours are prepared when
  you hover or press the album.
- Loading placeholders appear only when a page takes longer than a moment,
  so quick page changes no longer flash them.
- Accent colours follow each style's design source: Apple uses its system
  colours (Purple, Pink, Orange besides its blue), Google MD Material 3 tones
  (Green, Purple, Orange), OpenAI its Apps SDK colours (Blue, Purple, Orange),
  and Anthropic its brand Blue and Green. Each style's own accent moves to the
  same source. A saved Rain, Rose, Violet, Emerald, or Amber becomes the
  nearest hue, and Graphite becomes Original. The swatches show the colour
  you get in dark mode too, and the system's increased-contrast setting
  switches to stronger shades.
- Backgrounds, cards, menus, borders, and secondary text follow each style's
  design source as well: Anthropic's brand neutrals, the Apps SDK grays
  (OpenAI's dark page is now `#212121`, its light page white), Apple's system
  grays, and Material 3 surface roles. The browser bar matches the new page
  colours.
- On tablets and desktops, the Appearance panel closes when a style moves the
  appearance button between the header and the sidebar, instead of reopening
  from its new place.

</details>

<details>
<summary><strong>Library Browsing And Search</strong></summary>

- An album's song list shows each file's own quality (such as `FLAC 24/96` or
  `MP3 320 kbps`) and a lyrics column marking songs with lyrics. On tablets and
  desktops, once you have played songs from the album, a bar shows how often
  you played each one compared with your most played song there.

</details>

<details>
<summary><strong>Library Management</strong></summary>

- Online metadata lookups in the tag editor follow redirects only over HTTPS
  on the provider's own host, at most three, so a provider redirect to plain
  HTTP is refused instead of followed.

</details>

<details>
<summary><strong>Development And Docs</strong></summary>

- Every outbound HTTP client comes from one constructor in
  `internal/outbound` with an explicit redirect policy and response-size
  limit; a test rejects clients built any other way.
- Lint enforces the backend package layering (`depguard`), explained
  suppressions (`nolintlint`), and `util.NowMs()` for timestamps.
- Web features import each other only through the documented contracts in
  contract §9.2; shared dialogs, form fields, and admin queries live in
  `src/components`, `src/hooks`, and `src/lib`. `eslint-disable` comments
  need a reason, and files warn at 600 lines and fail at 800.
- Claude Code may not edit the lint configurations, CI workflows, or the
  outbound guard test (`.claude/settings.json`).

</details>
