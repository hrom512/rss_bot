package bot

import (
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"rss_bot/internal/fetcher"
	"rss_bot/internal/model"
)

func TestParseFilterCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    FilterArgs
		wantErr bool
	}{
		{
			name: "simple word",
			args: "1 kubernetes",
			want: FilterArgs{FeedID: 1, Scope: model.ScopeAll, Value: "kubernetes"},
		},
		{
			name: "multi-word value",
			args: "3 helm chart deployment",
			want: FilterArgs{FeedID: 3, Scope: model.ScopeAll, Value: "helm chart deployment"},
		},
		{
			name: "with scope title",
			args: "1 -s title deploy",
			want: FilterArgs{FeedID: 1, Scope: model.ScopeTitle, Value: "deploy"},
		},
		{
			name: "with scope content",
			args: "2 -s content promo material",
			want: FilterArgs{FeedID: 2, Scope: model.ScopeContent, Value: "promo material"},
		},
		{
			name: "with scope all",
			args: "1 -s all kubernetes",
			want: FilterArgs{FeedID: 1, Scope: model.ScopeAll, Value: "kubernetes"},
		},
		{
			name:    "missing value",
			args:    "1",
			wantErr: true,
		},
		{
			name:    "invalid id",
			args:    "abc kubernetes",
			wantErr: true,
		},
		{
			name:    "empty args",
			args:    "",
			wantErr: true,
		},
		{
			name:    "invalid scope",
			args:    "1 -s invalid word",
			wantErr: true,
		},
		{
			name:    "scope flag without value",
			args:    "1 -s title",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFilterCommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseIDArg(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		want    int64
		wantErr bool
	}{
		{name: "valid", args: "42", want: 42},
		{name: "with whitespace", args: "  7  ", want: 7},
		{name: "empty", args: "", wantErr: true},
		{name: "not a number", args: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIDArg(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseRenameArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantID   int64
		wantName string
		wantErr  bool
	}{
		{name: "valid", args: "1 New Name", wantID: 1, wantName: "New Name"},
		{name: "missing name", args: "1", wantErr: true},
		{name: "invalid id", args: "abc name", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, name, err := ParseRenameArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantID, id); diff != "" {
				t.Errorf("id mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantName, name); diff != "" {
				t.Errorf("name mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseIntervalArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		wantID   int64
		wantMins int
		wantErr  bool
	}{
		{name: "valid", args: "1 30", wantID: 1, wantMins: 30},
		{name: "min boundary", args: "2 1", wantID: 2, wantMins: 1},
		{name: "max boundary", args: "3 1440", wantID: 3, wantMins: 1440},
		{name: "too low", args: "1 0", wantErr: true},
		{name: "too high", args: "1 1441", wantErr: true},
		{name: "missing minutes", args: "1", wantErr: true},
		{name: "not a number", args: "1 abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, mins, err := ParseIntervalArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.wantID, id); diff != "" {
				t.Errorf("id mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantMins, mins); diff != "" {
				t.Errorf("minutes mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatNotification(t *testing.T) {
	tests := []struct {
		name     string
		feedName string
		item     fetcher.MatchedItem
		want     string
	}{
		{
			name:     "full item",
			feedName: "Habr DevOps",
			item: fetcher.MatchedItem{
				Title:       "K8s 1.32 Released",
				Description: "New version with sidecar support.",
				Link:        "https://example.com/article",
			},
			want: "[Habr DevOps]\n\nK8s 1.32 Released\n\nNew version with sidecar support.\n\nhttps://example.com/article",
		},
		{
			name:     "no description",
			feedName: "Feed",
			item: fetcher.MatchedItem{
				Title: "Title Only",
				Link:  "https://example.com",
			},
			want: "[Feed]\n\nTitle Only\n\nhttps://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNotification(tt.feedName, tt.item)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFormatFeedList(t *testing.T) {
	tests := []struct {
		name         string
		feeds        []model.Feed
		filterCounts map[int64][2]int
		wantContains []string
	}{
		{
			name:         "empty list",
			feeds:        nil,
			filterCounts: nil,
			wantContains: []string{"no feeds yet"},
		},
		{
			name: "with feeds",
			feeds: []model.Feed{
				{ID: 1, Name: "Feed A", IntervalMinutes: 15, IsActive: true},
				{ID: 2, Name: "Feed B", IntervalMinutes: 60, IsActive: false},
			},
			filterCounts: map[int64][2]int{
				1: {2, 1},
				2: {0, 0},
			},
			wantContains: []string{
				"#1 Feed A",
				"[active]",
				"2 include, 1 exclude",
				"#2 Feed B",
				"[paused]",
				"no filters",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFeedList(tt.feeds, tt.filterCounts)
			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatFeedInfo(t *testing.T) {
	lastCheck := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name         string
		feed         *model.Feed
		filters      []model.Filter
		wantContains []string
	}{
		{
			name: "active feed with filters",
			feed: &model.Feed{
				ID: 1, Name: "DevOps Feed", URL: "https://example.com/rss",
				IntervalMinutes: 30, IsActive: true, LastCheckAt: &lastCheck,
			},
			filters: []model.Filter{
				{ID: 10, FeedID: 1, Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "k8s"},
				{ID: 11, FeedID: 1, Kind: model.FilterExclude, Scope: model.ScopeTitle, Value: "ad"},
			},
			wantContains: []string{
				"#1 DevOps Feed [active]",
				"https://example.com/rss",
				"every 30 min",
				"2025-06-15 10:30 UTC",
				"F10: k8s (title+content)",
				"F11: ad (title only)",
			},
		},
		{
			name: "paused feed no filters",
			feed: &model.Feed{
				ID: 5, Name: "Paused", URL: "https://p.com", IntervalMinutes: 60, IsActive: false,
			},
			filters: nil,
			wantContains: []string{
				"#5 Paused [paused]",
				"No filters",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFeedInfo(tt.feed, tt.filters)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestFormatFilterList(t *testing.T) {
	feed := &model.Feed{ID: 1, Name: "Test Feed"}

	tests := []struct {
		name         string
		filters      []model.Filter
		wantContains []string
	}{
		{
			name:    "no filters",
			filters: nil,
			wantContains: []string{
				"No filters for #1",
				"/include",
			},
		},
		{
			name: "all filter kinds",
			filters: []model.Filter{
				{ID: 1, Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "go"},
				{ID: 2, Kind: model.FilterIncludeRe, Scope: model.ScopeTitle, Value: "(?i)release"},
				{ID: 3, Kind: model.FilterExclude, Scope: model.ScopeContent, Value: "spam"},
				{ID: 4, Kind: model.FilterExcludeRe, Scope: model.ScopeAll, Value: "(?i)ads"},
			},
			wantContains: []string{
				"Include (word):",
				"F1: go (title+content)",
				"Include (regex):",
				"F2: (?i)release (title only)",
				"Exclude (word):",
				"F3: spam (content only)",
				"Exclude (regex):",
				"F4: (?i)ads (title+content)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatFilterList(feed, tt.filters)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q:\n%s", want, got)
				}
			}
		})
	}
}

func TestScopeLabel(t *testing.T) {
	tests := []struct {
		scope model.FilterScope
		want  string
	}{
		{model.ScopeTitle, "title only"},
		{model.ScopeContent, "content only"},
		{model.ScopeAll, "title+content"},
		{"unknown", "title+content"},
	}

	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			got := scopeLabel(tt.scope)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("scopeLabel(%q) mismatch (-want +got):\n%s", tt.scope, diff)
			}
		})
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
