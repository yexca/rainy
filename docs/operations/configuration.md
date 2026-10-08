# Configuration

Rainy is configured in three layers:

1. **Compose variables** in a `.env` file beside `docker-compose.yml` choose the
   image, host paths, host port, and the environment passed to the container.
2. **Process environment variables** (`RAINY_*`, plus the container-only
   `PUID`, `PGID`, `UMASK`, and `TZ`) are read once at startup.
3. **Server settings** are edited in the web app under **Admin** (the
   Settings, yt-dlp, Sources, Scrobbling, and System tabs) and stored in the
   database.

## Compose Variables

Copy [`.env.example`](../../.env.example) to `.env` and change what you need.
Compose reads `.env` automatically, and exported shell variables take
precedence.

| Variable | Default | Purpose |
| --- | --- | --- |
| `RAINY_IMAGE` | `yexca/rainy:latest` | Image to run; pin a release tag or digest for reproducible deployments |
| `RAINY_MUSIC_PATH` | `./data` | Host music folder, mounted at `/data` |
| `RAINY_DATA_PATH` | `./config` | Host application-state folder, mounted at `/config` |
| `RAINY_HOST_PORT` | `7650` | Host port published for the container's port 7650 |
| `PUID` | `1000` | User ID that runs Rainy inside the container |
| `PGID` | `1000` | Group ID that runs Rainy inside the container |
| `UMASK` | `022` | Permission mask for files Rainy creates |
| `TZ` | `Asia/Shanghai` | Time zone for logs |
| `RAINY_SCAN_INTERVAL` | `1h` | Default periodic scan interval |
| `RAINY_SCAN_ON_START` | `true` | Quick scan at startup |
| `RAINY_LOG_LEVEL` | `info` | Log level |
| `RAINY_TRUST_PROXY` | `false` | Trust the client IP headers from a reverse proxy |
| `RAINY_SESSION_TTL` | `720h` | Web session lifetime |

A minimal `.env`:

```dotenv
RAINY_MUSIC_PATH=/path/to/music
PUID=1000
PGID=1000
```

After editing `.env`, validate and apply it. `docker compose restart` does not
apply changed variables.

```sh
docker compose config --quiet
docker compose up -d
```

## Process Environment Variables

All variables are optional. The Docker image sets the data and music paths and
JSON logging; everything else uses the program defaults.

| Variable | Default (binary) | Default (image) | Meaning |
| --- | --- | --- | --- |
| `RAINY_ADDRESS` | `0.0.0.0` | same | Listen address |
| `RAINY_PORT` | `7650` | same | Listen port; the health check uses it too |
| `RAINY_DATA_DIR` | `./config` | `/config` | Database, `secret.key`, caches, trash, and upload staging |
| `RAINY_MUSIC_DIR` | `./data` | `/data` | Default library, added automatically on first start when no library exists |
| `RAINY_SCAN_INTERVAL` | `1h` | same | Default for the `scanInterval` setting; `0` disables periodic scans |
| `RAINY_SCAN_ON_START` | `true` | same | Run a quick scan at startup |
| `RAINY_FFMPEG_PATH` | `ffmpeg` | same (bundled) | ffmpeg binary used for transcoding and tag fallbacks |
| `RAINY_YTDLP_PATH` | empty | empty | A yt-dlp binary you manage yourself. Empty: Rainy installs and updates yt-dlp in `<data>/ytdlp` when an administrator asks (useful when the data folder is mounted `noexec`) |
| `RAINY_LOG_LEVEL` | `info` | same | `debug`, `info`, `warn`, or `error` |
| `RAINY_LOG_FORMAT` | `text` | `json` | `text` or `json` |
| `RAINY_SESSION_TTL` | `720h` | same | Sliding web session lifetime (minimum `1m`) |
| `RAINY_TRUST_PROXY` | `false` | same | Honour `X-Real-IP` or the last `X-Forwarded-For` hop; enable only behind your own proxy |
| `RAINY_DEV_CORS` | empty | empty | Development only: one extra origin allowed to call `/api`, such as `http://localhost:5173` |

Durations accept Go syntax plus a day unit, for example `30m`, `12h`, `7d`, or
`1d12h`. Booleans accept `true`/`false`, `1`/`0`, `yes`/`no`, and `on`/`off`.
An invalid value stops startup with an error that names the variable.

### Container-Only Variables

| Variable | Default | Meaning |
| --- | --- | --- |
| `PUID` / `PGID` | `1000` / `1000` | The entrypoint makes `/config` owned by this user and group, then drops privileges to it. `/data` is never changed. |
| `UMASK` | `022` | Use `002` to make new files group-writable |
| `TZ` | `UTC` in the image | Time zone for server logs; the web app always shows times in the browser's time zone |

If the container is started with an explicit `user:` (Compose) or `--user`,
the entrypoint runs Rainy as that user and ignores `PUID` and `PGID`; the data
folder must then already be writable by that user. `PUID=0` runs as root and is
not recommended. See [Docker](docker.md#permissions-puid-and-pgid).

## Server Settings

Administrators change these in **Admin → Settings**. They apply to every user
and are stored in the `settings` table.

| Setting | Default | Meaning |
| --- | --- | --- |
| Scan interval | from `RAINY_SCAN_INTERVAL` | Periodic quick scan; `0` disables, otherwise at least `1m` |
| Genre separators | `;/,` | Characters that split one genre tag into several genres |
| Ignored articles | `The El La Los Las Le Les` | Leading words ignored when sorting and indexing names |
| Cover art files | `cover.*,folder.*,front.*,album.*,albumart*.*` | Folder image names, in priority order |
| Transcode format | `mp3` | Default target when a client limits the bitrate (`mp3`, `opus`, or `aac`) |
| Transcode bitrate | `192` | Default bitrate in kbps |
| Rename pattern | `{albumartist}/{album}/[{disc}-]{track:2} {title}` | Default pattern for rename and organized uploads |
| Repair garbled tags when reading | on | Show GBK, Big5, and Shift-JIS tags correctly without modifying files |
| Allow downloads | on | Let users with the download permission save original files |
| Online metadata lookup | off | Let managers search NetEase Cloud Music, QQ Music, Kugou, Kuwo, and iTunes from the tag editor. The server contacts these services only when someone searches; see [Privacy](../../PRIVACY.md#online-metadata-lookup). Outbound requests honor `HTTPS_PROXY` / `HTTP_PROXY`. |
| Downloads from YouTube and bilibili | off | On the **yt-dlp** tab. Let managers download audio from YouTube and bilibili links on the Tracks → Links tab. The server contacts those sites only when a download starts, and GitHub only when an administrator checks for or installs a yt-dlp update; see [Privacy](../../PRIVACY.md#downloads-from-youtube-and-bilibili). |
| Online music and music sources | off | On the **Sources** tab. Let managers search online catalogues in **Tracks → Online** and download songs into a library through lx-music custom source scripts that an administrator imports. Scripts run on the server and contact the servers their authors chose; see [Privacy](../../PRIVACY.md#online-music-and-music-sources). The same tab chooses automatic fallback between sources (by priority, the default) or one fixed source. |
| Scrobbling to Last.fm | off | On the **Scrobbling** tab, together with the Last.fm API key and shared secret of an API account you create at Last.fm (the secret is stored encrypted and never shown again). Users then connect their Last.fm account in **Settings → Scrobbling**, and the server sends their plays, "now playing", and loved (starred) songs to Last.fm; see [Privacy](../../PRIVACY.md#scrobbling-to-lastfm-and-listenbrainz). |
| Scrobbling to ListenBrainz | off | On the **Scrobbling** tab. Users paste their ListenBrainz user token in **Settings → Scrobbling**; no API account is needed. |
| Pretend to search from mainland China | off | With online lookup on, add an `X-Real-IP` header with a random mainland-China address to NetEase, QQ Music, Kugou, and Kuwo requests. Some songs are hidden from searches outside China; the spoofed header may or may not lift that. |

Once saved in the web app, the scan interval setting takes precedence over
`RAINY_SCAN_INTERVAL`.

## Data Directory Layout

```text
/config/rainy.db (+ -wal, -shm)   SQLite database
/config/secret.key                encryption key for stored passwords (created on first start)
/config/cache/artwork/            resized cover cache (safe to delete)
/config/trash/<libraryId>/...     deleted files, restorable from Manage → Trash
/config/tmp/                      upload staging and running downloads (ytdlp-job-*, online-job-*)
/config/ytdlp/                    yt-dlp installed by Rainy, its cache, and encrypted sign-in cookies
```

See [Database](database.md) for backups.

## Related Docs

- [Docker](docker.md)
- [Reverse proxy](reverse-proxy.md)
- [Deployment security](security.md)
