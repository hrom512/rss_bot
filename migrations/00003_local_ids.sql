-- +goose Up
ALTER TABLE feeds ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE filters ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

UPDATE feeds SET position = (
    SELECT COALESCE(COUNT(*), 0) + 1
    FROM feeds f2
    WHERE f2.chat_id = feeds.chat_id AND f2.id < feeds.id
);

UPDATE filters SET position = (
    SELECT COALESCE(COUNT(*), 0) + 1
    FROM filters f2
    WHERE f2.feed_id = filters.feed_id AND f2.id < filters.id);

CREATE UNIQUE INDEX IF NOT EXISTS feeds_chat_position ON feeds(chat_id, position);
CREATE UNIQUE INDEX IF NOT EXISTS filters_feed_position ON filters(feed_id, position);

-- +goose Down
DROP INDEX IF EXISTS feeds_chat_position;
DROP INDEX IF EXISTS filters_feed_position;
ALTER TABLE filters DROP COLUMN position;
ALTER TABLE feeds DROP COLUMN position;