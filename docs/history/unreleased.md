# Unreleased

Changes through v0.1.3 are summarized in [v0.1.3](v0.1.3.md).

<details>
<summary><strong>Playback And The Web Player</strong></summary>

- In immersive mode, the next song's colours fade in across the whole window
  in a slower, softer cross-fade instead of spreading out from a tap, click,
  or the artwork. Taps and clicks still work while the colours fade.
- On tablets and desktops the bottom bar is now a pill floating over the
  content, laid out like the Apple Music web player. Click the artwork for
  the full-screen player or the title for the floating window; click the
  progress line to jump to that point, and hover it to preview the time. The
  bar still hides and pins as before. The floating mini bar is gone: the
  layout menu offers full screen, floating window, and bottom bar. The corner
  mascot sits on the bar near its right end.
- The full-screen player remembers whether you left the lyrics open. On
  phones it starts on the artwork until you open the lyrics; on tablets and
  desktops it starts on the lyrics until you close them.

</details>

<details>
<summary><strong>Library Browsing And Search</strong></summary>

- **Home** is calmer and more focused: **Made for You** shows your daily mix,
  weekly mix, and first-week monthly recap as horizontal poster cards; song
  recommendations share one **Picked for You** shelf with **Quick picks**, **On
  repeat**, and **Rediscover** pills beside the title; and **Recently added**
  shows only albums imported in the last 14 days, above **Discover** and below
  **Recently played**.
- **Search** (`Ctrl+K` / `⌘K`, the search pill on Home, or the search button in
  the bar) combines live song, album, artist, and playlist results with page
  navigation and playback controls, with a link to the full `/search` results
  page.
- **Playlists** moves into the phone tab bar (**Home · Playlists · Library**),
  and **Genres** is replaced by genre filters in the **Albums** and **Songs**
  toolbars (`/genres` links redirect to **Albums**).
- Search no longer runs on a Chinese or Japanese input method's reading
  (pinyin, kana) while you type. The reading stays in the field, and the
  search starts once you pick the text. Escape cancels the reading instead of
  clearing the search. The track search, album and artist pickers on the
  Tracks page, and the playlist filter behave the same way.

</details>

<details>
<summary><strong>Library Management</strong></summary>

- Link and online music downloads can wait in the new **Tracks → Review** tab
  instead of going straight into the library. Correct their tags, folder, and
  organizing there, see possible duplicates (an existing file, an earlier
  download from the same source, similar tracks), then import or discard them.
  **Review downloads before importing** in Admin → Settings sets the default;
  the Links and Online tabs can change it per session.
- At startup Rainy removes staged uploads and online download work
  directories that a crash left in `<data>/tmp`.
- **Tracks**, **Library tools**, **Admin**, **Settings**, and **About** share one
  centered column across every tab so page titles, section tabs, tables, and
  forms keep matching left and right edges on wide screens.
- **Library tools → Folders** uses a split directory tree and breadcrumb browser
  that groups subfolders, playable audio tracks with multi-select tag actions,
  artwork thumbnails, and `.lrc` / text files with built-in image and text
  previews.

</details>

<details>
<summary><strong>Development And Docs</strong></summary>

- Validation runs in two tiers: **PR** checks the jobs a pull request's
  changes need, and **Full** checks every job on each `main` commit. The
  required check is **PR / Gate**, and a release waits for a successful Full
  run of the tagged commit.
- `make ci-policy` lints the workflows with actionlint. Go and web advisory
  checks move from the CI phases to a daily Dependency audit workflow and
  `make audit`, so a new advisory no longer fails an unrelated pull request.
  A pull request that changes a Go or web dependency manifest still runs the
  matching audit and needs it to pass.

</details>
