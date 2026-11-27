-- Folder access (docs/architecture/contract.md §5.1): besides whole libraries
-- (user_libraries), a user with restrict_libraries = 1 reads the folders listed in
-- user_folders: the tracks whose library-relative dir is the folder or lies below it.
-- Existing grants are unchanged.
CREATE TABLE user_folders (
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    path       TEXT    NOT NULL,           -- library-relative folder, forward slashes, never ""
    PRIMARY KEY (user_id, library_id, path)
);
CREATE INDEX idx_user_folders_library ON user_folders (library_id);
