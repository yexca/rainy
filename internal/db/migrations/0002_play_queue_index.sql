-- Index into track_ids of the current entry (a queue may hold the same song twice);
-- -1 = unknown (the first occurrence of current_id).
ALTER TABLE play_queues ADD COLUMN current_index INTEGER NOT NULL DEFAULT -1;
