// Package fetcher handles RSS feed downloading, parsing, and filtering.
package fetcher

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"rss_bot/internal/filter"
	"rss_bot/internal/model"
)

// HTTPClient is the interface for performing HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Result holds the outcome of fetching and filtering an RSS feed.
type Result struct {
	Items []MatchedItem
	Title string
}

// MatchedItem represents a single RSS item that passed filtering.
type MatchedItem struct {
	Title       string
	Description string
	Content     string
	Link        string
	GUID        string
	ImageURL    string
}

// Fetcher downloads and parses RSS feeds.
type Fetcher struct {
	client  HTTPClient
	timeout time.Duration
}

// New creates a Fetcher with the given HTTP client.
func New(client HTTPClient) *Fetcher {
	return &Fetcher{
		client:  client,
		timeout: 30 * time.Second,
	}
}

// Fetch downloads and parses an RSS feed from the given URL.
func (f *Fetcher) Fetch(ctx context.Context, url string) (*gofeed.Feed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "RSSNotifyBot/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	parser := gofeed.NewParser()
	feed, err := parser.ParseString(string(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed: %w", err)
	}
	return feed, nil
}

// ItemGUID returns the GUID for an RSS item.
// If the item has no GUID, a SHA-256 hash of title+link is used.
func ItemGUID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	h := sha256.Sum256([]byte(item.Title + "|" + item.Link))
	return fmt.Sprintf("sha256:%x", h[:16])
}

// FilterItems applies filters to RSS items and returns those that match.
func FilterItems(items []*gofeed.Item, filters []model.Filter) []MatchedItem {
	var matched []MatchedItem
	for _, item := range items {
		fi := filter.FeedItem{
			Title:       item.Title,
			Description: item.Description,
		}
		if filter.Match(fi, filters) {
			mi := MatchedItem{
				Title:       item.Title,
				Description: item.Description,
				Content:     item.Content,
				Link:        item.Link,
				GUID:        ItemGUID(item),
				ImageURL:    extractImageURL(item),
			}
			matched = append(matched, mi)
		}
	}
	return matched
}

func extractImageURL(item *gofeed.Item) string {
	for _, enc := range item.Enclosures {
		if enc.URL != "" && strings.HasPrefix(enc.Type, "image/") {
			return enc.URL
		}
	}

	if item.Image != nil && item.Image.URL != "" {
		return item.Image.URL
	}

	return ""
}
