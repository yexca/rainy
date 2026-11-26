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
