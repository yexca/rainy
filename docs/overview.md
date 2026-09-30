# Overview

Rainy is a self-hosted music server for NAS devices. It streams a music library
to its own installable web app and to any Subsonic or OpenSubsonic client, and
it lets trusted users manage that library from the browser.

## Goals

- Make a folder of music files browsable and playable from anywhere the owner
  chooses, on phones and desktops.
- Stay compatible with the existing ecosystem of Subsonic clients instead of
  requiring a Rainy-specific app.
- Treat library management as a core feature: tags, covers, lyrics, file names,
  uploads, and deletions should not require a separate desktop tool.
- Never lose user files: deletions go to a restorable trash, every file change
  is recorded in an edit history, and file operations stay inside the library
  roots.
- Run as one small container on modest NAS hardware, including ARM devices.

## Current Status

Rainy currently includes:

- A Go server with an embedded React web app, packaged as one Docker image for
  `linux/amd64` and `linux/arm64`.
- A SQLite database for the library index, users, sessions, playlists, play
  queues, annotations, settings, trash, and the edit log.
- A scanner that indexes libraries by tags, detects moved files, repairs
  legacy-encoded tags when reading, and runs on a schedule, at startup, or on
  demand.
- The native JSON API (`/api`) used by the web app, and a Subsonic API 1.16.1
  implementation with OpenSubsonic extensions (`/rest`).
- Raw streaming with HTTP range support and on-the-fly transcoding through
  ffmpeg.
- A web player with an iOS-style Now Playing screen, synced lyrics, queue
  editing, Media Session integration, and PWA installation.
- Management features: tag editing, cover art, lyrics, rename and organize,
  upload, trash, a library doctor, a folder browser, and an edit history, plus
  opt-in downloads from YouTube and bilibili links and online music through
  lx-music compatible music sources.
- Multiple users with administrator, manager, and download permissions, and
  multiple libraries.

## Current Limits

- One Rainy process owns one data directory. SQLite and the in-process scan
  lock assume a single instance.
- Folder browsing for Subsonic clients is simulated from tags (artist → album →
  songs) rather than mirroring the directory tree. See
  [ADR-0002](decisions/ADR-0002-simulated-folder-browsing.md).
- Rainy cannot be served under a URL sub-path; use a dedicated host name.
- Podcasts, shares, chat, and jukebox mode are not implemented; the Subsonic
  endpoints for them return empty results or "not supported".
- Known edge cases are listed under
  [Known limitations](architecture/contract.md#known-limitations) in the
  architecture contract.

## Related Docs

- [User guide](user/index.md)
- [Architecture](architecture/index.md)
- [Docker](operations/docker.md)
