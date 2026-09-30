# Management

Managers and administrators can change the music library from the **Manage**
section. Tag editing is used most, so **Metadata** is always visible: in the
sidebar on desktop and as its own tab on phones. The other tools (Folders,
Upload, Doctor, Trash, History) and the admin pages stay folded under
**Library tools** and **Admin** until you open them. Every change writes to your
files, so:

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
through TagLib. Use it when ordinary tag editing fails on a WAV with legacy
INFO metadata; those old metadata bytes stay in the file as an inert chunk.
If Rainy cannot read an old field, it uses the track's indexed title, artist,
or album where available. The action also works on files whose tags are
already readable. Select the tracks in **Metadata** and choose
**Rebuild tags**; each file is checked before replacing the original.

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

**Manage → Upload** accepts files and whole folders by drag and drop, with
per-file progress. Choose the target library and folder, and optionally
**Organize by tags** to place each file with the rename pattern. Without
organizing, uploaded folder structure is kept. Uploaded files appear in the
library immediately.

Uploads through a reverse proxy are limited by the proxy's body size; see
[Reverse proxy](../operations/reverse-proxy.md).

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
covers, lyrics, renames, uploads, deletions, restores, purges, and encoding
fixes, with who made them, when, and a readable before-and-after view.

## Related Docs

- [Library](library.md)
- [Library management architecture](../architecture/library-management.md)
- [Deployment security](../operations/security.md)
