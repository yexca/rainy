# Troubleshooting

Start with the container log: `docker compose logs --tail=200 rainy`. Rainy and
its entrypoint explain most permission and configuration problems there.

## The Page Does Not Open

- Check that the container is running: `docker compose ps`.
- Confirm the port mapping. The host side (`RAINY_HOST_PORT`, default `7650`)
  must not be used by another service, and the NAS firewall must allow it.
- If the health check stays unhealthy, make sure `RAINY_ADDRESS` and
  `RAINY_PORT` are not overridden inside the container; the check connects to
  `127.0.0.1:7650`.
- A configuration error, such as an invalid duration, stops startup with a
  message naming the variable.

## The Library Is Empty

- Open **Admin → Libraries** and check that the library path is `/data` and
  that it exists.
- Check that `RAINY_MUSIC_PATH` points at the right host folder.
- Check that `PUID:PGID` can read it. The log reports a folder that is not
  readable.
- Folders containing a `.rainyignore` file, hidden folders, and NAS system
  folders (`@eaDir`, `#recycle`, `#snapshot`, `$RECYCLE.BIN`, `lost+found`) are
  skipped. See [Library](../user/library.md#what-gets-scanned).
- After fixing the problem, start a scan from **Admin → Libraries**.

## New Music Does Not Appear

Rainy runs a quick scan every hour by default. Start a scan from **Admin →
Libraries**, or rescan one folder from **Manage → Folders**. Files uploaded or
edited through Rainy appear immediately.

## Management Reports Read-Only Files

The music volume is mounted with `:ro`, or `PUID:PGID` lacks write permission
on the files. Check the NAS shared-folder permissions and the file owners, for
example with `ls -ln /path/to/music/<album>`. See
[Docker](docker.md#permissions-puid-and-pgid).

## Tracks Show as Missing After a Disk or Share Went Offline

Rainy refuses to scan a library whose root is missing, unreadable, or suddenly
empty, so a disconnected share does not mark everything missing. Tracks marked
missing come back automatically when their files reappear. Purge them from
**Manage → Doctor** only when the files are gone for good.

## A Subsonic Client Cannot Connect

- Enter the server address without `/rest`, and check `http` versus `https`.
- Try an API key if the password contains unusual characters.
- After repeated failures, sign-in is locked for 5 minutes: 10 failed web
  sign-ins or 20 failed Subsonic authentications within 10 minutes. Wait and
  retry.

See [Clients](../user/clients.md).

## Transcoding Does Not Work

The image includes ffmpeg. When running the binary yourself, install ffmpeg or
set `RAINY_FFMPEG_PATH`. **Admin → System** shows whether ffmpeg was
found and its version. Without ffmpeg, Rainy streams original files.

## Scan Progress Does Not Move

A reverse proxy is buffering `/api/events`. Disable buffering for that path;
see [Reverse proxy](reverse-proxy.md).

## Uploads Fail Through the Proxy

Raise the proxy's request body limit (`client_max_body_size` in nginx), or
upload through the LAN address.

## Playback Stops on iOS When the Screen Locks

Use the installed PWA, or keep the Safari tab open. iOS limits background web
audio; lock-screen playback usually works, but the system may suspend a page
that stays in the background for a long time. Native clients such as Amperfy
or play:Sub are more robust for long sessions.

## Log Times Are Wrong

Set `TZ`, for example `TZ=Asia/Shanghai`. The web app always shows times in the
browser's time zone.

## A Forgotten Password

Reset it from the host:

```sh
docker compose exec -u 1000:1000 rainy rainy user reset-password <username> '<new-password>'
```

## Related Docs

- [Docker](docker.md)
- [Configuration](configuration.md)
- [Reverse proxy](reverse-proxy.md)
