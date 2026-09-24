# Library

Rainy organizes music by **tags**, not by folders. Clean tags give the best
browsing experience; the folder layout mainly helps Rainy find covers, artist
images, and lyrics files.

## Recommended Layout

Use one folder per album artist and one folder per album:

```text
/music
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

- **ALBUMARTIST** keeps an album together. Without it, an album with guest
  artists splits into several albums. For compilations, use `Various Artists`
  and set `COMPILATION=1`.
- **DISCNUMBER** (for example `1/2`) and **TRACKNUMBER** (for example `3/12`)
  order multi-disc albums.
- **DATE** or **YEAR** provide the year; **ORIGINALDATE** is also read.
- **GENRE** can hold several genres, such as `Pop; Rock`. The separators
  (default `;`, `/`, and `,`) are configurable in **Admin → Settings**.
- **Sort tags** (`SORTARTIST`, `SORTALBUM`, and so on) and MusicBrainz IDs are
  used when present.
- **ReplayGain** tags enable volume normalization in the player.

When a tag is missing, Rainy falls back to the file name for the title,
`[Unknown Album]`, and `[Unknown Artist]`.

Artist names are indexed by initial letter. Chinese names use the pinyin
initial (for example, a name starting with 周 is listed under Z), and leading
articles such as "The" and "La" are ignored when sorting.

## Covers and Lyrics

- **Album covers** come from the folder image first (`cover.*`, `folder.*`,
  `front.*`, `album.*`, `albumart*.*` in JPEG, PNG, or WebP), then from the
  embedded picture. For multi-disc albums, a cover one level above `CD1` or
  `Disc 2` folders is found too. The file-name patterns are configurable.
- **Artist images** are `artist.*` files in the folder above the album folder;
  otherwise the latest album cover is used.
- **Lyrics** come from a `.lrc` file with the same name as the audio file, then
  from the embedded `LYRICS` or `UNSYNCEDLYRICS` tag.

## Supported Formats

MP3, FLAC, AAC/M4A/M4B/ALAC, Ogg Vorbis, Opus, WAV, AIFF, APE, WavPack,
Musepack, TTA, WMA, Speex, and DSD (DSF/DFF). Formats that browsers cannot play
are transcoded automatically when ffmpeg is available.

## What Gets Scanned

Rainy scans every library on startup (unless `RAINY_SCAN_ON_START=false`), on
the scan interval (hourly by default), and when you start a scan manually.

- A **quick scan** reads only new files and files whose size or modification
  time changed.
- A **full scan** re-reads every file. Use it after changing settings that
  affect how tags are read, such as genre separators or encoding repair.

Start either one under **Admin → Libraries**, or rescan a single folder from
**Manage → Folders**.

Rainy skips:

- Hidden files and folders (names starting with `.`).
- NAS and OS system folders: `@eaDir`, `#recycle`, `#snapshot`, `.@__thumb`,
  `$RECYCLE.BIN`, `lost+found`, and `System Volume Information`.
- Any folder that contains a file named `.rainyignore`, including all of its
  subfolders. Create an empty file with that name to exclude a folder.

### Moved and Removed Files

When you move or rename files outside Rainy, the next scan recognizes a moved
file by its size, duration, and title, and keeps its favorites, ratings, play
counts, and playlist entries. Files that disappear are marked missing and
hidden; they return automatically if the files come back. Purge them from
**Manage → Doctor** once they are gone for good.

If a whole library suddenly looks empty or unreadable (for example, an
unmounted disk or share), Rainy skips that library and logs a warning instead
of marking everything missing.

## Fixing Garbled Tags

Many older Chinese and Japanese MP3 files store GBK (Simplified Chinese), Big5
(Traditional Chinese), or Shift-JIS (Japanese) text in ID3 tags that claim to be
Latin-1. Most players then show garbage such as `ÖÜ½ÜÂ×`.

Rainy handles this in two steps:

1. **Repair when reading (on by default).** The scanner detects this mojibake
   and stores the corrected text in the database. Your files are not modified.
   The switch is **Repair garbled tags when reading** in **Admin → Settings**.
2. **Fix the files permanently.** Open the **Encoding** issues in
   **Manage → Doctor**, review the old and new value of every field, and
   confirm. Rainy rewrites the tags as UTF-8, so every other player and
   Subsonic client shows them correctly too. This requires a writable music
   folder.

Detection is conservative: text changes only when the decoded result is
plausible Chinese or Japanese, so normal Western tags such as `Exémplé` are left
alone.

## Related Docs

- [Management](management.md)
- [Configuration: server settings](../operations/configuration.md#server-settings)
- [Troubleshooting](../operations/troubleshooting.md)
