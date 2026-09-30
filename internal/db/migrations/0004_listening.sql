-- Listening reports and scrobbling to Last.fm / ListenBrainz (docs/architecture/contract.md
-- §5.16, §5.17).

-- play_history keeps what was played even after the track is purged: a snapshot of the
-- track's names, ids and duration at play time. Reports prefer the live track row and fall
-- back to the snapshot.
ALTER TABLE play_history ADD COLUMN title        TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN artist       TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN album        TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN album_artist TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN artist_id    TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN album_id     TEXT NOT NULL DEFAULT '';
ALTER TABLE play_history ADD COLUMN duration     REAL NOT NULL DEFAULT 0;

-- Backfill from the tracks that still exist (plays of already purged tracks stay blank).
UPDATE play_history SET (title, artist, album, album_artist, artist_id, album_id, duration) =
    (SELECT t.title, t.artist, t.album, t.album_artist, t.artist_id, t.album_id, t.duration
     FROM tracks t WHERE t.id = play_history.track_id)
WHERE EXISTS (SELECT 1 FROM tracks t WHERE t.id = play_history.track_id);

-- First plays per track ("new this period").
CREATE INDEX idx_play_history_user_track ON play_history (user_id, track_id, played_at);

-- A user's linked scrobbling account. The credential (Last.fm session key or ListenBrainz
-- user token) is encrypted with secret.key and never returned by the API.
CREATE TABLE scrobble_accounts (
    user_id        TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    service        TEXT    NOT NULL,                -- 'lastfm' | 'listenbrainz'
    username       TEXT    NOT NULL DEFAULT '',     -- the account name at the service
    credential_enc TEXT    NOT NULL DEFAULT '',     -- '' = the service revoked it; link again
    enabled        INTEGER NOT NULL DEFAULT 1,      -- the user paused scrobbling when 0
    last_error     TEXT    NOT NULL DEFAULT '',
    last_error_at  INTEGER NOT NULL DEFAULT 0,
    last_sent_at   INTEGER NOT NULL DEFAULT 0,      -- last accepted scrobble
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    PRIMARY KEY (user_id, service)
);

-- Plays waiting to be sent to a linked account. Rows are removed once the service accepted
-- or permanently refused them, or when they are too old to be accepted.
CREATE TABLE scrobble_queue (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    service         TEXT    NOT NULL,
    track_id        TEXT    NOT NULL,
    title           TEXT    NOT NULL,
    artist          TEXT    NOT NULL,
    album           TEXT    NOT NULL DEFAULT '',
    album_artist    TEXT    NOT NULL DEFAULT '',
    track_number    INTEGER NOT NULL DEFAULT 0,
    duration        REAL    NOT NULL DEFAULT 0,
    mbz_track_id    TEXT    NOT NULL DEFAULT '',
    played_at       INTEGER NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL
);
CREATE INDEX idx_scrobble_queue_due ON scrobble_queue (service, next_attempt_at);
CREATE INDEX idx_scrobble_queue_user ON scrobble_queue (user_id, service, played_at);
