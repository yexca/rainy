# Library Management

Library management is what distinguishes Rainy from a read-only music server.
Managers can change tags, covers, lyrics, file names, and folder layout, upload
files, and delete them. Because these features write to the operator's own
music collection, they follow strict rules. The HTTP contract is in
[contract §7.6](contract.md#76-manage-requires-canmanage-or-admin--owner-manage-agent);
user-facing behavior is in [Management](../user/management.md).

## Rules for Every File Operation

Every operation in `internal/manage`:

1. Resolves paths with `util.SafeJoin(root, rel)`, which rejects absolute
   paths, `..`, NUL bytes, and results outside the library root. Symlink-aware
   checks guard folder and artist images.
2. Holds the scanner's library lock across the file change and the rescan, so a
   scheduled scan never sees a half-finished operation. HTTP handlers wait a
   bounded time for the lock and answer `409` when the library stays busy.
3. Rescans the affected paths afterwards (old and new paths for renames), so
   the database follows the disk.
4. Writes an `edit_log` row describing what changed.
5. Publishes a `library` event so open browsers refresh.

Read-only mounts fail gracefully with the `readonly` error and a helpful
message instead of partial writes.

## Tags

Tags are read and written through TagLib compiled to WebAssembly
(`go.senan.xyz/taglib`), using upper-case TagLib property names such as
`TITLE`, `ALBUMARTIST`, and `TRACKNUMBER`. A tag edit replaces only the listed
keys; an empty list deletes a key, and other tags are untouched. The web
editor sends per-track differences only, so a batch edit never overwrites a
field the user did not change.

When TagLib cannot parse a file, reading falls back to ffprobe/ffmpeg. Writing
such a file still requires TagLib and reports a per-track error.

Managers can explicitly rebuild tags for selected tracks. Rainy first reads the
existing tags, copies the file in its library folder, writes tags to the copy
through TagLib, verifies it, and then replaces the original. If the old tags
cannot be read, indexed fields provide a fallback. For WAV, legacy LIST/INFO
chunks that TagLib cannot parse become inert JUNK chunks; their bytes, the
audio, and any ID3 chunk stay in the file. The operation is logged and rescanned.

## Online Lookup

With the administrator setting `onlineMetadata` on (it is off by default),
the tag editor can search NetEase Cloud Music, QQ Music, Kugou, Kuwo, and the
iTunes Search API through `internal/metasearch`. This is the only code that
contacts third-party services, and it never writes files: a chosen result
fills the editor's draft (fields, cover, lyrics), and saving goes through the
normal tag, cover, and lyrics endpoints with their lock, edit log, and rescan.
Covers are proxied only from the providers' image hosts, so the browser does
not contact them and the proxy cannot reach other addresses. See
[contract §5.13](contract.md#513-metasearch-owner-manage-agent).

## Downloads from Links

With the administrator setting `ytdlpEnabled` on (off by default), managers can
paste a YouTube or bilibili link on the Tracks → Upload tab. The server runs
[yt-dlp](https://github.com/yt-dlp/yt-dlp) through `internal/ytdlp` with a fixed
argument list in a work directory under `<data>/tmp`: configuration files,
plugins, and the generic extractor are disabled, and only links to the
supported hosts are accepted, so yt-dlp cannot be pointed at other servers.
Rainy then writes the title, artist, album, date, source link, and a square
cover cropped from the thumbnail through TagLib, and imports the files exactly
like an upload: library lock, rescan, a `download` edit-log row per file, and
a library event. Existing files are never overwritten.

Administrators install and update yt-dlp from **Admin → Settings → yt-dlp**:
Rainy downloads the official release from GitHub into `<data>/ytdlp`, verifies
it against the release checksums, and checks that it runs before replacing the
old binary. Optional sign-in cookies are filtered to the site's own domain and
stored encrypted with `secret.key`; no endpoint returns them. See
[contract §5.14](contract.md#514-ytdlp-owner-manage-agent).

## Online Music

With the administrator setting `lxSourcesEnabled` on (off by default),
managers can search the five catalogues lx-music knows (Kuwo, Kugou, QQ Music,
NetEase Cloud Music, and Migu) in **Tracks → Online** and download songs into
a library through `internal/lxmusic`. Rainy ships no way to obtain audio
itself: the download link comes from an lx-music *custom source* script that
an administrator imports in **Admin → Settings → Sources**. Scripts run on the
server in a goja interpreter with only the lx-music script API; their network
access is limited to public internet addresses, and every call has a time
limit. With several sources, Rainy tries the enabled ones in priority order
until one delivers a file that is really audio (or uses one fixed source,
when the administrator chooses that); like lx-music, it asks a source for a
new link once when the first one has expired (HTTP 401, 403 or 410).

A downloaded file gets the catalogue's title, artists, album, lyrics (with a
paired translation), and cover written through TagLib, and is then imported
like an upload: library lock, rescan, a `download` edit-log row naming the
song's catalogue page, the quality, and the source, and a library event.
Existing files are never overwritten. See
[contract §5.15](contract.md#515-lxmusic-owner-manage-agent).

## Encoding Repair

Many older Chinese and Japanese files store GBK, Big5, or Shift-JIS text in tags
that claim to be Latin-1. With `fixEncodingOnScan` enabled (the default), the
scanner repairs this text when reading, without modifying files. The doctor's
encoding tool previews the same repair per field and, on confirmation, writes
the corrected text back as UTF-8. Detection is deliberately conservative so
legitimate Western text is left alone.

## Rename and Organize

Rename patterns use tokens such as `{albumartist}`, `{album}`, `{track:2}`, and
`{title}`, with `/` for directories and `[...]` for optional segments. Rainy
always previews a batch first, marking each item `ok`, `unchanged`,
`conflict`, or `invalid`. Illegal characters are replaced, sidecar `.lrc` files
move with their track, and folders left empty (or holding only NAS junk files
such as `@eaDir` or `.DS_Store`) are removed, never the library root.

## Trash

Deleting a track moves the file to `<data>/trash/<libraryId>/<original path>`
and records a trash entry. Replaced folder covers go to the trash as well.
Restoring moves the file back and rescans it; purging deletes it permanently.
Because the trash lives in the data directory, deleted music keeps using space
there until the trash is emptied.

## Uploads

Uploads are staged in `<data>/tmp` and then moved into the chosen library
folder, either keeping the uploaded folder structure or organizing by tags with
the rename pattern. One request is capped at 32 GiB. Uploads, link downloads,
and online music share the destination on the **Tracks** page's Upload and
Online tabs.

## Library Doctor

The doctor finds tracks with missing tags, albums without covers, likely
duplicates, missing files, and encoding problems, and links each issue to the
tool that fixes it.

## Related Docs

- [Backend](backend.md)
- [Secure development](../development/security.md)
- [Management guide](../user/management.md)
