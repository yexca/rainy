# ADR-0002: Simulated Folder Browsing

## Status

Accepted.

## Context

The Subsonic API has two browsing models: ID3-based browsing (`getArtists`,
`getArtist`, `getAlbum`) and folder-based browsing (`getIndexes`,
`getMusicDirectory`). Some clients use only the folder model. Exposing the real
directory tree would require a second identity for every folder, would expose
file-system layout to every listener, and would give different views of the
same music depending on how a client browses.

## Decision

Rainy simulates folder browsing from tags:

- `getMusicFolders` returns the configured libraries.
- `getIndexes` lists album artists.
- `getMusicDirectory` on an artist ID returns that artist's albums as
  directories, and on an album ID returns its songs.

Folder IDs are the same artist and album IDs used by the ID3 endpoints and the
native API.

## Consequences

- Both browsing models show the same library, and stars, ratings, and plays
  apply to the same items.
- A folder-mode client does not see the literal directory tree. Managers who
  need it use **Manage → Folders** in the web app.
- A clean `ALBUMARTIST` tag matters for every client; see the
  [library guide](../user/library.md#tags-that-matter).
