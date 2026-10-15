# Library

Rainy organizes music by **tags**, not by folders. Clean tags give the best
browsing experience; the folder layout mainly helps Rainy find covers, artist
images, and lyrics files.

## Recommended Layout

Use one folder per album artist and one folder per album:

```text
/data
├── Example Artist/
│   ├── artist.jpg                  artist image, one level above album folders
│   └── 2003 - Example Album/
│       ├── cover.jpg               album cover
│       ├── 01 First Song.flac
│       ├── 01 First Song.lrc       synced lyrics with the same name
│       └── 02 Second Song.flac
└── Various Artists/
    └── Example Compilation/
        ├── CD1/ ...
        └── CD2/ ...
```

The default rename pattern, `{albumartist}/{album}/[{disc}-]{track:2} {title}`,
produces this layout automatically; see
[Management](management.md#rename-and-organize).

## Tags That Matter

- **ALBUMARTIST** keeps an album together. Without it, the track artist is
  used, so an album with guest artists splits into several albums. For
  compilations, set `COMPILATION=1` (iTunes' compilation flag counts too); a
  compilation without ALBUMARTIST is filed under `Various Artists`.
- **DISCNUMBER** (for example `1/2`) and **TRACKNUMBER** (for example `3/12`)
  order multi-disc albums.
- **DATE** or **YEAR** provide the year; **ORIGINALDATE** is also read.
- **GENRE** can hold several genres, such as `Pop; Rock`. The separators
  (default `;`, `/`, and `,`) are configurable in **Admin → Settings**.
- **Sort tags** (`ARTISTSORT`, `ALBUMSORT`, `ALBUMARTISTSORT`, `TITLESORT`)
  and MusicBrainz IDs are used when present.
- **ReplayGain** tags enable volume normalization in the player.

When a tag is missing, Rainy falls back to the file name for the title,
`[Unknown Album]`, and `[Unknown Artist]`.

Artist names are indexed by initial letter. Chinese names use the pinyin
initial (for example, a name starting with 周 is listed under Z), and leading
articles such as "The" and "La" are ignored when sorting; the list is
**Ignored articles** in **Admin → Settings**.

## Covers and Lyrics

- **Album covers** come from the folder image first (`cover.*`, `folder.*`,
  `front.*`, `album.*`, `albumart*.*` in JPEG, PNG, or WebP), then from the
  embedded picture. For multi-disc albums, a cover one level above disc
  folders such as `CD1`, `Disc 2`, or `Vinyl 1` is found too. The file-name
  patterns are **Cover image files** in **Admin → Settings**.
- **Artist images** are `artist.*` files in the folder above the album folder,
  else in the album folder itself; otherwise the cover of the artist's most
  recent album is used.
- **Lyrics** come from a `.lrc` file with the same name as the audio file
  (UTF-8, UTF-16, GBK, Big5, or Shift-JIS), then from the embedded `LYRICS` or
  `UNSYNCEDLYRICS` tag.

## Supported Formats

MP3, FLAC, AAC/M4A/M4B/MP4/ALAC, Ogg Vorbis, Opus, WAV, AIFF, APE, WavPack,
Musepack, TTA, WMA, Speex, and DSD (DSF/DFF). Other files are not indexed.
Formats that browsers cannot play are transcoded automatically when ffmpeg is
available.

## What Gets Scanned

Rainy scans every library on startup (unless `RAINY_SCAN_ON_START=false`), on
the **Automatic scan** interval in **Admin → Settings** (every hour by
default), and when you start a scan manually.

- A **quick scan** reads only new files and files whose size or modification
  time changed.
- A **full scan** re-reads every file. Use it after changing settings that
  affect how tags are read, such as genre separators or encoding repair.

Start either one under **Admin → Libraries**, or rescan a single folder from
**Library tools → Folders**.

Rainy skips:

- Hidden files and folders (names starting with `.`, such as `.@__thumb`).
- NAS and OS system folders: `@eaDir`, `#recycle`, `#snapshot`, `@Recycle`,
  `$RECYCLE.BIN`, `lost+found`, and `System Volume Information`.
- Any folder that contains a file named `.rainyignore`, including all of its
  subfolders. Create an empty file with that name to exclude a folder.

### Moved and Removed Files

When you move or rename files outside Rainy, the next scan recognizes a moved
file by its size, duration, and title, and keeps its favorites, ratings, play
counts, and playlist entries. Files that disappear are marked missing and
hidden; they return automatically if the files come back. Remove them under
**Missing files** in **Library tools → Doctor** once they are gone for good.

If a whole library suddenly looks empty or unreadable (for example, an
unmounted disk or share), Rainy skips that library and logs a warning instead
of marking everything missing. The same applies inside a library to a folder
or link that cannot be read and to a folder that held 20 or more songs and is
now completely empty: its songs stay as they are until it comes back. To remove
such songs for good, delete the folder itself.

## Fixing Garbled Tags

Many older Chinese and Japanese MP3 files store GBK (Simplified Chinese), Big5
(Traditional Chinese), or Shift-JIS (Japanese) text in ID3 tags that claim to be
Latin-1. Most players then show garbage such as `ÖÜ½ÜÂ×`.

Rainy handles this in two steps:

1. **Repair when reading (on by default).** The scanner detects this mojibake
   and stores the corrected text in the database. Your files are not modified.
   The switch is **Repair garbled tags when reading** in **Admin → Settings**.
2. **Fix the files permanently.** Open the **Encoding** check in
   **Library tools → Doctor** (or select tracks in **Tracks → Metadata** and
   choose **Fix encoding**), review the old and new value of every field, and
   confirm. Rainy rewrites the tags as UTF-8, so every other player and
   Subsonic client shows them correctly too. This requires a writable music
   folder.

Detection is conservative: text changes only when the decoded result is
plausible Chinese or Japanese, so normal Western tags such as `Exémplé` are left
alone.

## Playlist Files

Choose **Import playlist** on the Playlists page and select a UTF-8
`.m3u` or `.m3u8` file. Preview the matches, name the playlist, and choose
**Create playlist**. It is private and contains only the matched songs in
their original order, including repeated songs. No music files are changed.

Paths must be relative to a music library. If the playlist lives inside a
subfolder and its entries are relative to that folder, enter the folder in
**Playlist folder**. Paths from the library root use an empty folder.
Windows path separators are accepted; absolute paths and URLs are not.
Matching uses exact paths, including letter case.

The preview identifies missing files, unsupported paths, and paths that match
more than one library. Choose a library to resolve ambiguous paths and
preview again. Unmatched entries are skipped only after you acknowledge
them. Files are limited to 512 KiB and 10,000 entries.

The playlist's **… → Export M3U8** action downloads a list of its playable
songs, including their library-relative paths. It contains no audio or
credentials. Keep it relative to the library root when using another player.
Rainy adds an optional track-id comment so imports into the same instance can
distinguish identical paths in different libraries; imports elsewhere fall
back to paths. Other players ignore this comment.

## Related Docs

- [Management](management.md)
- [Configuration: server settings](../operations/configuration.md#server-settings)
- [Troubleshooting](../operations/troubleshooting.md)
