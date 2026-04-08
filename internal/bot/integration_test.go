package bot

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/h2non/gock"

	"rss_bot/internal/config"
	"rss_bot/internal/fetcher"
	"rss_bot/internal/storage"
)

// =============================================================================
// Test Data
// =============================================================================

// sampleRSSFeed contains 5 items with different DevOps topics.
var sampleRSSFeed = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>DevOps Weekly</title>
    <link>https://devops.example.com</link>
    <description>Weekly DevOps news</description>
    <item>
      <title>Kubernetes 1.32 Released</title>
      <link>https://devops.example.com/k8s-132</link>
      <description>New Kubernetes version with sidecar container support.</description>
      <guid>item-1</guid>
    </item>
    <item>
      <title>Docker Desktop Update</title>
      <link>https://devops.example.com/docker-update</link>
      <description>Docker Desktop gets new features.</description>
      <guid>item-2</guid>
    </item>
    <item>
      <title>DevOps Job Vacancy at BigCorp</title>
      <link>https://devops.example.com/vacancy</link>
      <description>We are hiring senior DevOps engineers.</description>
      <guid>item-3</guid>
    </item>
    <item>
      <title>Helm Chart Best Practices</title>
      <link>https://devops.example.com/helm-bp</link>
      <description>Learn how to write production-ready Helm charts.</description>
      <guid>item-4</guid>
    </item>
    <item>
      <title>Online Course: K8s Training</title>
      <link>https://devops.example.com/course</link>
      <description>Comprehensive training course covering K8s basics.</description>
      <guid>item-5</guid>
    </item>
  </channel>
</rss>`

// ========================================
// Integration Test Helpers
// ========================================

// cmd simulates a user typing a command in Telegram and verifies exact response.
func cmd(t *testing.T, api *fakeAPI, command string, handler func(), want string) {
	t.Helper()
	api.clear()
	handler()
	got := strings.TrimSpace(api.last())
	if got != want {
		t.Errorf("%s:\n  want: %q\n  got: %q", command, want, got)
	}
}

// setupBot creates a test bot with HTTP interception.
func setupBot(t *testing.T, xmlBody string) (*Bot, *fakeAPI, *storage.SQLite) {
	t.Helper()
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	api := &fakeAPI{}
	httpClient := http.DefaultClient
	gock.Intercept()

	if xmlBody != "" {
		for i := 0; i < 10; i++ {
			gock.New("https://devops.example.com/rss").
				Reply(200).
				BodyString(xmlBody)
		}
	}

	b := &Bot{
		api:     api,
		store:   store,
		cfg:     &config.Config{},
		fetcher: fetcher.New(httpClient),
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return b, api, store
}

// =============================================================================
// Fake Telegram API
// =============================================================================

// testMsg holds a captured message.
type testMsg struct {
	ChatID int64
	Text   string
}

// fakeAPI captures all messages sent by the bot.
type fakeAPI struct {
	mu   sync.Mutex
	msgs []testMsg
}

// Send captures messages.
func (m *fakeAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch cfg := c.(type) {
	case tgbotapi.MessageConfig:
		m.msgs = append(m.msgs, testMsg{ChatID: cfg.ChatID, Text: cfg.Text})
	case tgbotapi.VideoConfig:
		m.msgs = append(m.msgs, testMsg{ChatID: cfg.ChatID, Text: cfg.Caption})
	}
	return tgbotapi.Message{}, nil
}

func (m *fakeAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (m *fakeAPI) StopReceivingUpdates() {}

// last returns the most recent message.
func (m *fakeAPI) last() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.msgs) == 0 {
		return ""
	}
	return m.msgs[len(m.msgs)-1].Text
}

// clear clears captured messages.
func (m *fakeAPI) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.msgs = nil
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestIntegrationCommands(t *testing.T) {
	defer gock.Off()
	ctx := context.Background()

	t.Run("full command sequence", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		// клиент ввел /start -> получил полный текст приветствия
		cmd(t, api, "/start", func() { b.handleStart(chatID) },
			`Welcome to RSS Notify Bot!

Subscribe to RSS feeds and get filtered notifications.

Quick start:
1. /add <url> — add an RSS feed
2. /include <id> <word> — add a whitelist filter
3. /exclude <id> <word> — add a blacklist filter

Use /help for the full command reference.`)

		// клиент ввел /add https://devops.example.com/rss -> получил подтверждение
		cmd(t, api, "/add", func() {
			b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		}, `Feed added successfully!

#1 DevOps Weekly (every 15 min)
URL: https://devops.example.com/rss
No filters yet. Use /include, /exclude to add filters.`)

		feeds, _ := store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(1, len(feeds)); diff != "" {
			t.Errorf("feed count (-want +got):\n%s", diff)
		}

		// клиент ввел /include 1 kubernetes -> получил подтверждение
		cmd(t, api, "/include", func() {
			b.handleAddFilter(ctx, chatID, "1 kubernetes", "include")
		}, `Filter F1 added to #1 "DevOps Weekly".

Actual filters:

Include (word):
  F1: kubernetes (title+content)`)

		// клиент ввел /exclude 1 -s title spam -> получил подтверждение
		cmd(t, api, "/exclude", func() {
			b.handleAddFilter(ctx, chatID, "1 -s title spam", "exclude")
		}, `Filter F2 added to #1 "DevOps Weekly".

Actual filters:

Include (word):
  F1: kubernetes (title+content)

Exclude (word):
  F2: spam (title only)`)

		// клиент ввел /list -> получил список с фильтрами
		cmd(t, api, "/list", func() {
			b.handleList(ctx, chatID)
		}, `Your feeds:

#1 DevOps Weekly [active]
URL: https://devops.example.com/rss
Filters: 1 include, 1 exclude`)

		// клиент ввел /info 1 -> получил детали фида
		cmd(t, api, "/info", func() {
			b.handleInfo(ctx, chatID, "1")
		}, `#1 DevOps Weekly [active]
URL: https://devops.example.com/rss
Interval: every 15 min

Filters:

Include (word):
  F1: kubernetes (title+content)

Exclude (word):
  F2: spam (title only)`)

		// клиент ввел /filters 1 -> получил список фильтров
		cmd(t, api, "/filters", func() {
			b.handleFilters(ctx, chatID, "1")
		}, `Filters for #1 "DevOps Weekly":

Include (word):
  F1: kubernetes (title+content)

Exclude (word):
  F2: spam (title only)`)

		// клиент ввел /check 1 -> получил Found
		api.clear()
		b.handleCheck(ctx, chatID, "1")
		want := `Found 1 new item(s) in #1 "DevOps Weekly".`
		if got := api.last(); got != want {
			t.Errorf("/check:\n  want: %q\n  got: %q", want, got)
		}

		api.clear()
		feedAfterCheck, _ := store.GetFeed(ctx, feeds[0].ID)
		cmd(t, api, "/info after check", func() {
			b.handleInfo(ctx, chatID, "1")
		}, fmt.Sprintf(`#1 DevOps Weekly [active]
URL: https://devops.example.com/rss
Interval: every 15 min
Last check: %s

Filters:

Include (word):
  F1: kubernetes (title+content)

Exclude (word):
  F2: spam (title only)`, feedAfterCheck.LastCheckAt.Format("2006-01-02 15:04 UTC")))

		// клиент ввел /interval 1 30 -> получил подтверждение
		cmd(t, api, "/interval 1 30", func() {
			b.handleInterval(ctx, chatID, "1 30")
		}, `Feed #1 interval set to 30 min.`)

		feed, _ := store.GetFeed(ctx, feeds[0].ID)
		if diff := cmp.Diff(30, feed.IntervalMinutes); diff != "" {
			t.Errorf("interval (-want +got):\n%s", diff)
		}

		// клиент ввел /pause 1 -> получил подтверждение
		cmd(t, api, "/pause 1", func() {
			b.handlePause(ctx, chatID, "1")
		}, `Feed #1 "DevOps Weekly" paused.`)

		feed, _ = store.GetFeed(ctx, feeds[0].ID)
		if diff := cmp.Diff(false, feed.IsActive); diff != "" {
			t.Errorf("IsActive (-want +got):\n%s", diff)
		}

		// клиент ввел /resume 1 -> получил подтверждение
		cmd(t, api, "/resume 1", func() {
			b.handleResume(ctx, chatID, "1")
		}, `Feed #1 "DevOps Weekly" resumed.`)

		feed, _ = store.GetFeed(ctx, feeds[0].ID)
		if diff := cmp.Diff(true, feed.IsActive); diff != "" {
			t.Errorf("IsActive (-want +got):\n%s", diff)
		}

		// клиент ввел /rename 1 Kubernetes News -> получил подтверждение
		cmd(t, api, "/rename", func() {
			b.handleRename(ctx, chatID, "1 Kubernetes News")
		}, `Feed #1 renamed to "Kubernetes News".`)

		feed, _ = store.GetFeed(ctx, feeds[0].ID)
		if diff := cmp.Diff("Kubernetes News", feed.Name); diff != "" {
			t.Errorf("name (-want +got):\n%s", diff)
		}

		// клиент ввел /rmfilter 1 -> получил подтверждение
		cmd(t, api, "/rmfilter 1", func() {
			b.handleRmFilter(ctx, chatID, "1")
		}, `Filter F1 removed from #1 "Kubernetes News".

Actual filters:

Exclude (word):
  F1: spam (title only)`)

		filters, _ := store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(1, len(filters)); diff != "" {
			t.Errorf("filter count (-want +got):\n%s", diff)
		}

		// клиент ввел /remove 1 -> получил подтверждение
		cmd(t, api, "/remove 1", func() {
			b.handleRemove(ctx, chatID, "1")
		}, `Feed #1 "Kubernetes News" deleted.`)

		feeds, _ = store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(0, len(feeds)); diff != "" {
			t.Errorf("feeds should be empty (-want +got):\n%s", diff)
		}

		// клиент ввел /list (пустой) -> получил сообщение об отсутствии фидов
		cmd(t, api, "/list empty", func() {
			b.handleList(ctx, chatID)
		}, `You have no feeds yet. Use /add <url> to add one.`)
	})

	t.Run("formatting checks", func(t *testing.T) {
		b, api, _ := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatID, "1 k8s", "include")

		cmd(t, api, "/list", func() {
			b.handleList(ctx, chatID)
		}, `Your feeds:

#1 DevOps Weekly [active]
URL: https://devops.example.com/rss
Filters: 1 include, 0 exclude`)
	})

	t.Run("error handling", func(t *testing.T) {
		b, api, _ := setupBot(t, "")
		chatID := int64(100)

		tests := []struct {
			name string
			fn   func()
		}{
			{
				name: "add empty",
				fn:   func() { b.handleAdd(ctx, chatID, "") },
			},
			{
				name: "add bad URL",
				fn:   func() { b.handleAdd(ctx, chatID, "not-a-url") },
			},
			{
				name: "info empty",
				fn:   func() { b.handleInfo(ctx, chatID, "") },
			},
			{
				name: "info not found",
				fn:   func() { b.handleInfo(ctx, chatID, "999") },
			},
			{
				name: "remove empty",
				fn:   func() { b.handleRemove(ctx, chatID, "") },
			},
			{
				name: "remove not found",
				fn:   func() { b.handleRemove(ctx, chatID, "999") },
			},
			{
				name: "rename empty",
				fn:   func() { b.handleRename(ctx, chatID, "1") },
			},
			{
				name: "interval empty",
				fn:   func() { b.handleInterval(ctx, chatID, "1") },
			},
			{
				name: "interval invalid",
				fn:   func() { b.handleInterval(ctx, chatID, "1 abc") },
			},
			{
				name: "interval range",
				fn:   func() { b.handleInterval(ctx, chatID, "1 2000") },
			},
			{
				name: "pause empty",
				fn:   func() { b.handlePause(ctx, chatID, "") },
			},
			{
				name: "pause not found",
				fn:   func() { b.handlePause(ctx, chatID, "999") },
			},
			{
				name: "resume empty",
				fn:   func() { b.handleResume(ctx, chatID, "") },
			},
			{
				name: "resume not found",
				fn:   func() { b.handleResume(ctx, chatID, "999") },
			},
			{
				name: "check empty",
				fn:   func() { b.handleCheck(ctx, chatID, "") },
			},
			{
				name: "check not found",
				fn:   func() { b.handleCheck(ctx, chatID, "999") },
			},
			{
				name: "filters empty",
				fn:   func() { b.handleFilters(ctx, chatID, "") },
			},
			{
				name: "filters not found",
				fn:   func() { b.handleFilters(ctx, chatID, "999") },
			},
			{
				name: "include empty",
				fn:   func() { b.handleAddFilter(ctx, chatID, "", "include") },
			},
			{
				name: "include not found",
				fn:   func() { b.handleAddFilter(ctx, chatID, "999 word", "include") },
			},
			{
				name: "exclude empty",
				fn:   func() { b.handleAddFilter(ctx, chatID, "", "exclude") },
			},
			{
				name: "exclude not found",
				fn:   func() { b.handleAddFilter(ctx, chatID, "999 word", "exclude") },
			},
			{
				name: "rmfilter empty",
				fn:   func() { b.handleRmFilter(ctx, chatID, "") },
			},
			{
				name: "rmfilter not found",
				fn:   func() { b.handleRmFilter(ctx, chatID, "999") },
			},
		}

		for _, tt := range tests {
			api.clear()
			tt.fn()
			if api.last() == "" {
				t.Errorf("%s: empty response", tt.name)
			}
		}
	})
}

func TestIntegrationMultiClient(t *testing.T) {
	defer gock.Off()
	ctx := context.Background()

	t.Run("client isolation", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)
		chatA := int64(100)
		chatB := int64(200)

		b.handleAdd(ctx, chatA, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatA, "1 k8s", "include")

		api.clear()
		b.handleAdd(ctx, chatB, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatB, "1 spam", "exclude")

		feedsA, _ := store.ListFeeds(ctx, chatA)
		feedsB, _ := store.ListFeeds(ctx, chatB)

		if diff := cmp.Diff(1, len(feedsA)); diff != "" {
			t.Errorf("client A feed count (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(1, len(feedsB)); diff != "" {
			t.Errorf("client B feed count (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(1, feedsA[0].Position); diff != "" {
			t.Errorf("client A feed position (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(1, feedsB[0].Position); diff != "" {
			t.Errorf("client B feed position (-want +got):\n%s", diff)
		}

		cmd(t, api, "/remove", func() {
			b.handleRemove(ctx, chatA, "1")
		}, `Feed #1 "DevOps Weekly" deleted.`)

		feedsB, _ = store.ListFeeds(ctx, chatB)
		if diff := cmp.Diff(1, len(feedsB)); diff != "" {
			t.Errorf("client B feed count after A removal (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(1, feedsB[0].Position); diff != "" {
			t.Errorf("client B feed position changed (-want +got):\n%s", diff)
		}
	})
}

func TestIntegrationCallbacks(t *testing.T) {
	defer gock.Off()
	ctx := context.Background()
	testUser := &tgbotapi.User{ID: 42, UserName: "testuser"}

	t.Run("filters callback", func(t *testing.T) {
		b, api, _ := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatID, "1 k8s", "include")

		cb := &tgbotapi.CallbackQuery{
			ID:   "cb1",
			From: testUser,
			Data: "filters:1",
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID},
			},
		}
		b.handleCallback(ctx, cb)
		want := `Filters for #1 "DevOps Weekly":

Include (word):
  F1: k8s (title+content)`
		if got := strings.TrimSpace(api.last()); got != want {
			t.Errorf("callback filters:\n  want: %q\n  got: %q", want, got)
		}
	})

	t.Run("delete callback", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")

		feeds, _ := store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(1, len(feeds)); diff != "" {
			t.Errorf("expected 1 feed (-want +got):\n%s", diff)
		}

		cb := &tgbotapi.CallbackQuery{
			ID:   "cb2",
			From: testUser,
			Data: "delete:1",
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID},
			},
		}
		b.handleCallback(ctx, cb)
		want := `Feed #1 "DevOps Weekly" deleted.`
		if got := api.last(); got != want {
			t.Errorf("callback delete:\n  want: %q\n  got: %q", want, got)
		}

		feeds, _ = store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(0, len(feeds)); diff != "" {
			t.Errorf("feed should be deleted (-want +got):\n%s", diff)
		}
	})

	t.Run("rmfilter callback", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatID, "1 k8s", "include")

		feeds, _ := store.ListFeeds(ctx, chatID)
		filters, _ := store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(1, len(filters)); diff != "" {
			t.Errorf("expected 1 filter (-want +got):\n%s", diff)
		}

		cb := &tgbotapi.CallbackQuery{
			ID:   "cb4",
			From: testUser,
			Data: "rmfilter:1",
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID},
			},
		}
		b.handleCallback(ctx, cb)
		want := `Filter F1 removed from #1 "DevOps Weekly".

No filters for #1 "DevOps Weekly".
Use /include, /exclude, /include_re, /exclude_re to add filters.`
		if got := strings.TrimSpace(api.last()); got != want {
			t.Errorf("callback rmfilter:\n  want: %q\n  got: %q", want, got)
		}

		filters, _ = store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(0, len(filters)); diff != "" {
			t.Errorf("filter should be deleted (-want +got):\n%s", diff)
		}
	})

	t.Run("show more callback", func(t *testing.T) {
		b, api, s := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		feeds, _ := s.ListFeeds(ctx, chatID)

		fullContent := `<p>This is the <strong>full content</strong> of the article.</p>
<p>It contains multiple paragraphs:</p>
<ul>
<li>First list item</li>
<li>Second list item</li>
<li>Third list item</li>
</ul>
<p>And a code block:</p>
<pre><code>const foo = "bar";
console.log(foo);</code></pre>
<p>End of article.</p>`

		_ = s.MarkSeen(ctx, feeds[0].ID, "item-1", fullContent)

		cb := &tgbotapi.CallbackQuery{
			ID:   "cb5",
			From: testUser,
			Data: "show_more:1:item-1",
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID},
			},
		}
		b.handleCallback(ctx, cb)
		want := `[DevOps Weekly]

DevOps Weekly

This is the full content of the article.

It contains multiple paragraphs:

• First list item
• Second list item
• Third list item

And a code block:

const foo = "bar";
console.log(foo);
End of article.`
		if got := api.last(); got != want {
			t.Errorf("callback show_more:\n  want: %q\n  got: %q", want, got)
		}
	})

	t.Run("show more not found", func(t *testing.T) {
		b, api, _ := setupBot(t, sampleRSSFeed)
		chatID := int64(100)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")

		cb := &tgbotapi.CallbackQuery{
			ID:   "cb6",
			From: testUser,
			Data: "show_more:1:non-existent-item",
			Message: &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: chatID},
			},
		}
		b.handleCallback(ctx, cb)
		want := "Could not retrieve content."
		if got := api.last(); got != want {
			t.Errorf("callback show_more not found:\n  want: %q\n  got: %q", want, got)
		}
	})
}

func TestIntegrationPositionRecalculation(t *testing.T) {
	defer gock.Off()
	ctx := context.Background()
	chatID := int64(100)

	t.Run("feed position recalculation", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")

		feeds, _ := store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(3, len(feeds)); diff != "" {
			t.Errorf("expected 3 feeds (-want +got):\n%s", diff)
		}

		positions := []int{feeds[0].Position, feeds[1].Position, feeds[2].Position}
		if diff := cmp.Diff([]int{1, 2, 3}, positions); diff != "" {
			t.Errorf("positions (-want +got):\n%s", diff)
		}

		cmd(t, api, "/remove 2", func() {
			b.handleRemove(ctx, chatID, "2")
		}, `Feed #2 "DevOps Weekly" deleted.`)

		feeds, _ = store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(2, len(feeds)); diff != "" {
			t.Errorf("expected 2 feeds (-want +got):\n%s", diff)
		}

		positions = []int{feeds[0].Position, feeds[1].Position}
		if diff := cmp.Diff([]int{1, 2}, positions); diff != "" {
			t.Errorf("positions after delete (-want +got):\n%s", diff)
		}

		api.clear()
		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		feeds, _ = store.ListFeeds(ctx, chatID)
		if diff := cmp.Diff(3, len(feeds)); diff != "" {
			t.Errorf("expected 3 feeds after add (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(3, feeds[2].Position); diff != "" {
			t.Errorf("new feed position (-want +got):\n%s", diff)
		}

		api.clear()
		b.handleRemove(ctx, chatID, "1")
		feeds, _ = store.ListFeeds(ctx, chatID)
		positions = []int{feeds[0].Position, feeds[1].Position}
		if diff := cmp.Diff([]int{1, 2}, positions); diff != "" {
			t.Errorf("positions after first delete (-want +got):\n%s", diff)
		}
	})

	t.Run("filter position recalculation", func(t *testing.T) {
		b, api, store := setupBot(t, sampleRSSFeed)

		b.handleAdd(ctx, chatID, "https://devops.example.com/rss")
		b.handleAddFilter(ctx, chatID, "1 k8s", "include")
		b.handleAddFilter(ctx, chatID, "1 docker", "include")
		b.handleAddFilter(ctx, chatID, "1 helm", "include")

		feeds, _ := store.ListFeeds(ctx, chatID)
		filters, _ := store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(3, len(filters)); diff != "" {
			t.Errorf("expected 3 filters (-want +got):\n%s", diff)
		}

		positions := []int{filters[0].Position, filters[1].Position, filters[2].Position}
		if diff := cmp.Diff([]int{1, 2, 3}, positions); diff != "" {
			t.Errorf("filter positions (-want +got):\n%s", diff)
		}

		api.clear()
		b.handleRmFilter(ctx, chatID, "2")
		filters, _ = store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(2, len(filters)); diff != "" {
			t.Errorf("expected 2 filters (-want +got):\n%s", diff)
		}

		positions = []int{filters[0].Position, filters[1].Position}
		if diff := cmp.Diff([]int{1, 2}, positions); diff != "" {
			t.Errorf("filter positions after delete (-want +got):\n%s", diff)
		}

		api.clear()
		b.handleAddFilter(ctx, chatID, "1 -s title prometheus", "include")
		filters, _ = store.ListFilters(ctx, feeds[0].ID)
		if diff := cmp.Diff(3, len(filters)); diff != "" {
			t.Errorf("expected 3 filters after add (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff(3, filters[2].Position); diff != "" {
			t.Errorf("new filter position (-want +got):\n%s", diff)
		}
	})
}
