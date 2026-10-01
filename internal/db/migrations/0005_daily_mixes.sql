-- Daily mixes (docs/architecture/contract.md §5.18): the songs picked for a user on one local
-- day, kept so the mix stays the same all day and on every device. Rows older than 30 days
-- are removed when a new mix is made.
CREATE TABLE daily_mixes (
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    day        TEXT    NOT NULL,           -- local date YYYY-MM-DD in the time zone that asked
    track_ids  TEXT    NOT NULL,           -- JSON array of track ids, in play order
    created_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, day)
);
