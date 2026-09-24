# ADR-0001: Pure Go Runtime

## Status

Accepted.

## Context

Rainy targets NAS devices on both `amd64` and `arm64`. Reading and writing
audio tags and storing data in SQLite traditionally require C libraries
(TagLib and SQLite), which bring CGO, per-architecture C toolchains, slower
cross-compilation, and larger images. Development also happens on Windows,
where CGO toolchains are an extra burden.

## Decision

Build Rainy with `CGO_ENABLED=0` and use only pure-Go dependencies:

- SQLite through `modernc.org/sqlite`, a transpiled pure-Go SQLite.
- Tags through `go.senan.xyz/taglib`, which runs TagLib 2 compiled to
  WebAssembly inside the Go process. It reads and writes tags, audio
  properties, and embedded pictures.
- Image processing through the standard library and `golang.org/x/image`.

ffmpeg remains an external binary, shipped in the Docker image, and is used for
transcoding. It is also a fallback reader: when TagLib's WebAssembly build
cannot parse a file, `tags` reads tags, properties, and pictures through
ffprobe and ffmpeg instead. Writing such a file still requires TagLib and
reports a per-track error.

## Consequences

- One static binary cross-compiles for every target from any development
  machine, and the final image needs only Alpine, ffmpeg, and a few utilities.
- Contributors must not add CGO dependencies; the Docker image and release
  builds compile with `CGO_ENABLED=0`.
- The pure-Go SQLite and the WebAssembly TagLib are slower than their native
  counterparts. Scans compensate with parallel tag reading.
- A few real-world files that native TagLib would handle need the ffprobe
  fallback for reading and cannot be written.
- Without ffmpeg, Rainy still serves original files but cannot transcode.
