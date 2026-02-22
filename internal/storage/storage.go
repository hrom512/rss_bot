// Package storage defines the persistence interface and its implementations.
package storage

import (
	"context"

	"rss_bot/internal/model"
)

// Storage is the interface for all persistence operations.
type Storage interface {
	CreateFeed(ctx context.Context, feed *model.Feed) error
	GetFeed(ctx context.Context, id int64) (*model.Feed, error)
	ListFeeds(ctx context.Context, chatID int64) ([]model.Feed, error)
	ListDueFeeds(ctx context.Context) ([]model.Feed, error)
	UpdateFeed(ctx context.Context, feed *model.Feed) error
	DeleteFeed(ctx context.Context, id int64) error

	CreateFilter(ctx context.Context, f *model.Filter) error
	ListFilters(ctx context.Context, feedID int64) ([]model.Filter, error)
	GetFilter(ctx context.Context, id int64) (*model.Filter, error)
	DeleteFilter(ctx context.Context, id int64) error

	MarkSeen(ctx context.Context, feedID int64, guid string) error
	IsSeen(ctx context.Context, feedID int64, guid string) (bool, error)

	Close() error
}
