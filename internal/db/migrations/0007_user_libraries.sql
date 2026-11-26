-- Library access (docs/architecture/contract.md §5.1): a user with restrict_libraries = 1
-- reads only the libraries listed in user_libraries. Administrators and library managers
-- always read every library. Existing users keep access to every library.
ALTER TABLE users ADD COLUMN restrict_libraries INTEGER NOT NULL DEFAULT 0;

CREATE TABLE user_libraries (
    user_id    TEXT    NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    library_id INTEGER NOT NULL REFERENCES libraries (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, library_id)
);
CREATE INDEX idx_user_libraries_library ON user_libraries (library_id);
