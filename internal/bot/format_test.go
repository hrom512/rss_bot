package bot

import (
	"strings"
	"testing"

	"rss_bot/internal/fetcher"
)

const testFeedName = "Test Feed"

func TestFormatNotification_WithHTML(t *testing.T) {
	feedName := testFeedName
	item := fetcher.MatchedItem{
		Title:       "Test Title",
		Content:     "<p>This is <strong>HTML</strong> content with <a href=\"http://example.com\">a link</a>.</p>",
		Description: "Plain description",
		Link:        "https://example.com/article",
	}

	result := FormatNotification(feedName, item)

	if result == "" {
		t.Fatal("expected non-empty result")
	}

	if !strings.Contains(result, "[Test Feed]") {
		t.Error("should contain feed name")
	}
	if !strings.Contains(result, "Test Title") {
		t.Error("should contain title")
	}
	if !strings.Contains(result, "HTML content") {
		t.Error("should contain parsed HTML content")
	}
	if !strings.Contains(result, "a link") {
		t.Error("should contain link text")
	}
	if !strings.Contains(result, "example.com/article") {
		t.Error("should contain link")
	}
}

func TestFormatNotification_PlainText(t *testing.T) {
	feedName := testFeedName
	item := fetcher.MatchedItem{
		Title:       "Plain Title",
		Content:     "",
		Description: "Just plain text description.",
		Link:        "https://example.com",
	}

	result := FormatNotification(feedName, item)

	if !strings.Contains(result, "Just plain text description.") {
		t.Errorf("should preserve plain text, got: %s", result)
	}
}

func TestFormatNotificationShort_WithImage(t *testing.T) {
	feedName := testFeedName
	feedID := int64(1)
	item := fetcher.MatchedItem{
		Title:       "Title with Image",
		Description: "<p>Description with image: <img src=\"https://example.com/photo.jpg\"/></p>",
		ImageURL:    "https://example.com/enclosure.jpg",
		Link:        "https://example.com",
		GUID:        "item-123",
	}

	result := FormatNotificationShort(feedID, feedName, item)

	if result.ImageURL == "" {
		t.Error("should have ImageURL")
	}
	if !strings.Contains(result.Text, "Title with Image") {
		t.Error("should contain title")
	}
}
