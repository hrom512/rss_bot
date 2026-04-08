package bot

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"rss_bot/internal/fetcher"
	"rss_bot/internal/model"
	"rss_bot/internal/text"
)

const (
	statusActive     = "active"
	statusPaused     = "paused"
	callbackShowMore = "show_more"
)

// NotificationWithKeyboard holds a formatted notification and its optional keyboard.
type NotificationWithKeyboard struct {
	Text     string
	ImageURL string
	Markup   *tgbotapi.InlineKeyboardMarkup
}

// FormatNotificationShort formats a shortened notification with a "Show more" button.
func FormatNotificationShort(feedID int64, feedName string, item fetcher.MatchedItem) NotificationWithKeyboard {
	formatted := text.FormatNotificationShort(feedID, feedName, item)

	var markup *tgbotapi.InlineKeyboardMarkup
	if formatted.Truncated {
		row := tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Show more", fmt.Sprintf("%s:%d:%s", callbackShowMore, feedID, item.GUID)),
		)
		markup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{row}}
	}

	return NotificationWithKeyboard{
		Text:     formatted.Text,
		ImageURL: formatted.ImageURL,
		Markup:   markup,
	}
}

// FormatNotification formats an RSS item as a Telegram notification message.
func FormatNotification(feedName string, item fetcher.MatchedItem) string {
	return text.FormatNotification(feedName, item)
}

// FormatNotificationFull formats a full notification without truncation.
func FormatNotificationFull(feedName string, item fetcher.MatchedItem) string {
	return text.FormatNotificationFull(feedName, item)
}

// FormatFeedList formats a list of feeds for display.
func FormatFeedList(feeds []model.Feed, filterCounts map[int64][2]int) string {
	if len(feeds) == 0 {
		return "You have no feeds yet. Use /add <url> to add one."
	}
	var b strings.Builder
	b.WriteString("Your feeds:\n")
	for _, f := range feeds {
		status := statusActive
		if !f.IsActive {
			status = statusPaused
		}
		fmt.Fprintf(&b, "\n#%d %s  (every %d min) [%s]\n", f.ID, f.Name, f.IntervalMinutes, status)
		inc, exc := filterCounts[f.ID][0], filterCounts[f.ID][1]
		if inc == 0 && exc == 0 {
			b.WriteString("   no filters\n")
		} else {
			fmt.Fprintf(&b, "   %d include, %d exclude filters\n", inc, exc)
		}
	}
	return b.String()
}

// FormatFeedInfo formats detailed information about a single feed.
func FormatFeedInfo(feed *model.Feed, filters []model.Filter) string {
	var b strings.Builder
	status := statusActive
	if !feed.IsActive {
		status = statusPaused
	}
	fmt.Fprintf(&b, "#%d %s [%s]\n", feed.ID, feed.Name, status)
	fmt.Fprintf(&b, "URL: %s\n", feed.URL)
	fmt.Fprintf(&b, "Interval: every %d min\n", feed.IntervalMinutes)
	if feed.LastCheckAt != nil {
		fmt.Fprintf(&b, "Last check: %s\n", feed.LastCheckAt.Format("2006-01-02 15:04 UTC"))
	}
	b.WriteString("\n")
	b.WriteString(FormatFilterList(feed, filters))
	return b.String()
}

// FormatFilterList formats the filter rules of a feed grouped by kind.
func FormatFilterList(feed *model.Feed, filters []model.Filter) string {
	if len(filters) == 0 {
		return fmt.Sprintf("No filters for #%d \"%s\".\nUse /include, /exclude, /include_re, /exclude_re to add filters.", feed.ID, feed.Name)
	}

	groups := map[string][]model.Filter{
		"Include (word)":  {},
		"Include (regex)": {},
		"Exclude (word)":  {},
		"Exclude (regex)": {},
	}
	for _, f := range filters {
		switch f.Kind {
		case model.FilterInclude:
			groups["Include (word)"] = append(groups["Include (word)"], f)
		case model.FilterIncludeRe:
			groups["Include (regex)"] = append(groups["Include (regex)"], f)
		case model.FilterExclude:
			groups["Exclude (word)"] = append(groups["Exclude (word)"], f)
		case model.FilterExcludeRe:
			groups["Exclude (regex)"] = append(groups["Exclude (regex)"], f)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Filters for #%d \"%s\":\n", feed.ID, feed.Name)

	order := []string{"Include (word)", "Include (regex)", "Exclude (word)", "Exclude (regex)"}
	for _, groupName := range order {
		fs := groups[groupName]
		if len(fs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s:\n", groupName)
		for _, f := range fs {
			fmt.Fprintf(&b, "  F%d: %s (%s)\n", f.ID, f.Value, scopeLabel(f.Scope))
		}
	}
	return b.String()
}

func scopeLabel(s model.FilterScope) string {
	switch s {
	case model.ScopeTitle:
		return "title only"
	case model.ScopeContent:
		return "content only"
	default:
		return "title+content"
	}
}
