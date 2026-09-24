# Privacy and Data Handling

Rainy is self-hosted software. This document describes the data flows
implemented by the current repository so operators can make informed
deployment decisions. It does not cover the privacy practices of your NAS
vendor, reverse proxy, VPN, hosting provider, or third-party Subsonic apps.

## No Telemetry

Rainy does not include analytics, crash reporting, advertising, update checks,
or any other project-operated service. The server makes no outbound network
requests of its own: covers, lyrics, similar songs, and artist information all
come from your own files and database. All data stays on your instance unless
you, a user, or a client app sends it elsewhere.

The only external connections happen in the browser:

- **Internet radio.** The browser connects directly to each station's stream
  URL, so the station sees the listener's IP address.
- **Links you open.** Links in the web app or documentation, such as to
  suggested client apps, open the linked site.

## Server-Side Data

The data folder (`/data` in Docker) contains:

| Location | Contents |
| --- | --- |
| `rainy.db` | Usernames, display names, email addresses (optional), roles, encrypted passwords, hashed API keys, sessions (hashed token, user agent, IP address, timestamps), favorites, ratings, play counts, play history with the client name, playlists, play queues, bookmarks, radio stations, server settings, the library index, trash entries, and the edit history (who changed which file, when, and the old and new values) |
| `secret.key` | The key that encrypts stored passwords |
| `trash/` | Music files deleted through Rainy, until the trash is emptied |
| `cache/` | Resized cover images |
| `tmp/` | Uploads in progress |

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
