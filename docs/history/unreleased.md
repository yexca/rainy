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
- Errors for single files in uploads, downloads, and **Review** no longer show
  server folder paths or raw system errors: they say what failed in plain
  words (for example that the disk is full), and the server log keeps the
  details.
- **Tracks**, **Library tools**, **Admin**, **Settings**, and **About** share one
  centered column across every tab so page titles, section tabs, tables, and
  forms keep matching left and right edges on wide screens.
- **Library tools → Folders** uses a split directory tree and breadcrumb browser
  that groups subfolders, playable audio tracks with multi-select tag actions,
  artwork thumbnails, and `.lrc` / text files with built-in image and text
  previews.
- Management pages are quieter: page subtitles show only counts and status,
  and explanatory captions are gone from the track table, upload area, and
  tag editor. In the tag editor, artists, album artists, and composers are
  chips like genres (type `;` or press Enter to add one, click one to edit
  it in place), the cover preview is clickable to choose a new image, and embedded
  picture details move to the **File** tab.
- **Library tools → Doctor** shows only the checks with problems as cards and
  opens the first one; passing checks shrink to green chips. **Folders**
  drops repeated headings: content group titles appear only when a folder
  holds more than one kind of file, and rescanning lives in the **Actions**
  menu.
- **Tracks** has three tabs: **Metadata**, **Add**, and **Review**. **Add**
  switches between **Upload**, **Links**, and **Online** beside one row of
  destination chips (library, folder, organizing by tags, and **Review
  first**), whose details open from each chip. **Review** shows how many
  downloads are waiting and appears only while some are. Existing
  `/manage/upload`, `/manage/links`, and `/manage/online` links keep working.
- **Admin** has three tabs: **Libraries**, **Settings**, and **Users**.
  **Settings** switches between **General**, **yt-dlp**, **Sources**,
  **Scrobbling**, and **System** at the top of the page; existing links keep
  working, and **Admin** opens on **Libraries**. Settings
  drop captions that repeated their labels. **Libraries** shows scan buttons
  on each library only when there is more than one, and after a scan lists
  only what changed. **Scrobbling** shows your own accounts once, above the
  services.
- On Tracks, Library tools, and Admin pages, counts such as "27 tracks" sit
  to the right of the page title instead of below it.

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
- Agent instructions are split into a short root `AGENTS.md`, area guides in
  `internal/`, `web/`, and `docs/`, and `migration` and `release` skills in
  `.claude/skills/`. Changes to them count as documentation in the PR plan.
- `make docs-check` also checks that every link's heading anchor exists, and
  covers the area guides and skills.

</details>
