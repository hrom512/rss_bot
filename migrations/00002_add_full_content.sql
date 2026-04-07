-- +goose Up
ALTER TABLE seen_items ADD COLUMN full_content TEXT;

-- +goose Down
-- SQLite doesn't support DROP COLUMN, so we recreate the table
CREATE TABLE IF NOT EXISTS seen_items_backup AS SELECT feed_id, guid, seen_at FROM seen_items;
DROP TABLE IF EXISTS seen_items;
ALTER TABLE seen_items_backup RENAME TO seen_items;