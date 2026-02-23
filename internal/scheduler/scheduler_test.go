package scheduler

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"rss_bot/internal/fetcher"
	"rss_bot/internal/model"
	"rss_bot/internal/storage"
)

type sentMessage struct {
	ChatID int64
	Text   string
}

type mockSender struct {
	mu       sync.Mutex
	messages []sentMessage
}

func (m *mockSender) SendMessage(chatID int64, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, sentMessage{ChatID: chatID, Text: text})
}

func (m *mockSender) getMessages() []sentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]sentMessage, len(m.messages))
	copy(cp, m.messages)
	return cp
}

type mockHTTP struct {
	body string
}

func (m *mockHTTP) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
	}, nil
}

func loadFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sample.xml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func newTestStore(t *testing.T) *storage.SQLite {
	t.Helper()
	s, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSchedulerProcessesDueFeeds(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	xml := loadFixture(t)

	feed := model.Feed{
		ChatID:          100,
		Name:            "DevOps Weekly",
		URL:             "https://devops.example.com/rss",
		IntervalMinutes: 15,
		IsActive:        true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	if err := store.CreateFilter(ctx, &model.Filter{
		FeedID: feed.ID,
		Kind:   model.FilterInclude,
		Scope:  model.ScopeAll,
		Value:  "kubernetes",
	}); err != nil {
		t.Fatalf("create filter: %v", err)
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: xml}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()

	wantCount := 3
	if diff := cmp.Diff(wantCount, len(msgs)); diff != "" {
		t.Errorf("message count mismatch (-want +got):\n%s", diff)
		for i, m := range msgs {
			t.Logf("msg[%d]: %s", i, m.Text[:min(80, len(m.Text))])
		}
	}

	for _, m := range msgs {
		if diff := cmp.Diff(int64(100), m.ChatID); diff != "" {
			t.Errorf("chatID mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestSchedulerSkipsSeenItems(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	xml := loadFixture(t)

	feed := model.Feed{
		ChatID:          100,
		Name:            "Test",
		URL:             "https://example.com/rss",
		IntervalMinutes: 15,
		IsActive:        true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	// Mark all items as seen
	for _, guid := range []string{"item-1", "item-2", "item-3", "item-4", "item-5"} {
		if err := store.MarkSeen(ctx, feed.ID, guid); err != nil {
			t.Fatalf("mark seen %s: %v", guid, err)
		}
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: xml}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()
	if diff := cmp.Diff(0, len(msgs)); diff != "" {
		t.Errorf("expected no messages for seen items (-want +got):\n%s", diff)
	}
}

func TestSchedulerUpdatesLastCheck(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	xml := loadFixture(t)

	feed := model.Feed{
		ChatID:          100,
		Name:            "Test",
		URL:             "https://example.com/rss",
		IntervalMinutes: 15,
		IsActive:        true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: xml}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	before := time.Now().UTC().Add(-time.Second)

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	updated, err := store.GetFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if updated.LastCheckAt == nil {
		t.Fatal("expected LastCheckAt to be set")
	}
	if updated.LastCheckAt.Before(before) {
		t.Errorf("LastCheckAt %v is before test start %v", updated.LastCheckAt, before)
	}
}

func TestSchedulerWithExcludeFilter(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	xml := loadFixture(t)

	feed := model.Feed{
		ChatID:          100,
		Name:            "Test",
		URL:             "https://example.com/rss",
		IntervalMinutes: 15,
		IsActive:        true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	for _, fl := range []model.Filter{
		{FeedID: feed.ID, Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
		{FeedID: feed.ID, Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
		{FeedID: feed.ID, Kind: model.FilterExcludeRe, Scope: model.ScopeAll, Value: "course.*training"},
	} {
		if err := store.CreateFilter(ctx, &fl); err != nil {
			t.Fatalf("create filter: %v", err)
		}
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: xml}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()

	// items with "kubernetes": 1 (K8s 1.32), 4 (Helm + kubernetes in desc), 5 (K8s Training)
	// item 3 (vacancy) doesn't have kubernetes anyway
	// item 5 matches "course.*training" exclude
	wantCount := 2
	if diff := cmp.Diff(wantCount, len(msgs)); diff != "" {
		t.Errorf("message count mismatch (-want +got):\n%s", diff)
		for i, m := range msgs {
			t.Logf("msg[%d]: %s", i, m.Text[:min(80, len(m.Text))])
		}
	}
}

func TestSchedulerCancelledContext(t *testing.T) {
	store := newTestStore(t)
	xml := loadFixture(t)

	setupCtx := context.Background()
	feed := model.Feed{
		ChatID: 100, Name: "Test", URL: "https://example.com/rss",
		IntervalMinutes: 15, IsActive: true,
	}
	if err := store.CreateFeed(setupCtx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sender := &mockSender{}
	f := fetcher.New(&mockHTTP{body: xml})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()
	if diff := cmp.Diff(0, len(msgs)); diff != "" {
		t.Errorf("expected no messages when context cancelled (-want +got):\n%s", diff)
	}
}

func TestSchedulerRunStopsOnCancel(t *testing.T) {
	store := newTestStore(t)
	sender := &mockSender{}
	f := fetcher.New(&mockHTTP{body: "<rss><channel></channel></rss>"})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.SetTickInterval(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		sched.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
}

func TestSchedulerFetchError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	feed := model.Feed{
		ChatID: 100, Name: "Bad Feed", URL: "https://bad.example.com/rss",
		IntervalMinutes: 15, IsActive: true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: "not xml"}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()
	if diff := cmp.Diff(0, len(msgs)); diff != "" {
		t.Errorf("expected no messages on fetch error (-want +got):\n%s", diff)
	}

	// last_check_at should still be updated even on error
	updated, err := store.GetFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if updated.LastCheckAt == nil {
		t.Error("expected LastCheckAt to be set even after fetch error")
	}
}

func TestSchedulerNoFiltersPassesAll(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	xml := loadFixture(t)

	feed := model.Feed{
		ChatID: 100, Name: "Unfiltered", URL: "https://example.com/rss",
		IntervalMinutes: 15, IsActive: true,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	// No filters added

	sender := &mockSender{}
	httpClient := &mockHTTP{body: xml}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()
	wantCount := 5
	if diff := cmp.Diff(wantCount, len(msgs)); diff != "" {
		t.Errorf("expected all 5 items with no filters (-want +got):\n%s", diff)
	}
}

func TestSchedulerInactiveFeedSkipped(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	feed := model.Feed{
		ChatID: 100, Name: "Inactive", URL: "https://example.com/rss",
		IntervalMinutes: 15, IsActive: false,
	}
	if err := store.CreateFeed(ctx, &feed); err != nil {
		t.Fatalf("create feed: %v", err)
	}

	sender := &mockSender{}
	httpClient := &mockHTTP{body: "should not be fetched"}
	f := fetcher.New(httpClient)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	sched := NewWithFetcher(store, f, sender, log)
	sched.checkAll(ctx)

	msgs := sender.getMessages()
	if diff := cmp.Diff(0, len(msgs)); diff != "" {
		t.Errorf("inactive feed should not produce messages (-want +got):\n%s", diff)
	}
}
