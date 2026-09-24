# Management

Managers and administrators can change the music library from the **Manage**
section. Every change writes to your files, so:

- The music folder must be writable by the container user; see
  [Docker](../operations/docker.md#permissions-puid-and-pgid). On a read-only
  mount, these tools explain that files are read-only.
- Keep a backup of your music. Rainy's trash and edit history help undo
  mistakes, but they do not replace a backup.

## Library Manager

**Manage → Library** is a table of every track with sortable, configurable
columns (title, artist, album, album artist, track, disc, year, genre, format,
bitrate, and path), search, and filters by album, artist, genre, folder, or
missing files. Select tracks with click, Shift-click for a range, Ctrl/Cmd-click
to toggle, or select everything that matches. On phones, the list switches to a
selection mode.

The toolbar applies actions to the selection: **Edit tags**, **Cover**,
**Rename / organize**, **Fix encoding**, **Delete**, and **Rescan**.

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

Saving writes only the fields that changed for each track and reports any
per-track errors.

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

**Manage → History** records every change Rainy made to your files: tag edits,
covers, lyrics, renames, uploads, deletions, restores, purges, and encoding
fixes, with who made them, when, and a readable before-and-after view.

## Related Docs

- [Library](library.md)
- [Library management architecture](../architecture/library-management.md)
- [Deployment security](../operations/security.md)
