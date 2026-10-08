#!/bin/sh
# Rainy container entrypoint.
#
# - Started as root (the image default): makes sure the data directory belongs to PUID:PGID
#   (default 1000:1000), applies UMASK and drops privileges with su-exec, so the server never
#   runs as root. The music directory is NEVER chowned — its ownership belongs to your NAS.
# - Started as another user (`docker run --user 1026:100`, compose `user:`): runs as that
#   user unchanged (PUID/PGID are ignored; the data directory must already be writable).
# - Arguments that are not a command are passed to rainy, so `docker run <image> version`
#   and `docker compose run --rm rainy user list` work.
set -eu

log() { printf '[entrypoint] %s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }

DATA_DIR="${RAINY_DATA_DIR:-/config}"
MUSIC_DIR="${RAINY_MUSIC_DIR:-/data}"
PUID="${PUID:-1000}"
PGID="${PGID:-1000}"
UMASK="${UMASK:-022}"

is_uint() { case "$1" in '' | *[!0-9]*) return 1 ;; *) return 0 ;; esac; }

# True when $1 is itself a read-only mount (e.g. `-v /path/to/music:/data:ro`). busybox `test -w`
# only looks at permission bits, so it can't tell.
is_ro_mount() {
	awk -v dir="$1" '$2 == dir { split($4, o, ","); ro = (o[1] == "ro") } END { exit !ro }' /proc/mounts 2>/dev/null
}

# `docker run <image> --help`, `docker run <image> serve` → rainy <args>
if [ "$#" -eq 0 ]; then
	set -- rainy serve
elif [ "${1#-}" != "$1" ] || ! command -v "$1" >/dev/null 2>&1; then
	set -- rainy "$@"
fi

case "$UMASK" in
'' | *[!0-7]* | ?????*) die "UMASK must be an octal value such as 022 or 002 (got '$UMASK')" ;;
esac
umask "$UMASK"

# Only the long-running server needs the ownership fix-up; one-off commands (version, user …)
# still drop privileges so they never create root-owned files in the data directory.
if [ "$(id -u)" = "0" ]; then
	is_uint "$PUID" || die "PUID must be a numeric user id (got '$PUID')"
	is_uint "$PGID" || die "PGID must be a numeric group id (got '$PGID')"

	if [ "$PUID" = "0" ]; then
		log "warning: PUID=0 — running Rainy as root is not recommended"
		exec "$@"
	fi

	mkdir -p "$DATA_DIR" || die "cannot create data directory $DATA_DIR"
	# Fix ownership only where it differs (fast on restarts, even with a large artwork cache).
	# -h never follows symlinks, so a link inside /config can't be used to chown files elsewhere.
	if ! find "$DATA_DIR" \( ! -user "$PUID" -o ! -group "$PGID" \) \
		-exec chown -h "$PUID:$PGID" {} + 2>/dev/null; then
		log "warning: could not change ownership of $DATA_DIR to $PUID:$PGID (NFS/SMB share with root squash?)"
	fi
	if ! su-exec "$PUID:$PGID" test -w "$DATA_DIR"; then
		die "$DATA_DIR is not writable by $PUID:$PGID — fix the host folder permissions or set PUID/PGID to its owner"
	fi

	if [ "${1:-}" = "rainy" ] && [ "${2:-serve}" = "serve" ]; then
		if [ ! -d "$MUSIC_DIR" ]; then
			log "note: music directory $MUSIC_DIR does not exist (mount your library there or add one in Admin → Libraries)"
		elif ! su-exec "$PUID:$PGID" test -r "$MUSIC_DIR" -a -x "$MUSIC_DIR"; then
			log "warning: $MUSIC_DIR is not readable by $PUID:$PGID — set PUID/PGID to the owner of your music folder"
		elif is_ro_mount "$MUSIC_DIR"; then
			log "note: $MUSIC_DIR is mounted read-only — playback works, tag editing/upload/rename are disabled"
		elif ! su-exec "$PUID:$PGID" test -w "$MUSIC_DIR"; then
			log "note: $MUSIC_DIR is read-only for $PUID:$PGID — playback works, tag editing/upload/rename are disabled"
		fi
	fi

	exec su-exec "$PUID:$PGID" "$@"
fi

exec "$@"
