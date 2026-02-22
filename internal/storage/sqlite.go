package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // SQLite driver registration.

	"rss_bot/internal/model"
	"rss_bot/migrations"
)

const timeLayout = "2006-01-02T15:04:05Z"

// SQLite implements Storage backed by a SQLite database.
type SQLite struct {
	db *sql.DB
}

// NewSQLite opens a SQLite database at dsn and runs pending migrations.
func NewSQLite(dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("disable foreign keys: %w", err)
	}

	if err := migrations.Run(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &SQLite{db: db}, nil
}

// Close closes the underlying database connection.
func (s *SQLite) Close() error {
	return s.db.Close()
}

// CreateFeed inserts a new feed and populates its ID and CreatedAt.
func (s *SQLite) CreateFeed(ctx context.Context, feed *model.Feed) error {
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO feeds (chat_id, name, url, interval_minutes, is_active, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		feed.ChatID, feed.Name, feed.URL, feed.IntervalMinutes, boolToInt(feed.IsActive), now,
	)
	if err != nil {
		return fmt.Errorf("insert feed: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	feed.ID = id
	feed.CreatedAt, _ = time.Parse(timeLayout, now)
	return nil
}

// GetFeed returns a single feed by its ID.
func (s *SQLite) GetFeed(ctx context.Context, id int64) (*model.Feed, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, chat_id, name, url, interval_minutes, is_active, last_check_at, created_at
		 FROM feeds WHERE id = ?`, id,
	)
	return scanFeed(row)
}

// ListFeeds returns all feeds belonging to the given chat.
func (s *SQLite) ListFeeds(ctx context.Context, chatID int64) ([]model.Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chat_id, name, url, interval_minutes, is_active, last_check_at, created_at
		 FROM feeds WHERE chat_id = ? ORDER BY id`, chatID,
	)
	if err != nil {
		return nil, fmt.Errorf("query feeds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanFeeds(rows)
}

// ListDueFeeds returns all active feeds that are due for checking.
func (s *SQLite) ListDueFeeds(ctx context.Context) ([]model.Feed, error) {
	now := time.Now().UTC().Format(timeLayout)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, chat_id, name, url, interval_minutes, is_active, last_check_at, created_at
		 FROM feeds
		 WHERE is_active = 1
		   AND (last_check_at IS NULL
		        OR datetime(last_check_at, '+' || interval_minutes || ' minutes') <= datetime(?))`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("query due feeds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanFeeds(rows)
}

// UpdateFeed persists changes to an existing feed.
func (s *SQLite) UpdateFeed(ctx context.Context, feed *model.Feed) error {
	var lastCheck *string
	if feed.LastCheckAt != nil {
		v := feed.LastCheckAt.UTC().Format(timeLayout)
		lastCheck = &v
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET name = ?, url = ?, interval_minutes = ?, is_active = ?, last_check_at = ?
		 WHERE id = ?`,
		feed.Name, feed.URL, feed.IntervalMinutes, boolToInt(feed.IsActive), lastCheck, feed.ID,
	)
	if err != nil {
		return fmt.Errorf("update feed: %w", err)
	}
	return nil
}

// DeleteFeed removes a feed and its associated filters and seen items.
func (s *SQLite) DeleteFeed(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM seen_items WHERE feed_id = ?`, id); err != nil {
		return fmt.Errorf("delete seen_items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM filters WHERE feed_id = ?`, id); err != nil {
		return fmt.Errorf("delete filters: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM feeds WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	return tx.Commit()
}

// CreateFilter inserts a new filter and populates its ID and CreatedAt.
func (s *SQLite) CreateFilter(ctx context.Context, f *model.Filter) error {
	now := time.Now().UTC().Format(timeLayout)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO filters (feed_id, kind, scope, value, created_at) VALUES (?, ?, ?, ?, ?)`,
		f.FeedID, string(f.Kind), string(f.Scope), f.Value, now,
	)
	if err != nil {
		return fmt.Errorf("insert filter: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	f.ID = id
	f.CreatedAt, _ = time.Parse(timeLayout, now)
	return nil
}

// ListFilters returns all filters for the given feed.
func (s *SQLite) ListFilters(ctx context.Context, feedID int64) ([]model.Filter, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, feed_id, kind, scope, value, created_at FROM filters WHERE feed_id = ? ORDER BY id`, feedID,
	)
	if err != nil {
		return nil, fmt.Errorf("query filters: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var filters []model.Filter
	for rows.Next() {
		f, err := scanFilter(rows)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

// GetFilter returns a single filter by its ID.
func (s *SQLite) GetFilter(ctx context.Context, id int64) (*model.Filter, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, feed_id, kind, scope, value, created_at FROM filters WHERE id = ?`, id,
	)
	var f model.Filter
	var kindStr, scopeStr, createdStr string
	err := row.Scan(&f.ID, &f.FeedID, &kindStr, &scopeStr, &f.Value, &createdStr)
	if err != nil {
		return nil, fmt.Errorf("scan filter: %w", err)
	}
	f.Kind = model.FilterKind(kindStr)
	f.Scope = model.FilterScope(scopeStr)
	f.CreatedAt, _ = time.Parse(timeLayout, createdStr)
	return &f, nil
}

// DeleteFilter removes a filter by its ID.
func (s *SQLite) DeleteFilter(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM filters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete filter: %w", err)
	}
	return nil
}

// MarkSeen records that an RSS item has been processed.
func (s *SQLite) MarkSeen(ctx context.Context, feedID int64, guid string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO seen_items (feed_id, guid) VALUES (?, ?)`,
		feedID, guid,
	)
	if err != nil {
		return fmt.Errorf("mark seen: %w", err)
	}
	return nil
}

// IsSeen checks whether an RSS item has already been processed.
func (s *SQLite) IsSeen(ctx context.Context, feedID int64, guid string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM seen_items WHERE feed_id = ? AND guid = ?`,
		feedID, guid,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check seen: %w", err)
	}
	return count > 0, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type scannable interface {
	Scan(dest ...any) error
}

func scanFeed(row scannable) (*model.Feed, error) {
	var f model.Feed
	var isActive int
	var lastCheck, created sql.NullString
	err := row.Scan(&f.ID, &f.ChatID, &f.Name, &f.URL, &f.IntervalMinutes, &isActive, &lastCheck, &created)
	if err != nil {
		return nil, fmt.Errorf("scan feed: %w", err)
	}
	f.IsActive = isActive == 1
	if lastCheck.Valid {
		t, _ := time.Parse(timeLayout, lastCheck.String)
		f.LastCheckAt = &t
	}
	if created.Valid {
		f.CreatedAt, _ = time.Parse(timeLayout, created.String)
	}
	return &f, nil
}

func scanFeeds(rows *sql.Rows) ([]model.Feed, error) {
	var feeds []model.Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		feeds = append(feeds, *f)
	}
	return feeds, rows.Err()
}

func scanFilter(row scannable) (model.Filter, error) {
	var f model.Filter
	var kindStr, scopeStr, createdStr string
	err := row.Scan(&f.ID, &f.FeedID, &kindStr, &scopeStr, &f.Value, &createdStr)
	if err != nil {
		return f, fmt.Errorf("scan filter: %w", err)
	}
	f.Kind = model.FilterKind(kindStr)
	f.Scope = model.FilterScope(scopeStr)
	f.CreatedAt, _ = time.Parse(timeLayout, createdStr)
	return f, nil
}
