# Unreleased

Changes through v0.1.2 are summarized in [v0.1.2](v0.1.2.md).

<details>
<summary><strong>Playback And The Web Player</strong></summary>

- Queue restoration keeps the selected occurrence of repeated songs and
  adjusts its position when unavailable files are omitted.
- Now Playing opens on the lyrics. Closing it gives the side panel or floating
  window back the view it had.
- In the full-screen player on tablets and desktops, the queue opens in a
  floating card under the artwork and controls, which slide up to make room,
  and stays open alongside the lyrics.
- Bilingual lyrics also recognize translations in brackets at the end of a
  line (`待ってよ「等一下」`), and credit lines such as `词：…` no longer keep a
  text from counting as bilingual.
- Long songs (10 minutes or longer by default) are left out when a list is
  played or queued and from infinite mode's suggestions; the song you pick
  still plays. Turn it off or change the length in Settings → Playback.
- In immersive mode the next song's colours spread out in a circle from where
  you clicked or tapped, or from the song's artwork in the player when a song
  ends on its own, instead of switching at once.

</details>

<details>
<summary><strong>Library Browsing And Playlists</strong></summary>

- Import UTF-8 M3U/M3U8 playlists with a match preview, library and folder
  selection, and explicit acknowledgement of unmatched entries. Export
  playable songs as M3U8 while preserving order and duplicates.

</details>

<details>
<summary><strong>Library Management</strong></summary>

- In immersive mode the sticky selection bar on Tracks, the trash bar and the
  history day headers are glass like the app header, so the cover colours show
  through them.

</details>

<details>
<summary><strong>Operations And Deployment</strong></summary>

- Create database-and-key snapshots with `rainy backup create <name>` and
  check their hashes, integrity, migration compatibility and credentials
  with `rainy backup verify <name>` before restoring them.

</details>
