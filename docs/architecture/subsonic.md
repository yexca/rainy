# Subsonic API

Rainy implements the Subsonic API 1.16.1 with OpenSubsonic extensions under
`/rest`, so existing Subsonic clients work without a Rainy-specific app. The
full method list, envelope, error codes, and song fields are in
[contract §6](contract.md#6-subsonic--opensubsonic-rest--owner-subsonic-agent).
User-facing setup is in [Clients](../user/clients.md).

## Protocol

- Methods answer on `/rest/{method}` and `/rest/{method}.view`, with `GET` or
  form-encoded `POST` (OpenSubsonic `formPost`).
- Responses default to XML; `f=json` and `f=jsonp` are supported. The envelope
  reports `type="rainy"`, the server version, and `openSubsonic=true`.
- CORS allows any origin so browser-based clients can connect.

## Authentication

Clients authenticate with one of:

- `u` + `p` (plain or `enc:`-hex encoded password).
- `u` + `t` + `s`, where `t = md5(password + s)` (token authentication).
- `apiKey` (OpenSubsonic `apiKeyAuthentication`). Users create a key in
  **Settings → Subsonic apps**; it is shown once and stored as a hash.

Token authentication is why Rainy stores passwords reversibly encrypted; see
[Data model](data-model.md#passwords-and-keys). Failed authentications are rate
limited separately from the web sign-in: 20 failures within 10 minutes per
client IP or username lock that key for 5 minutes, because clients retry stored
credentials automatically.

## Advertised Extensions

`formPost`, `songLyrics`, `transcodeOffset`, `apiKeyAuthentication`, and
`indexBasedQueue`.

## Behavior Worth Knowing

- **IDs are shared** with the native API, so a track starred in a client is
  starred in the web app.
- **Folder browsing is simulated** from tags: `getIndexes` lists album artists,
  `getMusicDirectory` on an artist returns albums as directories, and on an
  album returns its songs. `getMusicFolders` returns the libraries. See
  [ADR-0002](../decisions/ADR-0002-simulated-folder-browsing.md).
- **`search3` with an empty query** returns everything, paged. Clients such as
  Symfonium use this to synchronize the whole library.
- **Streaming** honours `maxBitRate`, `format` (`raw` means the original),
  `timeOffset`, and `estimateContentLength`. Original files support HTTP range
  requests. Formats most clients cannot decode (APE, WMA, DSF/DFF, WavPack,
  Musepack, TTA) are transcoded to the server default unless `format=raw`.
- **Play queues** are shared with the web player, including the index-based
  variants, so playback can move between devices.
- **Scrobbling** with `submission=false` updates "now playing"; `true`
  records a play.
- **Similar and top songs** are computed locally from the library; Rainy makes
  no external lookups.
- **Administration** (`startScan`, user management, radio station writes)
  requires an administrator.
- **Not implemented:** podcasts, shares, chat, and jukebox return empty lists
  or error 0 ("not supported"); `getAvatar` returns "not found".

## Changing the Subsonic API

Subsonic clients cache aggressively and are maintained by third parties.
Treat response fields, IDs, and error codes as a public compatibility surface:
add fields rather than renaming them, and keep the contract section current.

## Related Docs

- [Native API](native-api.md)
- [Clients](../user/clients.md)
- [Deployment security](../operations/security.md)
