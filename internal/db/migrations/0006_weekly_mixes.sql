-- Weekly mixes (docs/architecture/contract.md §5.18): the songs picked for a user for one
-- local week (Monday to Sunday), kept so the mix stays the same all week and on every device.
-- Rows older than 12 weeks are removed when a new mix is made.
CREATE TABLE weekly_mixes (
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    week       TEXT    NOT NULL,           -- local date YYYY-MM-DD of the week's Monday in the time zone that asked
    track_ids  TEXT    NOT NULL,           -- JSON array of track ids, in play order
    created_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, week)
);
