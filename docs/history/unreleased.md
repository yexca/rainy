# Unreleased

Changes through v0.2.0 are summarized in [v0.2.0](v0.2.0.md).

> [!IMPORTANT]
> This release adds two database migrations (`0008_user_folders`,
> `0009_program_folders`). Back up the application-state directory (`/config`)
> before upgrading. Existing library access is unchanged, and no folder is a
> program folder until a manager marks one.

> [!NOTE]
> **Apple is the default style.** Browsers that never picked a style switch to
> Apple, with **Immersive** on, after the upgrade. Pick another style or turn
> Immersive off in **Settings → Appearance**; the choice is kept per browser.

<details>
<summary><strong>Appearance</strong></summary>

- New browsers start in the Apple style with Immersive on.
- Every page shows its subtitle, such as the album or song count, beside the
  title as the management pages do, moving below it only when there is no
  room. When you scroll, the subtitle follows the title into the bar at the
  top. Listening and Settings no longer have a subtitle.
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
- On tablets and desktops, the Apple style no longer has a Search button in
  the bottom-left corner of the sidebar; search from the field on the home
  page or with Ctrl+K / ⌘K.
- On tablets and desktops, the Appearance panel closes when a style moves the
  appearance button between the header and the sidebar, instead of reopening
  from its new place.
- Every mascot illustration is redrawn: the status scenes, the corner
  companion, and the pictures for missing covers, artist photos, playlists,
  and radio stations. Her hands are drawn correctly, and the art is sharper
  and more consistent.
- The title bar that appears when a page scrolls no longer sits on a strip
  with a line below it: the page fades and blurs softly under the title, and
  in immersive mode the cover's colours show through it. The Library
  statistics in
  Admin → Settings → System, the sort and filter buttons on Albums, Artists,
  and Songs, the Picked for You tabs on Home, the style choices in
  Appearance, and the tag editor's Raw rows let the cover's colours show
  through instead of sitting on dark blocks.

</details>

<details>
<summary><strong>Library Browsing And Search</strong></summary>

- An album's song list shows each file's own quality (such as `FLAC 24/96` or
  `MP3 320 kbps`) and a lyrics column marking songs with lyrics. On tablets and
  desktops, once you have played songs from the album, a bar shows how often
  you played each one compared with your most played song there.
- On desktops, an artist's top songs fill two columns, three on wide windows,
  like the home song shelves, with all ten shown at once and each row giving
  the album and year, so wide windows no longer stretch five rows across the
  page. Phones and tablets keep the single list with **Show all**.
- Songs in the home song shelves, such as **Quick picks**, show their
  duration on tablets and desktops.
- The artist name under an album's cover, on the home page and in album
  lists and search, opens the artist; the cover and title still open the
  album.
- A page's top bar, with its back button and the small title that replaces
  the large one, stays at the top while you scroll the whole page instead of
  scrolling away once you pass the page title.

</details>

<details>
<summary><strong>Playback And The Web Player</strong></summary>

- The bottom bar leaves more room for the song title. The elapsed and
  remaining times and the star appear over the end of the title while you
  hover the song (a favorite keeps its star in view), and the volume slider
  slides out when you hover the speaker. Player layout, picture-in-picture
  lyrics, and the auto-stop timer moved into a new **Player options** menu;
  while picture-in-picture lyrics or a timer is on, its highlighted button
  stays next to the menu.
- Recorded shows can live in your library as programs: a manager marks their
  folder with **Actions → Mark as program folder** in **Library tools →
  Folders**, and everything in it and below it becomes a program episode. An
  administrator can instead turn on **Program library** for a whole library in
  **Admin → Libraries**.
  Episodes stay out of the home page, random picks, mixes, infinite mode, the
  album and song lists, Subsonic random and album lists, and Last.fm and
  ListenBrainz scrobbles. **Radio → Programs** lists the shows with your
  progress, and an artist's page lists that artist's shows under
  **Programs**; a show's page marks each episode played or partly played and
  **Continue** picks up the first unfinished one. Episodes resume where you
  stopped, a position Subsonic apps share through bookmarks.

</details>

<details>
<summary><strong>Library Management</strong></summary>

- The bar with the Tracks actions and column headings, the Trash and Review
  bars, and the History day headings scroll with the page and sit on the page
  itself, with a thin line below them, instead of pinning to the top on a
  frosted strip.
- Online metadata lookups in the tag editor follow redirects only over HTTPS
  on the provider's own host, at most three, so a provider redirect to plain
  HTTP is refused instead of followed.
- A listener can be limited to folders of a library instead of whole
  libraries, so one library can hold all your music while some people see
  only certain artists. In the user dialog, open **Folders** next to a library
  and check folders; a checked folder includes everything below it. Subsonic
  apps show that library as the listener's music folder.
- Scans and uploads skip `.mp4` files, so videos in a library folder no
  longer show up as songs. Songs already indexed from `.mp4` files are marked
  missing on the next scan; rename an audio-only `.mp4` to `.m4a` to keep it,
  with its play counts and favorites.
- The **Add library** dialog, its path error, and the read-only message give
  the image's container paths: the default library is `/data`, and another
  library needs its own mount such as `/music2`.

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
