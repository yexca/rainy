# Management

Managers and administrators can change the music library from the **Manage**
section, which has three entries in the sidebar: **Tracks**, **Library
tools**, and (for administrators) **Admin**. Each entry's pages are tabs at
the top of the page. **Tracks** has **Metadata** (the track table and tag
editor), **Upload** (files from this device), **Links** (YouTube and
bilibili), and **Online** (online music); **Library tools** has Folders,
Doctor, Trash, and History; **Admin** has Users, Libraries, and the server
settings: Settings, yt-dlp, Sources, Scrobbling, and System. On phones the **Manage** tab opens
Tracks, with links to Library tools and Admin beside its tabs. Every change
writes to your files, so:

- The music folder must be writable by the container user; see
  [Docker](../operations/docker.md#permissions-puid-and-pgid). On a read-only
  mount, these tools explain that files are read-only.
- Keep a backup of your music. Rainy's trash and edit history help undo
  mistakes, but they do not replace a backup.

## Metadata

**Metadata** is a table of every track with sortable, configurable
columns (title, artist, album, album artist, track, disc, year, genre, format,
bitrate, and path), search, and filters by album, artist, genre, folder, or
missing files. Select tracks with click, Shift-click for a range, Ctrl/Cmd-click
to toggle, or select everything that matches. On phones, the list switches to a
selection mode.

The toolbar applies actions to the selection: **Edit tags**, **Cover**,
**Rename / organize**, **Fix encoding**, **Rebuild tags**, **Delete**, and **Rescan**.

## Tag Editor

The tag editor opens as a side panel on desktop and full screen on phones. It
is also available from the "…" menu of any track.

- **Details** edits common fields. With several tracks selected, fields that
  differ show "Multiple values" and are written only if you change them. Each
  field can be reverted before saving.
- **Cover** previews the picture, accepts drag and drop, paste, or upload, and
  can remove it, apply it to the whole album, or save it as the folder image.
- **Lyrics** edits embedded lyrics or the `.lrc` file (see below).
- **All tags** shows every raw tag, including custom ones, and lets you add or
  remove them.
- **File** shows read-only file information.

The **Tools** menu previews each change before applying it: auto-number tracks
in the current order, tags from the file name, find and replace (with optional
regular expressions), change case, copy one field to another, and clear a
field.

**Search online** looks the selection up in NetEase Cloud Music, QQ Music,
Kugou, Kuwo, or iTunes. An administrator must first turn on **Online metadata
lookup** in **Admin → Settings**; until then the dialog explains how. The
search starts with the track's title and artist (or the album and album
artist when several tracks are selected), and a length badge shows how close
each result is to your file. Opening a result lists each field as current →
new value with a checkbox; fields that would change are selected. With several
tracks selected, only album-level fields (album, album artist, totals, date,
genre) are offered. You can also take the result's cover and, for a single
track, its lyrics, optionally with the translation paired line by line.
**Fill in editor** only changes the editor's draft: review it and save as
usual. The search terms are sent to the chosen service from the server; see
[Privacy](../../PRIVACY.md#online-metadata-lookup). If the Chinese catalogues
return few results because the server is outside mainland China, an
administrator can also turn on **Pretend to search from mainland China**.

Saving writes only the fields that changed for each track and reports any
per-track errors.

**Rebuild tags** reads the file's existing tags first and writes them again
through TagLib. WAV INFO stays active alongside ID3; rewritten INFO text may
be normalized to UTF-8. Rebuilding does not archive old metadata in padding
chunks. If Rainy cannot read an old field, it uses the track's readable indexed
title, artist, or album where available. A field containing replacement
characters with no usable indexed value causes the file to fail without
replacing the original. Select the tracks in **Metadata** and choose
**Rebuild tags**; each copy is verified before replacing the original.
For a WAV whose tags are displayed incorrectly by an older reader, first
update Rainy and run a full scan to refresh the index without changing files.

## Covers

From the **Cover** action or the editor's Cover tab, you can embed a picture in
the audio files, save it as `cover.jpg` or `cover.png` in the album folder, or
both. WebP, GIF, and BMP images are converted to JPEG for embedding. A replaced
folder image moves to the trash, so it can be restored.

## Lyrics

Edit lyrics as plain text or LRC with `[mm:ss.xx]` timestamps, one track at a
time. While the track plays, **Stamp line** puts the current playback time at
the start of the current line and moves to the next one, which makes syncing
lyrics by ear easy. Choose whether to save into the embedded `LYRICS` tag or a
`.lrc` file beside the audio file (best for synced lyrics). Saving empty text
removes the lyrics.

### Bilingual Lyrics

Many downloaded lyrics put the original line and its Chinese translation on one
line, separated by a space (`君の名前を呼んだ 我呼唤了你的名字`). When you paste
or type lyrics like this, the Lyrics tab recognizes them and offers to split
them. **Preview** shows how each line would split, and **Split lines** rewrites
every timed line as two lines with the same timestamp, original first:

```text
[00:12.00]君の名前を呼んだ
[00:12.00]我呼唤了你的名字
```

This is the standard bilingual LRC layout, so Rainy and most Subsonic apps show
it as an original line with its translation. Nothing changes until you choose
**Split lines** and save; untimed lines are left as they are. Detection only
splits a line when a Japanese, Korean, or other non-Chinese part is followed by
a purely Chinese part, and only when most lines of the text look that way.

## Rename and Organize

**Rename / organize** moves and renames files by a pattern such as the default
`{albumartist}/{album}/[{disc}-]{track:2} {title}`.

| Token | Meaning |
| --- | --- |
| `{title}`, `{artist}`, `{album}`, `{albumartist}` | Tag values |
| `{track}`, `{disc}` | Track and disc numbers; `{disc}` is empty for single-disc albums |
| `{track:2}`, `{disc:2}` | Zero-padded to the given width |
| `{year}`, `{genre}`, `{composer}` | Tag values |
| `/` | Starts a new folder level |
| `[ ... ]` | Optional part, dropped when every token inside is empty |

The file extension is kept. Characters that are not allowed in file names are
replaced with `_`. The preview marks every file as ok, unchanged, conflict, or
invalid before anything moves. `.lrc` files move with their songs, and folders
left empty are removed (never the library root). The default pattern is set in
**Admin → Settings**.

## Upload

**Tracks → Upload** accepts files and whole folders by drag and drop, with
per-file progress. Choose the target library and folder, and optionally
**Organize by tags** to place each file with the rename pattern. Without
organizing, uploaded folder structure is kept. Uploaded files appear in the
library immediately.

Uploads through a reverse proxy are limited by the proxy's body size; see
[Reverse proxy](../operations/reverse-proxy.md).

### Download from a Link

The **Links** tab downloads the audio of a YouTube or bilibili video into the
same destination as Upload.
Paste a link (bilibili share text such as `【Title】 https://b23.tv/…` works
too), choose the audio format, and select **Download**. **Original (no
conversion)** keeps the site's audio stream (Opus from YouTube, AAC from
bilibili); M4A, MP3, and Opus convert it. **Whole playlist** downloads up to
100 entries of the playlist or multi-part video the link belongs to. Files go
to the destination above and follow **Organize by tags** like uploads;
otherwise they are named after the video title, and playlist entries go into a
folder named after the playlist. Rainy writes the title, artist (the channel
when nothing better is known), album, date, the source link as the comment, and
a square cover cut from the thumbnail; fix the rest in the tag editor. Downloads
run on the server, so you can leave the page; live streams are skipped. Only
download what you have the right to keep.

An administrator sets this up in **Admin → yt-dlp**:

1. Turn on **Allow downloads from YouTube and bilibili**.
2. Select **Install yt-dlp**. Rainy downloads the official release from GitHub
   and verifies it. YouTube changes often: use **Check for updates** now and
   then and update when a new version is available.
3. Optionally add **sign-in cookies** for members-only, age-restricted, or
   higher-quality downloads. Export them with the
   [Get cookies.txt LOCALLY](https://chromewebstore.google.com/detail/get-cookiestxt-locally/cclelndahbckbenkjhflpdbgdldlbecc)
   browser extension for the current site only, then paste the file or choose
   it. For YouTube, sign in from a private window and close it after
   exporting, because YouTube soon invalidates the cookies of a session that
   stays open.

> [!WARNING]
> Cookies are the keys to your account. Anyone, or any malicious program, that
> obtains them can act as you. Never share them. Rainy keeps only the cookies of
> the site itself, stores them encrypted on the server, never shows them again,
> and never uploads them anywhere; yt-dlp sends them only to that site during a
> download. Consider a separate account for downloads, and sign out in the
> browser to revoke a session.

## Online Music

**Tracks → Online** searches Kuwo, Kugou, QQ Music, NetEase Cloud Music, or
Migu and downloads songs into your library. Pick a catalogue, search, and
select songs (or use a row's download button). Under **Download options**
choose the destination (shared with the Upload and Links tabs), the best quality you
want (Hi-Res, FLAC, 320K, or 128K; a lower one is used when the song or the
sources lack it), and whether to embed the catalogue's lyrics (with a paired
translation) and cover. Rainy writes the title, artists, album, lyrics, and
cover, names files `Title - Artists.ext` unless you organize by tags, and
records each download in the edit history. Downloads run on the server, so you
can leave the page. Only download what you have the right to keep.

Rainy cannot obtain songs by itself: the download link comes from a **music
source**, an lx-music custom source script that an administrator imports.
Rainy doesn't include or recommend any source. An administrator sets this up
in **Admin → Sources**:

1. Turn on **Allow online music and music sources**.
2. Select **Import** and choose the script's `.js` file, or paste a link to it
   (Rainy remembers the link so **Update from link** can fetch new versions).
3. Use **Test / restart** from the source's menu to check that it starts; the
   list shows which catalogues and qualities it provides.
4. With several sources, order them with the arrows. **Automatic fallback**
   (the default) tries the enabled sources from top to bottom until one
   delivers the song; **Always use one source** asks only the chosen one.

A source may show an update notice from its author with a link; hide these per
source in its menu.

> [!WARNING]
> A music source is third-party code that runs on your server. It may contact
> any public internet address and receives the songs you download. Rainy
> limits its run time and network access (never your local network), but not
> its memory use. Import only scripts you trust.

## Trash

**Delete** moves files to the trash in the data folder. **Manage → Trash** can
restore them to their original place or delete them forever. Emptying the
trash is permanent.

## Doctor

**Manage → Doctor** finds common problems and links each one to the tool that
fixes it:

- **Missing tags**: tracks without a title, artist, or album.
- **No cover**: albums without artwork.
- **Duplicates**: likely duplicate tracks.
- **Missing files**: tracks whose files are gone; purge them once you are sure.
- **Encoding**: garbled GBK, Big5, or Shift-JIS tags that can be rewritten as
  UTF-8; see [Library](library.md#fixing-garbled-tags).

## Folders

**Manage → Folders** browses the files on disk, shows which files are indexed,
and can rescan a single folder.

## History

**Manage → History** records every change Rainy made to your files: tag edits and rebuilds,
covers, lyrics, renames, uploads, downloads from links, deletions, restores,
purges, and encoding fixes, with who made them, when, and a readable before-and-after view.

## Related Docs

- [Library](library.md)
- [Library management architecture](../architecture/library-management.md)
- [Deployment security](../operations/security.md)
