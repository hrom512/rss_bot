package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"rss_bot/internal/model"
)

var ignoreTimestamps = cmpopts.IgnoreFields(model.Feed{}, "CreatedAt", "LastCheckAt")
var ignoreFilterTS = cmpopts.IgnoreFields(model.Filter{}, "CreatedAt")

func newTestDB(t *testing.T) *SQLite {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestFeedCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	tests := []struct {
		name string
		feed model.Feed
	}{
		{
			name: "basic feed",
			feed: model.Feed{
				ChatID:          12345,
				Name:            "Test Feed",
				URL:             "https://example.com/rss",
				IntervalMinutes: 15,
				IsActive:        true,
			},
		},
		{
			name: "inactive feed with custom interval",
			feed: model.Feed{
				ChatID:          67890,
				Name:            "Another Feed",
				URL:             "https://example.com/atom",
				IntervalMinutes: 60,
				IsActive:        false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			feed := tt.feed
			if err := s.CreateFeed(ctx, &feed); err != nil {
				t.Fatalf("create: %v", err)
			}
			if feed.ID == 0 {
				t.Fatal("expected non-zero ID")
			}

			got, err := s.GetFeed(ctx, feed.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}

			want := tt.feed
			want.ID = feed.ID
			if diff := cmp.Diff(want, *got, ignoreTimestamps); diff != "" {
				t.Errorf("GetFeed mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestListFeeds(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	chatID := int64(111)
	feeds := []model.Feed{
		{ChatID: chatID, Name: "Feed A", URL: "https://a.com/rss", IntervalMinutes: 10, IsActive: true},
		{ChatID: chatID, Name: "Feed B", URL: "https://b.com/rss", IntervalMinutes: 30, IsActive: false},
		{ChatID: 999, Name: "Other Chat", URL: "https://c.com/rss", IntervalMinutes: 15, IsActive: true},
	}
	for i := range feeds {
		if err := s.CreateFeed(ctx, &feeds[i]); err != nil {
			t.Fatalf("create feed %d: %v", i, err)
		}
	}

	got, err := s.ListFeeds(ctx, chatID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(got))
	}

	want := []model.Feed{
		{ID: feeds[0].ID, ChatID: chatID, Name: "Feed A", URL: "https://a.com/rss", IntervalMinutes: 10, IsActive: true},
		{ID: feeds[1].ID, ChatID: chatID, Name: "Feed B", URL: "https://b.com/rss", IntervalMinutes: 30, IsActive: false},
	}
	if diff := cmp.Diff(want, got, ignoreTimestamps); diff != "" {
		t.Errorf("ListFeeds mismatch (-want +got):\n%s", diff)
	}
}

func TestUpdateFeed(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	feed := model.Feed{ChatID: 1, Name: "Old", URL: "https://old.com", IntervalMinutes: 10, IsActive: true}
	if err := s.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	feed.Name = "New"
	feed.IntervalMinutes = 60
	feed.IsActive = false
	feed.LastCheckAt = &now

	if err := s.UpdateFeed(ctx, &feed); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := s.GetFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	want := model.Feed{
		ID: feed.ID, ChatID: 1, Name: "New", URL: "https://old.com",
		IntervalMinutes: 60, IsActive: false,
	}
	if diff := cmp.Diff(want, *got, ignoreTimestamps); diff != "" {
		t.Errorf("UpdateFeed mismatch (-want +got):\n%s", diff)
	}
	if got.LastCheckAt == nil {
		t.Fatal("expected LastCheckAt to be set")
	}
}

func TestDeleteFeedCascade(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	feed := model.Feed{ChatID: 1, Name: "F", URL: "https://f.com", IntervalMinutes: 15, IsActive: true}
	if err := s.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	f := model.Filter{FeedID: feed.ID, Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "test"}
	if err := s.CreateFilter(ctx, &f); err != nil {
		t.Fatalf("create filter: %v", err)
	}
	if err := s.MarkSeen(ctx, feed.ID, "guid-1"); err != nil {
		t.Fatalf("mark seen: %v", err)
	}

	if err := s.DeleteFeed(ctx, feed.ID); err != nil {
		t.Fatalf("delete feed: %v", err)
	}

	_, err := s.GetFeed(ctx, feed.ID)
	if err == nil {
		t.Fatal("expected error getting deleted feed")
	}

	filters, err := s.ListFilters(ctx, feed.ID)
	if err != nil {
		t.Fatalf("list filters: %v", err)
	}
	if len(filters) != 0 {
		t.Errorf("expected 0 filters, got %d", len(filters))
	}

	seen, err := s.IsSeen(ctx, feed.ID, "guid-1")
	if err != nil {
		t.Fatalf("is seen: %v", err)
	}
	if seen {
		t.Error("expected seen item to be deleted")
	}
}

func TestFilterCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	feed := model.Feed{ChatID: 1, Name: "F", URL: "https://f.com", IntervalMinutes: 15, IsActive: true}
	if err := s.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	tests := []struct {
		name   string
		filter model.Filter
	}{
		{
			name:   "include word",
			filter: model.Filter{FeedID: feed.ID, Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
		},
		{
			name:   "exclude regex title only",
			filter: model.Filter{FeedID: feed.ID, Kind: model.FilterExcludeRe, Scope: model.ScopeTitle, Value: "(?i)spam"},
		},
		{
			name:   "include regex content only",
			filter: model.Filter{FeedID: feed.ID, Kind: model.FilterIncludeRe, Scope: model.ScopeContent, Value: "(?i)docker|helm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.filter
			if err := s.CreateFilter(ctx, &f); err != nil {
				t.Fatalf("create: %v", err)
			}
			if f.ID == 0 {
				t.Fatal("expected non-zero ID")
			}

			got, err := s.GetFilter(ctx, f.ID)
			if err != nil {
				t.Fatalf("get: %v", err)
			}

			want := tt.filter
			want.ID = f.ID
			if diff := cmp.Diff(want, *got, ignoreFilterTS); diff != "" {
				t.Errorf("GetFilter mismatch (-want +got):\n%s", diff)
			}
		})
	}

	allFilters, err := s.ListFilters(ctx, feed.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(allFilters) != len(tests) {
		t.Fatalf("expected %d filters, got %d", len(tests), len(allFilters))
	}

	if err := s.DeleteFilter(ctx, allFilters[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	remaining, _ := s.ListFilters(ctx, feed.ID)
	if len(remaining) != len(tests)-1 {
		t.Errorf("expected %d filters after delete, got %d", len(tests)-1, len(remaining))
	}
}

func TestSeenItems(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	feed := model.Feed{ChatID: 1, Name: "F", URL: "https://f.com", IntervalMinutes: 15, IsActive: true}
	if err := s.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	tests := []struct {
		name     string
		guid     string
		wantSeen bool
	}{
		{name: "not seen yet", guid: "guid-1", wantSeen: false},
		{name: "after marking", guid: "guid-1", wantSeen: true},
	}

	// First check: not seen
	tt := tests[0]
	t.Run(tt.name, func(t *testing.T) {
		got, err := s.IsSeen(ctx, feed.ID, tt.guid)
		if err != nil {
			t.Fatalf("is seen: %v", err)
		}
		if diff := cmp.Diff(tt.wantSeen, got); diff != "" {
			t.Errorf("IsSeen mismatch (-want +got):\n%s", diff)
		}
	})

	if err := s.MarkSeen(ctx, feed.ID, "guid-1"); err != nil {
		t.Fatalf("mark seen: %v", err)
	}

	// Second check: seen
	tt = tests[1]
	t.Run(tt.name, func(t *testing.T) {
		got, err := s.IsSeen(ctx, feed.ID, tt.guid)
		if err != nil {
			t.Fatalf("is seen: %v", err)
		}
		if diff := cmp.Diff(tt.wantSeen, got); diff != "" {
			t.Errorf("IsSeen mismatch (-want +got):\n%s", diff)
		}
	})

	// Duplicate insert should not error
	if err := s.MarkSeen(ctx, feed.ID, "guid-1"); err != nil {
		t.Fatalf("mark seen duplicate: %v", err)
	}
}

func TestListDueFeeds(t *testing.T) {
	ctx := context.Background()
	s := newTestDB(t)

	past := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	recent := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)

	feeds := []struct {
		name    string
		feed    model.Feed
		wantDue bool
	}{
		{
			name:    "never checked",
			feed:    model.Feed{ChatID: 1, Name: "A", URL: "https://a.com", IntervalMinutes: 15, IsActive: true},
			wantDue: true,
		},
		{
			name:    "checked long ago",
			feed:    model.Feed{ChatID: 1, Name: "B", URL: "https://b.com", IntervalMinutes: 15, IsActive: true, LastCheckAt: &past},
			wantDue: true,
		},
		{
			name:    "checked recently",
			feed:    model.Feed{ChatID: 1, Name: "C", URL: "https://c.com", IntervalMinutes: 15, IsActive: true, LastCheckAt: &recent},
			wantDue: false,
		},
		{
			name:    "inactive",
			feed:    model.Feed{ChatID: 1, Name: "D", URL: "https://d.com", IntervalMinutes: 15, IsActive: false},
			wantDue: false,
		},
	}

	for i := range feeds {
		if err := s.CreateFeed(ctx, &feeds[i].feed); err != nil {
			t.Fatalf("create: %v", err)
		}
		if feeds[i].feed.LastCheckAt != nil {
			if err := s.UpdateFeed(ctx, &feeds[i].feed); err != nil {
				t.Fatalf("update: %v", err)
			}
		}
	}

	got, err := s.ListDueFeeds(ctx)
	if err != nil {
		t.Fatalf("list due: %v", err)
	}

	var wantIDs []int64
	for _, f := range feeds {
		if f.wantDue {
			wantIDs = append(wantIDs, f.feed.ID)
		}
	}

	var gotIDs []int64
	for _, f := range got {
		gotIDs = append(gotIDs, f.ID)
	}

	if diff := cmp.Diff(wantIDs, gotIDs); diff != "" {
		t.Errorf("due feed IDs mismatch (-want +got):\n%s", diff)
	}
}

// Ensure the Storage interface is satisfied.
var _ Storage = (*SQLite)(nil)
