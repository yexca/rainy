# Privacy and Data Handling

Rainy is self-hosted software. This document describes the data flows
implemented by the current repository so operators can make informed
deployment decisions. It does not cover the privacy practices of your NAS
vendor, reverse proxy, VPN, hosting provider, or third-party Subsonic apps.

## No Telemetry

Rainy does not include analytics, crash reporting, advertising, update checks,
or any other project-operated service. By default the server makes no outbound
network requests of its own: covers, lyrics, similar songs, and artist
information all come from your own files and database. All data stays on your
instance unless you, a user, or a client app sends it elsewhere. The
exceptions are the opt-in online metadata lookup and downloads from YouTube
and bilibili, both described below.

The only external connections happen in the browser:

- **Internet radio.** The browser connects directly to each station's stream
  URL, so the station sees the listener's IP address.
- **Links you open.** Links in the web app or documentation, such as to
  suggested client apps, open the linked site.

## Online Metadata Lookup

An administrator can turn on **Online metadata lookup** in **Admin →
Settings**. It is off by default. While it is on, managers can search online
music catalogues from the tag editor, and the Rainy server (not the browser)
contacts the chosen service:

| Service | Hosts contacted |
| --- | --- |
| NetEase Cloud Music | `music.163.com`, `*.music.126.net` (covers) |
| QQ Music | `u.y.qq.com`, `shc.y.qq.com`, `c.y.qq.com`, `y.gtimg.cn` (covers) |
| Kugou | `mobilecdn.kugou.com` (plain HTTP), `lyrics.kugou.com`, `imge.kugou.com` (covers) |
| Kuwo | `search.kuwo.cn`, `kuwo.cn`, `img*.kuwo.cn` (covers) |
| iTunes | `itunes.apple.com`, `*.mzstatic.com` (covers) |

A request is sent only when a manager searches or opens a result. It carries
the search terms typed into the dialog (prefilled from the track's title and
artist, or album and album artist), the selected result's id when lyrics are
fetched, and the server's IP address. Kugou's search endpoint only supports
plain HTTP, so those search terms travel unencrypted. If the administrator
also turns on **Pretend to search from mainland China** (off by default), the
requests to NetEase Cloud Music, QQ Music, Kugou, and Kuwo additionally carry
an `X-Real-IP` header with a random mainland-China address; it does not hide
the server's real address from these services. No account, cookie,
file, path, or listening data is sent. Results only fill the tag editor; files
change only when the manager saves. Rainy does not store search terms or
results on the server, and the logs record only the provider and error of a
failed lookup. Each service's own privacy policy applies to the requests it
receives. Outbound requests honor the `HTTPS_PROXY` and `HTTP_PROXY`
environment variables.

## Downloads from YouTube and Bilibili

An administrator can turn on **Allow downloads from YouTube and bilibili** in
**Admin → Settings → yt-dlp**. It is off by default. While it is off, none of
the requests below are made.

- **Installing and updating yt-dlp.** Only when an administrator selects
  **Check for updates** or **Install**, the server asks `api.github.com` for
  the latest yt-dlp release and downloads it and its checksum file from
  `github.com` (served from `*.githubusercontent.com`). These requests carry
  the server's IP address and a `Rainy/<version>` user agent.
- **Downloading.** Only when a manager starts a download, the server runs
  yt-dlp, which contacts YouTube (`youtube.com`, `googlevideo.com`, and other
  Google hosts) or bilibili (`bilibili.com`, its CDNs, and `b23.tv` for share
  links, which Rainy resolves itself) to fetch the video page, the audio, and
  the thumbnail. The site sees the server's IP address and the requested
  video. Nothing about your library, users, or listening is sent.
- **Sign-in cookies (optional).** An administrator can paste cookies exported
  from their browser so downloads run as their account. Cookies are account
  credentials. Rainy keeps only the cookies of the site's own domain, stores
  them encrypted with `secret.key` in `ytdlp/cookies/` with owner-only
  permissions, never returns them through the API, never writes them to logs,
  and never uploads them to any external service. During a download, yt-dlp
  sends them only to the site they belong to, as a browser would. Removing
  them in the settings deletes the file; sign out in the browser to revoke the
  session itself.

The edit history records who downloaded which file and its source link. Rainy
keeps the list of download jobs only in memory. yt-dlp's requests honor the
`HTTPS_PROXY` and `HTTP_PROXY` environment variables. Each site's own privacy
policy applies to the requests it receives.

## Server-Side Data

The data folder (`/data` in Docker) contains:

| Location | Contents |
| --- | --- |
| `rainy.db` | Usernames, display names, email addresses (optional), roles, encrypted passwords, hashed API keys, sessions (hashed token, user agent, IP address, timestamps), favorites, ratings, play counts, play history with the client name, playlists, play queues, bookmarks, radio stations, server settings, the library index, trash entries, and the edit history (who changed which file, when, and the old and new values) |
| `secret.key` | The key that encrypts stored passwords |
| `trash/` | Music files deleted through Rainy, until the trash is emptied |
| `cache/` | Resized cover images |
| `tmp/` | Uploads and downloads in progress |
| `ytdlp/` | The yt-dlp program installed by an administrator, its cache, and the encrypted sign-in cookies (`cookies/`) |

Passwords are encrypted rather than hashed because the Subsonic token protocol
needs the original password. Anyone who obtains both `rainy.db` and
`secret.key` can recover every user's password. Session tokens and API keys
are stored only as SHA-256 hashes.

Rainy also keeps some data only in memory: the "now playing" list (who is
playing what, on which client, for up to 15 minutes) and sign-in rate-limit
counters keyed by IP address and username. Both are lost on restart.

Rainy does not encrypt the database or your music at rest. Host permissions,
disk encryption, and backup controls protect them.

## What Other Users Can See

- Every signed-in user can browse the whole library.
- Public playlists, including their owner's name, are visible to all users.
- The "now playing" list shows every user's current track and client to other
  signed-in users.
- Administrators can see all accounts and their last sign-in times. Managers
  and administrators can see the edit history, including who made each change.

## Browser Data

The web app stores per-device preferences and state in the browser: theme,
accent color, language, playback settings, sidebar and column preferences,
recent searches, and the current play queue. The installed app's service
worker caches cover images, and that cache survives sign-out. Use a private
browser profile on shared devices.

## Subsonic Clients

Subsonic apps store the server address and your credentials or API key on the
device, and many send them in request URLs. Their own privacy practices are
outside Rainy's control. Use HTTPS outside your home network, and prefer an API
key that you can revoke.

## Logs, Diagnostics, and Support

Server logs may contain usernames, client IP addresses, library paths, file
names, and error details. The admin system page shows the data directory, the
library paths, and storage sizes.

Before sharing logs or screenshots, redact them. Do not post databases,
`secret.key`, passwords, session cookies, API keys, personal paths, or music
files in a public issue. Follow [SECURITY.md](SECURITY.md) for a suspected
vulnerability.

## Backup and Removal

Backups of the data folder carry the same sensitivity as the originals,
including recoverable passwords. Store them encrypted and access-controlled.

Deleting a user removes their sessions, favorites, ratings, play history,
playlists, queue, and bookmarks through database cascades. Edit-history entries
keep the username that made each change. Deleting a user, a database row, or
the container does not remove copies in backups, reverse-proxy logs, or client
apps; operators are responsible for those systems.

## Related Documentation

- [Security policy](SECURITY.md)
- [Deployment security](docs/operations/security.md)
- [Database operations](docs/operations/database.md)
- [Configuration](docs/operations/configuration.md)
