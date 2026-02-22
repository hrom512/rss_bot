package scheduler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"rss_bot/internal/bot"
	"rss_bot/internal/fetcher"
	"rss_bot/internal/model"
	"rss_bot/internal/storage"
)

// Sender is the interface for sending Telegram messages.
type Sender interface {
	SendMessage(chatID int64, text string)
}

// Scheduler periodically checks RSS feeds and sends notifications.
type Scheduler struct {
	store   storage.Storage
	fetcher *fetcher.Fetcher
	sender  Sender
	log     *slog.Logger
	tick    time.Duration
}

// New creates a Scheduler with the default HTTP client.
func New(store storage.Storage, sender Sender, log *slog.Logger) *Scheduler {
	return &Scheduler{
		store:   store,
		fetcher: fetcher.New(http.DefaultClient),
		sender:  sender,
		log:     log,
		tick:    1 * time.Minute,
	}
}

// NewWithFetcher creates a Scheduler with a custom fetcher (useful for testing).
func NewWithFetcher(store storage.Storage, f *fetcher.Fetcher, sender Sender, log *slog.Logger) *Scheduler {
	return &Scheduler{
		store:   store,
		fetcher: f,
		sender:  sender,
		log:     log,
		tick:    1 * time.Minute,
	}
}

// SetTickInterval overrides the default 1-minute check interval.
func (s *Scheduler) SetTickInterval(d time.Duration) {
	s.tick = d
}

// Run starts the scheduler loop, blocking until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	s.checkAll(ctx)

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAll(ctx)
		}
	}
}

func (s *Scheduler) checkAll(ctx context.Context) {
	feeds, err := s.store.ListDueFeeds(ctx)
	if err != nil {
		s.log.Error("list due feeds", "error", err)
		return
	}

	for _, feed := range feeds {
		if ctx.Err() != nil {
			return
		}
		s.processFeed(ctx, feed)
	}
}

func (s *Scheduler) processFeed(ctx context.Context, feed model.Feed) {
	s.log.Debug("checking feed", "feed_id", feed.ID, "name", feed.Name)

	rssFeed, err := s.fetcher.Fetch(ctx, feed.URL)
	if err != nil {
		s.log.Error("fetch feed", "feed_id", feed.ID, "url", feed.URL, "error", err)
		s.updateLastCheck(ctx, &feed)
		return
	}

	filters, err := s.store.ListFilters(ctx, feed.ID)
	if err != nil {
		s.log.Error("list filters", "feed_id", feed.ID, "error", err)
		return
	}

	matched := fetcher.FilterItems(rssFeed.Items, filters)

	sent := 0
	for _, item := range matched {
		seen, err := s.store.IsSeen(ctx, feed.ID, item.GUID)
		if err != nil {
			s.log.Error("check seen", "feed_id", feed.ID, "guid", item.GUID, "error", err)
			continue
		}
		if seen {
			continue
		}

		msg := bot.FormatNotification(feed.Name, item)
		s.sender.SendMessage(feed.ChatID, msg)
		sent++

		if err := s.store.MarkSeen(ctx, feed.ID, item.GUID); err != nil {
			s.log.Error("mark seen", "feed_id", feed.ID, "guid", item.GUID, "error", err)
		}

		// Rate limit: ~20 messages/sec max for Telegram
		time.Sleep(50 * time.Millisecond)
	}

	if sent > 0 {
		s.log.Info("sent notifications", "feed_id", feed.ID, "name", feed.Name, "count", sent)
	}

	s.updateLastCheck(ctx, &feed)
}

func (s *Scheduler) updateLastCheck(ctx context.Context, feed *model.Feed) {
	now := time.Now().UTC()
	feed.LastCheckAt = &now
	if err := s.store.UpdateFeed(ctx, feed); err != nil {
		s.log.Error("update last check", "feed_id", feed.ID, "error", err)
	}
}
