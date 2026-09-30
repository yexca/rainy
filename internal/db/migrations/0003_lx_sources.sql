-- lx-music custom source scripts (docs/architecture/contract.md §5.15). An administrator
-- imports them; they turn online search results into download links.
CREATE TABLE lx_sources (
    id                 TEXT    PRIMARY KEY,
    name               TEXT    NOT NULL,
    description        TEXT    NOT NULL DEFAULT '',
    version            TEXT    NOT NULL DEFAULT '',
    author             TEXT    NOT NULL DEFAULT '',
    homepage           TEXT    NOT NULL DEFAULT '',
    source_url         TEXT    NOT NULL DEFAULT '',   -- where the script was imported from ('' = a file)
    script             TEXT    NOT NULL,
    script_hash        TEXT    NOT NULL UNIQUE,       -- sha256 of script (duplicate detection)
    enabled            INTEGER NOT NULL DEFAULT 1,
    position           INTEGER NOT NULL DEFAULT 0,    -- priority, lowest first
    allow_update_alert INTEGER NOT NULL DEFAULT 1,
    platforms          TEXT    NOT NULL DEFAULT '{}', -- JSON {"kw":["128k",…]} from the last successful start
    last_error         TEXT    NOT NULL DEFAULT '',   -- why the last start failed ('' = it worked)
    loaded_at          INTEGER NOT NULL DEFAULT 0,    -- last start attempt
    update_log         TEXT    NOT NULL DEFAULT '',   -- the script's latest update notice
    update_url         TEXT    NOT NULL DEFAULT '',
    update_at          INTEGER NOT NULL DEFAULT 0,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL
);
