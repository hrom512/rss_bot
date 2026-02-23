-- +goose Up
CREATE TABLE IF NOT EXISTS feeds (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    chat_id           INTEGER NOT NULL,
    name              TEXT NOT NULL,
    url               TEXT NOT NULL,
    interval_minutes  INTEGER NOT NULL DEFAULT 15,
    is_active         INTEGER NOT NULL DEFAULT 1,
    last_check_at     TEXT,
    created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS filters (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    feed_id     INTEGER NOT NULL,
    kind        TEXT NOT NULL CHECK(kind IN ('include','exclude','include_re','exclude_re')),
    scope       TEXT NOT NULL DEFAULT 'all' CHECK(scope IN ('title','content','all')),
    value       TEXT NOT NULL,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS seen_items (
    feed_id    INTEGER NOT NULL,
    guid       TEXT NOT NULL,
    seen_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (feed_id, guid)
);

-- +goose Down
DROP TABLE IF EXISTS seen_items;
DROP TABLE IF EXISTS filters;
DROP TABLE IF EXISTS feeds;
