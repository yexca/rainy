# Docker

Rainy ships as one Docker image for `linux/amd64` and `linux/arm64` (Synology
Plus models, ARM NAS devices, and Raspberry Pi 4/5 included). The image
contains the server, the web app, and ffmpeg. 32-bit ARM is not supported.

## Default Stack

The production [`docker-compose.yml`](../../docker-compose.yml) pulls
`${RAINY_IMAGE:-yexca/rainy:latest}` and does not build from source, so you only
need that file and a `.env` beside it:

```dotenv
RAINY_MUSIC_PATH=/path/to/music
PUID=1000
PGID=1000
TZ=Asia/Shanghai
```

```sh
docker compose up -d --pull always
docker compose logs -f rainy   # wait for "Rainy is listening"
```

Open `http://<host>:7650` and create the administrator account. Rainy adds
`/music` as the first library and starts a scan.

The service restarts automatically (`restart: unless-stopped`). Automatic
restarts and `docker compose restart` reuse the current image and do not
re-read `.env`; `docker compose up -d` applies configuration changes.

The container has two mounts:

| Host (`.env`) | Container | Contents | Access |
| --- | --- | --- | --- |
| `RAINY_DATA_PATH` (default `./data`) | `/data` | Database, `secret.key`, cover cache, trash, upload staging | Read-write; owned by `PUID:PGID` |
| `RAINY_MUSIC_PATH` | `/music` | Your music | Read for playback; read-write for management features |

All optional variables are listed in [Configuration](configuration.md).

### Standalone Compose Example

Some NAS interfaces ask you to paste a Compose file and do not offer a `.env`
file. Use literal values in that case:

```yaml
services:
  rainy:
    image: yexca/rainy:latest
    container_name: rainy
    restart: unless-stopped
    ports:
      - "7650:7650"
    environment:
      PUID: "1000"
      PGID: "1000"
      TZ: Asia/Shanghai
    volumes:
      - /path/to/rainy/data:/data
      - /path/to/music:/music
```

To change the host port, change only the left side, for example
`"17650:7650"`.

## Permissions (PUID and PGID)

The container starts as root only long enough to make `/data` owned by
`PUID:PGID`, applies `UMASK`, and then runs Rainy as that user. **`/music` is
never changed**; its permissions belong to your NAS.

1. Sign in to the NAS over SSH and run `id <your-user>`. For example,
   `uid=1026(example) gid=100(users)` means `PUID=1026` and `PGID=100`.
2. In the NAS shared-folder settings, give that user read-write access to the
   music folder, or read-only access if you only want playback.
3. For playback only, you can also mount the music read-only. Management
   features then report that files are read-only, and everything else works.
   In the standalone example, append `:ro` to the volume
   (`/path/to/music:/music:ro`). With the default Compose file, add a
   `docker-compose.override.yml` beside it:

   ```yaml
   services:
     rainy:
       volumes:
         - ${RAINY_MUSIC_PATH}:/music:ro
   ```

Set `UMASK=002` if other members of the group should be able to modify files
that Rainy writes. Alternatively, run the container with `user: "1026:100"`;
`PUID` and `PGID` are then ignored and the data folder must already be writable
by that user.

The startup log explains permission problems, for example that `/music` is
read-only for `1026:100` and editing is disabled.

## NAS Guides

All paths below are examples; replace them with your own.

### Synology (DSM 7.2+, Container Manager)

1. Install **Container Manager** from Package Center.
2. In File Station, create `docker/rainy` (for example `/volume1/docker/rainy`)
   with a `data` folder inside it.
3. Find the IDs: enable SSH under **Control Panel → Terminal & SNMP**, sign in,
   and run `id`. The first user you create is usually `uid=1026`, and the
   `users` group is `gid=100`.
4. Under **Control Panel → Shared Folder**, give that user read-write access to
   your music shared folder (read-only is enough for playback only) and to the
   `docker` shared folder.
5. In **Container Manager → Project → Create**, name the project `rainy`, choose
   `/volume1/docker/rainy` as the path, and paste the
   [standalone Compose example](#standalone-compose-example) with
   `/volume1/docker/rainy/data:/data`, `/volume1/music:/music`, `PUID: "1026"`,
   and `PGID: "100"`.
6. Finish the wizard and open `http://<nas-ip>:7650`.

Synology's `@eaDir` thumbnail folders and `#recycle` bins are ignored
automatically. For HTTPS, use the built-in reverse proxy described in
[Reverse proxy](reverse-proxy.md#synology-dsm).

### QNAP (Container Station 3)

1. Install **Container Station** from App Center.
2. In File Station, create `/share/Container/rainy/data`.
3. Run `id <user>` over SSH. Regular QNAP users usually start at `uid=500`, and
   the `everyone` group is `gid=100`. Do not use the `admin` account (uid 0).
4. Under **Control Panel → Privilege → Shared Folders**, give that user
   read-write access to your music share.
5. In **Container Station → Applications → Create**, paste the standalone
   example with these volumes:

   ```yaml
   - /share/Container/rainy/data:/data
   - /share/Multimedia/Music:/music
   ```

6. Open `http://<nas-ip>:7650`. If port 7650 is taken, change only the host
   side, for example `"17650:7650"`.

### Unraid

**Docker tab:** choose **Add Container** and set:

- Repository: `yexca/rainy:latest`
- Network type: `bridge`; port: container `7650` to host `7650`
- Path: container `/data` to `/mnt/user/appdata/rainy`
- Path: container `/music` to `/mnt/user/music` (Read/Write, or Read Only for
  playback only)
- Variables: `PUID=99`, `PGID=100`, `TZ=Asia/Shanghai`

Unraid shares belong to `nobody:users` (99:100) by default, which is why
`PUID=99` and `PGID=100` work.

**Compose Manager plugin:** install the Docker Compose Manager plugin, create a
stack, and paste the standalone example with the paths and IDs above.

### TrueNAS SCALE (24.10 and later)

1. Create two datasets, for example `tank/apps/rainy` for data and
   `tank/media/music` for music.
2. TrueNAS apps run as the `apps` user (`uid=568`, `gid=568`) by default. Edit
   the music dataset's ACL, give `apps` the **Modify** permission (or **Read**
   for playback only), and apply it recursively.
3. Open **Apps → Discover Apps**, choose **Install via YAML** from the menu,
   name the app `rainy`, and paste:

   ```yaml
   services:
     rainy:
       image: yexca/rainy:latest
       restart: unless-stopped
       ports: ["7650:7650"]
       environment:
         PUID: "568"
         PGID: "568"
         TZ: Asia/Shanghai
       volumes:
         - /mnt/tank/apps/rainy:/data
         - /mnt/tank/media/music:/music
   ```

4. Save and open `http://<nas-ip>:7650`.

Older Kubernetes-based SCALE releases (23.10 and earlier) use the Custom App
wizard with the same image, port, two host paths, and environment variables.

### OpenMediaVault (OMV 6 and 7)

1. Install [omv-extras](https://wiki.omv-extras.org/), then the
   **openmediavault-compose** plugin under **System → Plugins**.
2. Under **Storage → Shared Folders**, create an `appdata` folder and your
   `music` folder, and give your user read-write permission.
3. Run `id <user>` over SSH. Regular OMV users are usually `uid=1000`, and the
   `users` group is `gid=100`.
4. Under **Services → Compose → Files**, create a file from the standalone
   example. Use the absolute shared-folder path shown in the shared-folder
   list, for example `/srv/dev-disk-by-uuid-<id>/music:/music`, then choose
   **Up**.

### General Checks

- The page does not open: run `docker compose logs rainy`, and check the port
  mapping and the NAS firewall.
- The library is empty after a scan: check that the music volume points at the
  right folder and that `PUID:PGID` can read it.
- Management features report read-only files: the music volume is mounted with
  `:ro`, or `PUID:PGID` lacks write permission.

More answers are in [Troubleshooting](troubleshooting.md).

## Upgrade

Back up the data folder first (see [Database](database.md#backups)), then pull
the configured image and recreate the container:

```sh
docker compose up -d --pull always
docker image prune -f   # optional: remove old images
```

Plain `docker compose up -d` pulls a missing image and refreshes `latest`, but
reuses a cached image for other tags. Database migrations run automatically at
startup. Read the [release notes](../history/index.md) before upgrading across
several versions.

### Pin the Image

`latest` always follows the newest release. Each release is also published as
`<major>.<minor>.<patch>` (without the `v` of the Git tag) and as a moving
`<major>.<minor>` tag. For a reproducible deployment, set `RAINY_IMAGE` in
`.env` to a release tag from the
[releases page](https://github.com/yexca/rainy/releases), or to an image
digest, and change it deliberately during upgrades:

```dotenv
RAINY_IMAGE=yexca/rainy:0.1.0
# or
RAINY_IMAGE=yexca/rainy@sha256:<digest>
```

The same images are published to the GitHub Container Registry as
`ghcr.io/yexca/rainy`.

## Command-Line Tools

The `rainy` binary in the image has maintenance commands. `docker compose exec`
runs as root by default, so pass your `PUID:PGID` with `-u` to avoid creating
root-owned files in `/data`:

```sh
# Reset a forgotten password and sign the user out everywhere
docker compose exec -u 1000:1000 rainy rainy user reset-password admin 'new-password'

# List users
docker compose exec -u 1000:1000 rainy rainy user list

# Show the version
docker compose exec rainy rainy version
```

## Health Check

The Compose health check requests `http://127.0.0.1:7650/api/health` inside
the container. Leave `RAINY_ADDRESS` and `RAINY_PORT` at their defaults inside
the container, and change the published port with `RAINY_HOST_PORT` instead.

## Development Stack

[`deploy/compose/dev.yml`](../../deploy/compose/dev.yml) builds the image from
source and mounts the generated test library. Use it from the repository root
through the Makefile:

```sh
make testdata
make docker-up
```

See [Local development](../development/local-dev.md#docker).

## Related Docs

- [Configuration](configuration.md)
- [Reverse proxy](reverse-proxy.md)
- [Troubleshooting](troubleshooting.md)
