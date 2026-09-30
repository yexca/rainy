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
- On tablets and desktops a player layout menu switches between the bottom
  bar, a floating Now Playing window, and a floating mini bar in the
  bottom-right corner; full screen stays available from all of them. The
  bottom bar can auto-hide, leaving a small handle that shows it on hover and
  pins it on click. Dragging sideways on the mini bar seeks.

## Library Management

- Metadata editing is now a standalone entry in the sidebar and its own tab on
  phones. The other library tools and the admin pages are folded away until
  opened.
- The lyrics editor recognizes bilingual lyrics written as `original 中文`,
  previews the split, and on request rewrites timed lines as standard
  same-timestamp bilingual LRC. Stamping a line keeps its paired translation on
  the same time.
- The tag editor can search NetEase Cloud Music, QQ Music, Kugou, Kuwo, and
  iTunes for tags, covers, and lyrics (with optional paired translations). A
  chosen result only fills the editor; nothing is written until you save. With
  several tracks selected, only album-level fields are offered.
- The Upload page can download the audio of YouTube and bilibili videos with
  yt-dlp: paste a link (bilibili share text works too), pick the original audio
  or M4A, MP3, or Opus, and optionally the whole playlist (up to 100 entries).
  Files land in the chosen folder or follow "Organize by tags", get title,
  artist, date, the source link, and a square cover written through TagLib,
  and appear in the edit history. Downloads keep running when you leave the
  page.

## Operations and Deployment

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
