Changes through v0.1.0 are summarized in [v0.1.0](v0.1.0.md).

Add user-facing changes for the next release here, grouped by area. Start with
an `[!IMPORTANT]` upgrade note when a change adds a migration or needs operator
action.

> [!IMPORTANT]
> This release adds database migrations `0003_lx_sources` (the table for music
> source scripts) and `0004_listening` (a copy of each play's song details in
> the play history, filled in for existing plays, and the tables for
> scrobbling). Back up `/data` before upgrading.

## Listening and Scrobbling

- New **Listening** page (Library → Listening on phones): a personal listening
  report for the last 7, 30, or 90 days, the last 12 months, all time, or a
  calendar year, with plays and listening time compared with the period
  before, new artists and songs, days with music and the longest streak, plays
  over time, a weekday-by-hour chart of when you listen, top artists, albums,
  songs, and genres, and the players you used. A **History** tab lists every
  play by day. Songs removed from the library stay in the history under their
  old names.
- **Scrobbling** (off by default): after an administrator turns it on in
  Admin → Settings → Scrobbling (Last.fm also needs an API account there),
  users connect Last.fm or ListenBrainz in Settings → Scrobbling. Plays from
  the web app and Subsonic apps, "now playing", and (for Last.fm) starred songs
  as loves are sent to the service; plays wait on the server and are retried
  while the service is unreachable.

## Playback and the Web Player

- Bilingual lyrics show the translation in smaller text under each original
  line, both for lines that share a timestamp and for `original 中文` lines. A
  translation button beside the lyrics button (and a setting under Playback)
  hides or shows translations.
- Japanese and Korean lyric lines use matching Japanese or Korean fonts.
- On tablets and desktops a player layout menu switches between the bottom
  bar, a floating Now Playing window, and a floating mini bar in the
  bottom-right corner; full screen stays available from all of them. The
  bottom bar can auto-hide, leaving a small handle that shows it on hover and
  pins it on click. Dragging sideways on the mini bar seeks.

## Library Management

- **Tracks** is a standalone entry in the sidebar (the **Manage** tab on
  phones) with three tabs: Metadata, Upload, and the new Online. Upload left
  the folded library tools; Upload and Online share the destination folder.
  The other library tools and the admin pages are folded away until opened.
- **Online music** (off by default): search Kuwo, Kugou, QQ Music, NetEase
  Cloud Music, and Migu in Tracks → Online and download songs into the library
  in the quality you choose, with the catalogue's tags, lyrics (translation
  paired), and cover written through TagLib. Links come from lx-music custom
  source scripts that an administrator imports; Rainy includes none. Several
  sources can be tried in priority order automatically, or one fixed source
  used; like lx-music, an expired link is requested from the same source once
  more before moving on. Downloads appear in the edit history with the song's
  page, quality, and source.
- The lyrics editor recognizes bilingual lyrics written as `original 中文`,
  previews the split, and on request rewrites timed lines as standard
  same-timestamp bilingual LRC. Stamping a line keeps its paired translation on
  the same time.
- The tag editor can search NetEase Cloud Music, QQ Music, Kugou, Kuwo, and
  iTunes for tags, covers, and lyrics (with optional paired translations). A
  chosen result only fills the editor; nothing is written until you save. With
  several tracks selected, only album-level fields are offered.
- Tracks → Upload can download the audio of YouTube and bilibili videos with
  yt-dlp: paste a link (bilibili share text works too), pick the original audio
  or M4A, MP3, or Opus, and optionally the whole playlist (up to 100 entries).
  Files land in the chosen folder or follow "Organize by tags", get title,
  artist, date, the source link, and a square cover written through TagLib,
  and appear in the edit history. Downloads keep running when you leave the
  page.

## Operations and Deployment

- New **Sources** tab in Admin → Settings with **Allow online music and music
  sources** (off by default): import lx-music source scripts from a file or a
  link, order them, enable or disable each, test them, update them from their
  link, and see their update notices; choose automatic fallback or one fixed
  source. Scripts run on the server in a sandbox that allows public internet
  addresses only; see
  [Privacy](../../PRIVACY.md#online-music-and-music-sources).
- New server setting **Online metadata lookup** (off by default). While it is
  on, the server contacts the catalogues above when a manager searches; see
  [Privacy](../../PRIVACY.md#online-metadata-lookup). A second setting,
  **Pretend to search from mainland China** (also off), adds a spoofed
  mainland-China `X-Real-IP` header to the Chinese catalogues' requests.
- New **yt-dlp** tab in Admin → Settings with **Allow downloads from YouTube
  and bilibili** (off by default), yt-dlp install / update checks (the
  official GitHub release, verified against its checksums, installed into
  `/data/ytdlp`), and optional sign-in cookies. Cookies are account
  credentials: Rainy shows a warning first, keeps only the site's own cookies,
  stores them encrypted with `secret.key`, and never returns them; see
  [Privacy](../../PRIVACY.md#downloads-from-youtube-and-bilibili).
  `RAINY_YTDLP_PATH` points Rainy at a yt-dlp you manage yourself instead.
- The image now includes QuickJS (about 2 MiB), the JavaScript runtime yt-dlp
  needs for YouTube; yt-dlp itself is not bundled.

## Development and Docs

- `make frontend-test` runs unit tests for pure web helpers with Node's
  built-in test runner; `ci-frontend` includes it.
- Agents build and run the app through Docker by default (`make docker-build
  smoke`, `make docker-up`); see `AGENTS.md`.
