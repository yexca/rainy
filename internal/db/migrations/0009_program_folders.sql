-- Program folders (docs/architecture/contract.md §5.1): a library folder whose tracks, in it
-- and in every folder below it, are spoken-word program episodes. They stay playable and
-- browsable by folder but are left out of recommendations, random picks, the home page and
-- the default album and song lists. Nothing else changes for existing installations.
CREATE TABLE program_folders (
    library_id INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,           -- library-relative folder, forward slashes, never ""
    created_at INTEGER NOT NULL,
    PRIMARY KEY (library_id, path)
);

-- A program library: every track of the library is a program episode, as if its root were a
-- program folder. Existing libraries stay music libraries.
ALTER TABLE libraries ADD COLUMN program INTEGER NOT NULL DEFAULT 0; -- 0/1
