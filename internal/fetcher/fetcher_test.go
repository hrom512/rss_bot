package fetcher

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mmcdole/gofeed"

	"rss_bot/internal/model"
)

type mockTransport struct {
	body       string
	statusCode int
	err        error
}

func (m *mockTransport) Do(_ *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
	}, nil
}

func loadFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test-only fixture loading
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(data)
}

func TestFetch(t *testing.T) {
	xml := loadFixture(t, "../../testdata/sample.xml")

	tests := []struct {
		name      string
		transport *mockTransport
		wantTitle string
		wantItems int
		wantErr   bool
	}{
		{
			name:      "successful fetch",
			transport: &mockTransport{body: xml, statusCode: 200},
			wantTitle: "DevOps Weekly",
			wantItems: 5,
		},
		{
			name:      "http error status",
			transport: &mockTransport{body: "not found", statusCode: 404},
			wantErr:   true,
		},
		{
			name:      "network error",
			transport: &mockTransport{err: io.ErrUnexpectedEOF},
			wantErr:   true,
		},
		{
			name:      "invalid xml",
			transport: &mockTransport{body: "not xml at all", statusCode: 200},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New(tt.transport)
			feed, err := f.Fetch(context.Background(), "https://example.com/rss")

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if diff := cmp.Diff(tt.wantTitle, feed.Title); diff != "" {
				t.Errorf("title mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantItems, len(feed.Items)); diff != "" {
				t.Errorf("item count mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestItemGUID(t *testing.T) {
	tests := []struct {
		name     string
		item     *gofeed.Item
		wantGUID string
		hasHash  bool
	}{
		{
			name:     "with guid",
			item:     &gofeed.Item{GUID: "abc-123"},
			wantGUID: "abc-123",
		},
		{
			name:    "without guid generates hash",
			item:    &gofeed.Item{Title: "Post Without GUID", Link: "https://example.com/post-1"},
			hasHash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ItemGUID(tt.item)
			if tt.hasHash {
				if !strings.HasPrefix(got, "sha256:") {
					t.Errorf("expected sha256 prefix, got %q", got)
				}
				return
			}
			if diff := cmp.Diff(tt.wantGUID, got); diff != "" {
				t.Errorf("GUID mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFilterItems(t *testing.T) {
	xml := loadFixture(t, "../../testdata/sample.xml")
	parser := gofeed.NewParser()
	feed, err := parser.ParseString(xml)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	tests := []struct {
		name       string
		filters    []model.Filter
		wantTitles []string
	}{
		{
			name:    "no filters returns all",
			filters: nil,
			wantTitles: []string{
				"Kubernetes 1.32 Released",
				"Docker Desktop Update",
				"DevOps Job Vacancy at BigCorp",
				"Helm Chart Best Practices",
				"Online Course: K8s Training for Beginners",
			},
		},
		{
			name: "include kubernetes",
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			wantTitles: []string{
				"Kubernetes 1.32 Released",
				"Helm Chart Best Practices",
				"Online Course: K8s Training for Beginners",
			},
		},
		{
			name: "include kubernetes, exclude vacancy and course",
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
				{Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
				{Kind: model.FilterExcludeRe, Scope: model.ScopeAll, Value: "course.*training"},
			},
			wantTitles: []string{
				"Kubernetes 1.32 Released",
				"Helm Chart Best Practices",
			},
		},
		{
			name: "include by title scope only",
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeTitle, Value: "helm"},
			},
			wantTitles: []string{
				"Helm Chart Best Practices",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := FilterItems(feed.Items, tt.filters)
			var gotTitles []string
			for _, m := range matched {
				gotTitles = append(gotTitles, m.Title)
			}
			if diff := cmp.Diff(tt.wantTitles, gotTitles); diff != "" {
				t.Errorf("titles mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
