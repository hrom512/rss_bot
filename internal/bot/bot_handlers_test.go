package bot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-cmp/cmp"

	"rss_bot/internal/config"
	"rss_bot/internal/fetcher"
	"rss_bot/internal/model"
	"rss_bot/internal/storage"
)

// --- mocks ---

type sentMsg struct {
	ChatID int64
	Text   string
}

type mockAPI struct {
	mu   sync.Mutex
	sent []sentMsg
}

func (m *mockAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		m.mu.Lock()
		m.sent = append(m.sent, sentMsg{ChatID: msg.ChatID, Text: msg.Text})
		m.mu.Unlock()
	}
	return tgbotapi.Message{}, nil
}

func (m *mockAPI) GetUpdatesChan(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return make(tgbotapi.UpdatesChannel)
}

func (m *mockAPI) StopReceivingUpdates() {}

func (m *mockAPI) lastText() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.sent) == 0 {
		return ""
	}
	return m.sent[len(m.sent)-1].Text
}

func (m *mockAPI) allTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.sent))
	for i, s := range m.sent {
		out[i] = s.Text
	}
	return out
}

func (m *mockAPI) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = nil
}

type mockHTTPClient struct {
	body string
	err  error
}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
	}, nil
}

// --- helpers ---

func newTestBot(t *testing.T, httpBody string) (*Bot, *mockAPI, *storage.SQLite) {
	t.Helper()
	store, err := storage.NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	api := &mockAPI{}
	b := &Bot{
		api:     api,
		store:   store,
		cfg:     &config.Config{},
		fetcher: fetcher.New(&mockHTTPClient{body: httpBody}),
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return b, api, store
}

func seedFeed(t *testing.T, store *storage.SQLite, chatID int64, name, url string) *model.Feed {
	t.Helper()
	f := &model.Feed{ChatID: chatID, Name: name, URL: url, IntervalMinutes: 15, IsActive: true}
	if err := store.CreateFeed(context.Background(), f); err != nil {
		t.Fatalf("seed feed: %v", err)
	}
	return f
}

func seedFilter(t *testing.T, store *storage.SQLite, feedID int64, kind model.FilterKind, value string) *model.Filter {
	t.Helper()
	f := &model.Filter{FeedID: feedID, Kind: kind, Scope: model.ScopeAll, Value: value}
	if err := store.CreateFilter(context.Background(), f); err != nil {
		t.Fatalf("seed filter: %v", err)
	}
	return f
}

func loadSampleXML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sample.xml")
	if err != nil {
		t.Fatalf("read sample xml: %v", err)
	}
	return string(data)
}

func requireContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("reply missing %q, got:\n%s", want, got)
	}
}

// --- handler tests ---

func TestHandleStart(t *testing.T) {
	b, api, _ := newTestBot(t, "")
	b.handleStart(100)
	requireContains(t, api.lastText(), "Welcome to RSS Notify Bot")
}

func TestHandleHelp(t *testing.T) {
	b, api, _ := newTestBot(t, "")
	b.handleHelp(100)
	requireContains(t, api.lastText(), "/add")
	requireContains(t, api.lastText(), "/filters")
}

func TestHandleAdd(t *testing.T) {
	xml := loadSampleXML(t)
	ctx := context.Background()

	t.Run("empty args", func(t *testing.T) {
		b, api, _ := newTestBot(t, xml)
		b.handleAdd(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /add")
	})

	t.Run("fetch error", func(t *testing.T) {
		b, api, _ := newTestBot(t, "not xml at all")
		b.handleAdd(ctx, 100, "https://bad.example.com")
		requireContains(t, api.lastText(), "Failed to fetch feed")
	})

	t.Run("success uses feed title", func(t *testing.T) {
		b, api, store := newTestBot(t, xml)
		b.handleAdd(ctx, 100, "https://devops.example.com/rss")
		requireContains(t, api.lastText(), "Feed added")
		requireContains(t, api.lastText(), "DevOps Weekly")

		feeds, _ := store.ListFeeds(ctx, 100)
		if diff := cmp.Diff(1, len(feeds)); diff != "" {
			t.Errorf("feed count (-want +got):\n%s", diff)
		}
		if diff := cmp.Diff("DevOps Weekly", feeds[0].Name); diff != "" {
			t.Errorf("feed name (-want +got):\n%s", diff)
		}
	})

	t.Run("success fallback to url", func(t *testing.T) {
		noTitle := `<?xml version="1.0"?><rss><channel><title></title></channel></rss>`
		b, api, _ := newTestBot(t, noTitle)
		b.handleAdd(ctx, 100, "https://example.com/feed")
		requireContains(t, api.lastText(), "https://example.com/feed")
	})
}

func TestHandleList(t *testing.T) {
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleList(ctx, 100)
		requireContains(t, api.lastText(), "no feeds yet")
	})

	t.Run("with feeds and filters", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f1 := seedFeed(t, store, 100, "Feed A", "https://a.com")
		seedFeed(t, store, 100, "Feed B", "https://b.com")
		seedFilter(t, store, f1.ID, model.FilterInclude, "go")
		seedFilter(t, store, f1.ID, model.FilterExclude, "spam")

		b.handleList(ctx, 100)
		reply := api.lastText()
		requireContains(t, reply, "#1 Feed A")
		requireContains(t, reply, "#2 Feed B")
		requireContains(t, reply, "1 include, 1 exclude")
	})
}

func TestHandleInfo(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleInfo(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /info")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleInfo(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("wrong chat", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 200, "Other", "https://other.com")
		b.handleInfo(ctx, 100, "1")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "My Feed", "https://my.com/rss")
		b.handleInfo(ctx, 100, "1")
		reply := api.lastText()
		requireContains(t, reply, "#1 My Feed")
		requireContains(t, reply, "https://my.com/rss")
	})
}

func TestHandleRemove(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRemove(ctx, 100, "abc")
		requireContains(t, api.lastText(), "Usage: /remove")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRemove(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("wrong chat", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 200, "Other", "https://x.com")
		b.handleRemove(ctx, 100, "1")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Doomed", "https://bye.com")
		b.handleRemove(ctx, 100, "1")
		requireContains(t, api.lastText(), `"Doomed" deleted`)

		feeds, _ := store.ListFeeds(ctx, 100)
		if diff := cmp.Diff(0, len(feeds)); diff != "" {
			t.Errorf("feeds should be empty (-want +got):\n%s", diff)
		}
	})
}

func TestHandleRename(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRename(ctx, 100, "1")
		requireContains(t, api.lastText(), "/rename")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRename(ctx, 100, "999 New Name")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Old", "https://x.com")
		b.handleRename(ctx, 100, "1 New Name")
		requireContains(t, api.lastText(), `renamed to "New Name"`)

		feed, _ := store.GetFeed(ctx, 1)
		if diff := cmp.Diff("New Name", feed.Name); diff != "" {
			t.Errorf("name (-want +got):\n%s", diff)
		}
	})
}

func TestHandleInterval(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleInterval(ctx, 100, "1")
		reply := api.lastText()
		if reply == "" {
			t.Fatal("expected reply")
		}
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleInterval(ctx, 100, "999 30")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleInterval(ctx, 100, "1 60")
		requireContains(t, api.lastText(), "interval set to 60 min")

		feed, _ := store.GetFeed(ctx, 1)
		if diff := cmp.Diff(60, feed.IntervalMinutes); diff != "" {
			t.Errorf("interval (-want +got):\n%s", diff)
		}
	})
}

func TestHandlePause(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handlePause(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /pause")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handlePause(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handlePause(ctx, 100, "1")
		requireContains(t, api.lastText(), "paused")

		feed, _ := store.GetFeed(ctx, 1)
		if diff := cmp.Diff(false, feed.IsActive); diff != "" {
			t.Errorf("IsActive (-want +got):\n%s", diff)
		}
	})
}

func TestHandleResume(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleResume(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /resume")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleResume(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f := seedFeed(t, store, 100, "Feed", "https://x.com")
		f.IsActive = false
		_ = store.UpdateFeed(ctx, f)

		b.handleResume(ctx, 100, "1")
		requireContains(t, api.lastText(), "resumed")

		feed, _ := store.GetFeed(ctx, 1)
		if diff := cmp.Diff(true, feed.IsActive); diff != "" {
			t.Errorf("IsActive (-want +got):\n%s", diff)
		}
	})
}

func TestHandleCheck(t *testing.T) {
	xml := loadSampleXML(t)
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, xml)
		b.handleCheck(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /check")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, xml)
		b.handleCheck(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("fetch error", func(t *testing.T) {
		b, api, store := newTestBot(t, "broken xml")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleCheck(ctx, 100, "1")
		requireContains(t, api.lastText(), "Failed to fetch")
	})

	t.Run("no new items all seen", func(t *testing.T) {
		b, api, store := newTestBot(t, xml)
		f := seedFeed(t, store, 100, "Feed", "https://x.com")
		for _, guid := range []string{"item-1", "item-2", "item-3", "item-4", "item-5"} {
			_ = store.MarkSeen(ctx, f.ID, guid)
		}
		b.handleCheck(ctx, 100, "1")
		requireContains(t, api.lastText(), "No new matching items")
	})

	t.Run("with new items", func(t *testing.T) {
		b, api, store := newTestBot(t, xml)
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleCheck(ctx, 100, "1")

		texts := api.allTexts()
		// 5 items + 1 summary
		if diff := cmp.Diff(6, len(texts)); diff != "" {
			t.Errorf("reply count (-want +got):\n%s", diff)
		}
		requireContains(t, texts[len(texts)-1], "Found 5 new item(s)")
	})

	t.Run("with include filter", func(t *testing.T) {
		b, api, store := newTestBot(t, xml)
		f := seedFeed(t, store, 100, "Feed", "https://x.com")
		seedFilter(t, store, f.ID, model.FilterInclude, "docker")
		b.handleCheck(ctx, 100, "1")

		texts := api.allTexts()
		// 1 matching item + 1 summary
		if diff := cmp.Diff(2, len(texts)); diff != "" {
			t.Errorf("reply count (-want +got):\n%s", diff)
		}
		requireContains(t, texts[0], "Docker Desktop")
	})
}

func TestHandleFilters(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleFilters(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /filters")
	})

	t.Run("not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleFilters(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success with filters", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f := seedFeed(t, store, 100, "My Feed", "https://x.com")
		seedFilter(t, store, f.ID, model.FilterInclude, "k8s")
		seedFilter(t, store, f.ID, model.FilterExclude, "spam")

		b.handleFilters(ctx, 100, "1")
		reply := api.lastText()
		requireContains(t, reply, "Filters for #1")
		requireContains(t, reply, "k8s")
		requireContains(t, reply, "spam")
	})
}

func TestHandleAddFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleAddFilter(ctx, 100, "", "include")
		reply := api.lastText()
		if reply == "" {
			t.Fatal("expected error reply")
		}
	})

	t.Run("feed not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleAddFilter(ctx, 100, "999 word", "include")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("wrong chat", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 200, "Other", "https://x.com")
		b.handleAddFilter(ctx, 100, "1 word", "include")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("invalid regex", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleAddFilter(ctx, 100, "1 [invalid", "include_re")
		requireContains(t, api.lastText(), "Invalid regex")
	})

	t.Run("success include", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleAddFilter(ctx, 100, "1 kubernetes", "include")
		requireContains(t, api.lastText(), "Filter F1 added")
		requireContains(t, api.lastText(), "include kubernetes")

		filters, _ := store.ListFilters(ctx, 1)
		if diff := cmp.Diff(1, len(filters)); diff != "" {
			t.Errorf("filter count (-want +got):\n%s", diff)
		}
	})

	t.Run("success with scope", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleAddFilter(ctx, 100, "1 -s title deploy", "exclude")
		requireContains(t, api.lastText(), "Filter F1 added")
		requireContains(t, api.lastText(), "title only")

		filters, _ := store.ListFilters(ctx, 1)
		if diff := cmp.Diff(model.ScopeTitle, filters[0].Scope); diff != "" {
			t.Errorf("scope (-want +got):\n%s", diff)
		}
	})

	t.Run("success regex", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		b.handleAddFilter(ctx, 100, `1 (?i)release`, "include_re")
		requireContains(t, api.lastText(), "Filter F1 added")

		filters, _ := store.ListFilters(ctx, 1)
		if diff := cmp.Diff(model.FilterIncludeRe, filters[0].Kind); diff != "" {
			t.Errorf("kind (-want +got):\n%s", diff)
		}
	})
}

func TestHandleRmFilter(t *testing.T) {
	ctx := context.Background()

	t.Run("bad args", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRmFilter(ctx, 100, "")
		requireContains(t, api.lastText(), "Usage: /rmfilter")
	})

	t.Run("filter not found", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleRmFilter(ctx, 100, "999")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("wrong chat", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f := seedFeed(t, store, 200, "Other", "https://x.com")
		seedFilter(t, store, f.ID, model.FilterInclude, "word")
		b.handleRmFilter(ctx, 100, "1")
		requireContains(t, api.lastText(), "not found")
	})

	t.Run("success", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f := seedFeed(t, store, 100, "Feed", "https://x.com")
		seedFilter(t, store, f.ID, model.FilterInclude, "k8s")
		b.handleRmFilter(ctx, 100, "1")
		requireContains(t, api.lastText(), "Filter F1 removed")

		filters, _ := store.ListFilters(ctx, f.ID)
		if diff := cmp.Diff(0, len(filters)); diff != "" {
			t.Errorf("filters should be empty (-want +got):\n%s", diff)
		}
	})
}

func TestHandleCommand(t *testing.T) {
	ctx := context.Background()

	makeMsg := func(cmd, args string) *tgbotapi.Message {
		text := "/" + cmd
		if args != "" {
			text += " " + args
		}
		return &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 100},
			Text: text,
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: len("/" + cmd)},
			},
		}
	}

	t.Run("dispatches known commands", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")

		cmds := []struct {
			cmd      string
			contains string
		}{
			{"start", "Welcome"},
			{"help", "/add"},
			{"unknown_cmd", "Unknown command"},
		}

		for _, tc := range cmds {
			api.reset()
			b.handleCommand(ctx, makeMsg(tc.cmd, ""))
			requireContains(t, api.lastText(), tc.contains)
		}
	})

	t.Run("dispatches list", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		b.handleCommand(ctx, makeMsg("list", ""))
		requireContains(t, api.lastText(), "no feeds")
	})

	t.Run("dispatches filter commands", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")

		cases := []struct {
			cmd  string
			args string
		}{
			{"include", "1 word"},
			{"exclude", "1 spam"},
			{"include_re", "1 (?i)go"},
			{"exclude_re", "1 (?i)ad"},
		}
		for _, tc := range cases {
			api.reset()
			b.handleCommand(ctx, makeMsg(tc.cmd, tc.args))
			requireContains(t, api.lastText(), "Filter F")
		}
	})
}

func TestHandleCallback(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid data format", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb1",
			Data:    "nocolon",
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		if diff := cmp.Diff(0, len(api.allTexts())); diff != "" {
			t.Errorf("expected no text messages (-want +got):\n%s", diff)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		b, api, _ := newTestBot(t, "")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb2",
			Data:    "filters:abc",
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		if diff := cmp.Diff(0, len(api.allTexts())); diff != "" {
			t.Errorf("expected no text messages (-want +got):\n%s", diff)
		}
	})

	t.Run("filters callback", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb3",
			Data:    fmt.Sprintf("filters:%d", 1),
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		requireContains(t, api.lastText(), "No filters for #1")
	})

	t.Run("delete callback", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb4",
			Data:    "delete:1",
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		requireContains(t, api.lastText(), "deleted")
	})

	t.Run("delete_confirm callback", func(t *testing.T) {
		b, _, store := newTestBot(t, "")
		seedFeed(t, store, 100, "Feed", "https://x.com")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb5",
			Data:    "delete_confirm:1",
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		// delete_confirm sends a MessageConfig with inline keyboard, captured by mockAPI
	})

	t.Run("rmfilter callback", func(t *testing.T) {
		b, api, store := newTestBot(t, "")
		f := seedFeed(t, store, 100, "Feed", "https://x.com")
		seedFilter(t, store, f.ID, model.FilterInclude, "go")
		cb := &tgbotapi.CallbackQuery{
			ID:      "cb6",
			Data:    "rmfilter:1",
			Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 100}},
		}
		b.handleCallback(ctx, cb)
		requireContains(t, api.lastText(), "Filter F1 removed")
	})
}
