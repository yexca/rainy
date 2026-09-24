-- Rainy initial schema.
-- Conventions:
--   * all timestamps are INTEGER unix milliseconds (UTC); 0 means "never/unknown"
--   * booleans are INTEGER 0/1
--   * paths stored in the DB are library-relative and use forward slashes
--   * text columns are NOT NULL DEFAULT '' unless NULL carries meaning

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE libraries (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT    NOT NULL,
    path         TEXT    NOT NULL UNIQUE,          -- absolute OS path of the library root
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL,
    last_scan_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE users (
    id            TEXT    PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    display_name  TEXT    NOT NULL DEFAULT '',
    email         TEXT    NOT NULL DEFAULT '',
    password_enc  TEXT    NOT NULL,                 -- AES-256-GCM(password), base64; reversible (Subsonic token auth)
    is_admin      INTEGER NOT NULL DEFAULT 0,
    can_manage    INTEGER NOT NULL DEFAULT 0,       -- may edit tags / upload / rename / delete files
    can_download  INTEGER NOT NULL DEFAULT 1,
    api_key_hash  TEXT    UNIQUE,                   -- sha256 hex of OpenSubsonic API key, NULL if none
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    last_login_at INTEGER NOT NULL DEFAULT 0,
    last_seen_at  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE sessions (
    token_hash   TEXT    PRIMARY KEY,               -- sha256 hex of the session token
    user_id      TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_agent   TEXT    NOT NULL DEFAULT '',
    ip           TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions (user_id);

CREATE TABLE artists (
    id            TEXT    PRIMARY KEY,              -- util.HashID("artist", lower(name))
    name          TEXT    NOT NULL,
    sort_name     TEXT    NOT NULL,
    index_key     TEXT    NOT NULL,                 -- "A".."Z" or "#" (CJK -> pinyin initial)
    album_count   INTEGER NOT NULL DEFAULT 0,       -- albums where this artist is the album artist
    song_count    INTEGER NOT NULL DEFAULT 0,       -- tracks where artist_id or album_artist_id = id
    mbz_artist_id TEXT    NOT NULL DEFAULT '',
    image_path    TEXT    NOT NULL DEFAULT '',      -- absolute path to artist.* image, '' if none
    search_text   TEXT    NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);
CREATE INDEX idx_artists_sort ON artists (sort_name);

CREATE TABLE albums (
    id              TEXT    PRIMARY KEY,            -- util.HashID("album", lower(album_artist), lower(name))
    library_id      INTEGER NOT NULL,
    name            TEXT    NOT NULL,
    sort_name       TEXT    NOT NULL,
    album_artist    TEXT    NOT NULL,
    album_artist_id TEXT    NOT NULL,
    year            INTEGER NOT NULL DEFAULT 0,
    genre           TEXT    NOT NULL DEFAULT '',    -- most common genre among tracks
    compilation     INTEGER NOT NULL DEFAULT 0,
    song_count      INTEGER NOT NULL DEFAULT 0,
    disc_count      INTEGER NOT NULL DEFAULT 0,
    duration        REAL    NOT NULL DEFAULT 0,     -- seconds
    size            INTEGER NOT NULL DEFAULT 0,     -- bytes
    cover_path      TEXT    NOT NULL DEFAULT '',    -- absolute path of folder image (cover.jpg ...), '' if none
    cover_track_id  TEXT    NOT NULL DEFAULT '',    -- a track with embedded art, '' if none
    mbz_album_id    TEXT    NOT NULL DEFAULT '',
    search_text     TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL,               -- min(tracks.created_at) -> "recently added"
    updated_at      INTEGER NOT NULL                -- bumped whenever tracks/cover change (cover cache key)
);
CREATE INDEX idx_albums_artist ON albums (album_artist_id);
CREATE INDEX idx_albums_sort ON albums (sort_name);
CREATE INDEX idx_albums_created ON albums (created_at);
CREATE INDEX idx_albums_year ON albums (year);

CREATE TABLE tracks (
    id                  TEXT    PRIMARY KEY,        -- random util.NewID(), stable across renames/moves
    library_id          INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path                TEXT    NOT NULL,           -- library-relative, forward slashes, e.g. "Artist/Album/01 Song.flac"
    dir                 TEXT    NOT NULL,           -- library-relative parent dir ("" for root)
    filename            TEXT    NOT NULL,
    suffix              TEXT    NOT NULL,           -- lowercase extension without dot
    size                INTEGER NOT NULL,
    mtime               INTEGER NOT NULL,           -- file mtime, unix ms
    title               TEXT    NOT NULL,
    album               TEXT    NOT NULL,
    artist              TEXT    NOT NULL,
    album_artist        TEXT    NOT NULL,           -- effective album artist (see tags fallback rules)
    album_id            TEXT    NOT NULL,
    artist_id           TEXT    NOT NULL,
    album_artist_id     TEXT    NOT NULL,
    track_number        INTEGER NOT NULL DEFAULT 0,
    track_total         INTEGER NOT NULL DEFAULT 0,
    disc_number         INTEGER NOT NULL DEFAULT 0,
    disc_total          INTEGER NOT NULL DEFAULT 0,
    disc_subtitle       TEXT    NOT NULL DEFAULT '',
    year                INTEGER NOT NULL DEFAULT 0,
    date                TEXT    NOT NULL DEFAULT '',  -- raw DATE tag
    original_year       INTEGER NOT NULL DEFAULT 0,
    genre               TEXT    NOT NULL DEFAULT '',  -- genres joined with "; "
    composer            TEXT    NOT NULL DEFAULT '',
    comment             TEXT    NOT NULL DEFAULT '',
    lyrics              TEXT    NOT NULL DEFAULT '',  -- embedded LYRICS tag (plain or LRC text)
    has_lrc             INTEGER NOT NULL DEFAULT 0,   -- sidecar <basename>.lrc exists
    bpm                 INTEGER NOT NULL DEFAULT 0,
    compilation         INTEGER NOT NULL DEFAULT 0,
    duration            REAL    NOT NULL DEFAULT 0,   -- seconds
    bitrate             INTEGER NOT NULL DEFAULT 0,   -- kbit/s
    sample_rate         INTEGER NOT NULL DEFAULT 0,
    bit_depth           INTEGER NOT NULL DEFAULT 0,
    channels            INTEGER NOT NULL DEFAULT 0,
    codec               TEXT    NOT NULL DEFAULT '',  -- taglib format / inner codec, e.g. "flac", "mp4/alac"
    has_cover           INTEGER NOT NULL DEFAULT 0,   -- embedded picture present
    rg_track_gain       REAL,
    rg_track_peak       REAL,
    rg_album_gain       REAL,
    rg_album_peak       REAL,
    mbz_track_id        TEXT    NOT NULL DEFAULT '',
    mbz_album_id        TEXT    NOT NULL DEFAULT '',
    mbz_artist_id       TEXT    NOT NULL DEFAULT '',
    mbz_album_artist_id TEXT    NOT NULL DEFAULT '',
    sort_title          TEXT    NOT NULL DEFAULT '',
    sort_album          TEXT    NOT NULL DEFAULT '',
    sort_artist         TEXT    NOT NULL DEFAULT '',
    sort_album_artist   TEXT    NOT NULL DEFAULT '',
    search_text         TEXT    NOT NULL DEFAULT '',  -- util.NormalizeSearch(title artist album album_artist composer filename)
    missing             INTEGER NOT NULL DEFAULT 0,   -- file vanished during last scan (kept for playlists/annotations)
    created_at          INTEGER NOT NULL,             -- first discovery: min(file mtime, now)
    updated_at          INTEGER NOT NULL,
    UNIQUE (library_id, path)
);
CREATE INDEX idx_tracks_album ON tracks (album_id, disc_number, track_number);
CREATE INDEX idx_tracks_artist ON tracks (artist_id);
CREATE INDEX idx_tracks_album_artist ON tracks (album_artist_id);
CREATE INDEX idx_tracks_dir ON tracks (library_id, dir);
CREATE INDEX idx_tracks_created ON tracks (created_at);
CREATE INDEX idx_tracks_missing ON tracks (missing);

CREATE TABLE genres (
    id   TEXT PRIMARY KEY,                           -- util.HashID("genre", lower(name))
    name TEXT NOT NULL UNIQUE COLLATE NOCASE
);

CREATE TABLE track_genres (
    track_id TEXT NOT NULL REFERENCES tracks (id) ON DELETE CASCADE,
    genre_id TEXT NOT NULL REFERENCES genres (id) ON DELETE CASCADE,
    PRIMARY KEY (track_id, genre_id)
);
CREATE INDEX idx_track_genres_genre ON track_genres (genre_id);

CREATE TABLE annotations (
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    item_type  TEXT    NOT NULL,                     -- 'track' | 'album' | 'artist'
    item_id    TEXT    NOT NULL,
    starred_at INTEGER,                              -- NULL = not starred
    rating     INTEGER NOT NULL DEFAULT 0,           -- 0..5
    play_count INTEGER NOT NULL DEFAULT 0,
    played_at  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, item_type, item_id)
);
CREATE INDEX idx_annotations_starred ON annotations (user_id, item_type, starred_at);

CREATE TABLE play_history (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id   TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    track_id  TEXT    NOT NULL,
    played_at INTEGER NOT NULL,
    client    TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX idx_play_history_user ON play_history (user_id, played_at DESC);

CREATE TABLE playlists (
    id         TEXT    PRIMARY KEY,
    name       TEXT    NOT NULL,
    comment    TEXT    NOT NULL DEFAULT '',
    owner_id   TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public     INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX idx_playlists_owner ON playlists (owner_id);

CREATE TABLE playlist_tracks (
    playlist_id TEXT    NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,                    -- 0-based, contiguous
    track_id    TEXT    NOT NULL,                    -- no FK: entries survive missing/purged tracks (filtered by joins)
    PRIMARY KEY (playlist_id, position)
);
CREATE INDEX idx_playlist_tracks_track ON playlist_tracks (track_id);

CREATE TABLE play_queues (
    user_id     TEXT    PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    track_ids   TEXT    NOT NULL DEFAULT '[]',       -- JSON array of track ids
    current_id  TEXT    NOT NULL DEFAULT '',
    position_ms INTEGER NOT NULL DEFAULT 0,
    changed_by  TEXT    NOT NULL DEFAULT '',
    updated_at  INTEGER NOT NULL
);

CREATE TABLE bookmarks (
    user_id     TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    track_id    TEXT    NOT NULL,
    position_ms INTEGER NOT NULL DEFAULT 0,
    comment     TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    PRIMARY KEY (user_id, track_id)
);

CREATE TABLE radio_stations (
    id           TEXT    PRIMARY KEY,
    name         TEXT    NOT NULL,
    stream_url   TEXT    NOT NULL,
    homepage_url TEXT    NOT NULL DEFAULT '',
    created_at   INTEGER NOT NULL,
    updated_at   INTEGER NOT NULL
);

CREATE TABLE edit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    TEXT    NOT NULL DEFAULT '',
    username   TEXT    NOT NULL DEFAULT '',
    action     TEXT    NOT NULL,                     -- 'tags'|'cover'|'lyrics'|'rename'|'delete'|'restore'|'upload'|'purge'|'encoding'
    track_id   TEXT    NOT NULL DEFAULT '',
    path       TEXT    NOT NULL DEFAULT '',          -- library-relative path at time of action
    details    TEXT    NOT NULL DEFAULT '{}',        -- JSON, e.g. {"changes":{"TITLE":{"old":["a"],"new":["b"]}}}
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_edit_log_created ON edit_log (created_at DESC);
CREATE INDEX idx_edit_log_track ON edit_log (track_id);

CREATE TABLE trash (
    id            TEXT    PRIMARY KEY,
    library_id    INTEGER NOT NULL,
    original_path TEXT    NOT NULL,                  -- library-relative path before deletion
    trash_path    TEXT    NOT NULL,                  -- path relative to <data>/trash
    size          INTEGER NOT NULL DEFAULT 0,
    title         TEXT    NOT NULL DEFAULT '',
    artist        TEXT    NOT NULL DEFAULT '',
    album         TEXT    NOT NULL DEFAULT '',
    track_id      TEXT    NOT NULL DEFAULT '',
    deleted_by    TEXT    NOT NULL DEFAULT '',
    deleted_at    INTEGER NOT NULL
);
