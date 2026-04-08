package text

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"rss_bot/internal/fetcher"
)

func TestGetItemText(t *testing.T) {
	tests := []struct {
		name string
		item fetcher.MatchedItem
		want string
	}{
		{
			name: "content takes priority over description",
			item: fetcher.MatchedItem{
				Content:     "Full content here",
				Description: "Short description",
			},
			want: "Full content here",
		},
		{
			name: "uses description when content is empty",
			item: fetcher.MatchedItem{
				Content:     "",
				Description: "Plain description",
			},
			want: "Plain description",
		},
		{
			name: "parses HTML content",
			item: fetcher.MatchedItem{
				Content:     "<p>HTML <strong>content</strong></p>",
				Description: "",
			},
			want: "HTML content",
		},
		{
			name: "parses HTML description",
			item: fetcher.MatchedItem{
				Content:     "",
				Description: "<p>HTML <em>description</em></p>",
			},
			want: "HTML description",
		},
		{
			name: "empty returns empty",
			item: fetcher.MatchedItem{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getItemText(tt.item)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getItemText mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetItemImageURL(t *testing.T) {
	tests := []struct {
		name string
		item fetcher.MatchedItem
		want string
	}{
		{
			name: "returns item ImageURL directly",
			item: fetcher.MatchedItem{
				ImageURL: "https://example.com/image.jpg",
			},
			want: "https://example.com/image.jpg",
		},
		{
			name: "extracts from HTML content",
			item: fetcher.MatchedItem{
				Content:  "<p>Text<img src=\"https://example.com/content.jpg\"/></p>",
				ImageURL: "",
			},
			want: "https://example.com/content.jpg",
		},
		{
			name: "extracts from HTML description",
			item: fetcher.MatchedItem{
				Content:     "",
				Description: "<p>Text<img src=\"https://example.com/desc.jpg\"/></p>",
				ImageURL:    "",
			},
			want: "https://example.com/desc.jpg",
		},
		{
			name: "item ImageURL takes priority",
			item: fetcher.MatchedItem{
				ImageURL:    "https://example.com/direct.jpg",
				Description: "<p><img src=\"https://example.com/html.jpg\"/></p>",
			},
			want: "https://example.com/direct.jpg",
		},
		{
			name: "empty returns empty",
			item: fetcher.MatchedItem{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getItemImageURL(tt.item)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("getItemImageURL mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
