package text

import (
	"fmt"
	"strings"

	"rss_bot/internal/fetcher"
)

// FormattedContent holds formatted text and optional image URL.
type FormattedContent struct {
	Text     string
	ImageURL string
}

const maxPreviewLength = 1500

// FormatItemContent formats an RSS item content for display.
func FormatItemContent(item fetcher.MatchedItem) FormattedContent {
	var text strings.Builder

	desc := getItemText(item)
	if desc != "" {
		text.WriteString(desc)
	}

	imageURL := getItemImageURL(item)

	return FormattedContent{
		Text:     text.String(),
		ImageURL: imageURL,
	}
}

func getItemText(item fetcher.MatchedItem) string {
	if item.Content != "" {
		if IsHTML(item.Content) {
			return ParseHTMLToPlain(item.Content).Text
		}
		return item.Content
	}
	if item.Description != "" {
		if IsHTML(item.Description) {
			return ParseHTMLToPlain(item.Description).Text
		}
		return item.Description
	}
	return ""
}

func getItemImageURL(item fetcher.MatchedItem) string {
	if item.ImageURL != "" {
		return item.ImageURL
	}
	if item.Content != "" && IsHTML(item.Content) {
		return ParseHTMLToPlain(item.Content).ImageURL
	}
	if item.Description != "" && IsHTML(item.Description) {
		return ParseHTMLToPlain(item.Description).ImageURL
	}
	return ""
}

// FormatNotification formats an RSS item as a notification message.
func FormatNotification(feedName string, item fetcher.MatchedItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\n\n", feedName)
	b.WriteString(item.Title)
	desc := getItemText(item)
	if desc != "" {
		b.WriteString("\n\n")
		b.WriteString(desc)
	}
	if item.Link != "" {
		b.WriteString("\n\n")
		b.WriteString(item.Link)
	}
	return b.String()
}

// NotificationWithKeyboard holds a formatted notification and optional keyboard.
type NotificationWithKeyboard struct {
	Text      string
	ImageURL  string
	Truncated bool
}

// FormatNotificationShort formats a shortened notification with a "Show more" button.
func FormatNotificationShort(_ int64, feedName string, item fetcher.MatchedItem) NotificationWithKeyboard {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s]\n\n", feedName)
	b.WriteString(item.Title)

	desc := getItemText(item)
	truncated := false
	if len(desc) > maxPreviewLength {
		desc = desc[:maxPreviewLength]
		truncated = true
	}

	if desc != "" {
		b.WriteString("\n\n")
		b.WriteString(desc)
	}

	if item.Link != "" {
		b.WriteString("\n\n")
		b.WriteString(item.Link)
	}

	return NotificationWithKeyboard{
		Text:      b.String(),
		ImageURL:  getItemImageURL(item),
		Truncated: truncated,
	}
}

// FormatNotificationFull formats a full notification without truncation.
func FormatNotificationFull(feedName string, item fetcher.MatchedItem) string {
	return FormatNotification(feedName, item)
}
