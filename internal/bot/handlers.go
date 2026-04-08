package bot

import (
	"context"
	"fmt"
	"time"

	"rss_bot/internal/fetcher"
	"rss_bot/internal/filter"
	"rss_bot/internal/model"
)

func (b *Bot) handleStart(chatID int64) {
	b.reply(chatID, `Welcome to RSS Notify Bot!

Subscribe to RSS feeds and get filtered notifications.

Quick start:
1. /add <url> — add an RSS feed
2. /include <id> <word> — add a whitelist filter
3. /exclude <id> <word> — add a blacklist filter

Use /help for the full command reference.`)
}

func (b *Bot) handleHelp(chatID int64) {
	b.reply(chatID, `Feed management:
/add <url> — add a new RSS feed
/list — show all feeds
/info <id> — feed details
/remove <id> — delete a feed
/rename <id> <name> — rename a feed
/interval <id> <min> — set check interval (1-1440)
/pause <id> — pause checking
/resume <id> — resume checking
/check <id> — force check now

Filter management:
/filters <id> — show filters for a feed
/include <id> [-s scope] <word> — whitelist word/phrase
/exclude <id> [-s scope] <word> — blacklist word/phrase
/include_re <id> [-s scope] <regex> — whitelist regex
/exclude_re <id> [-s scope] <regex> — blacklist regex
/rmfilter <filter_id> — remove a filter

Scope flag: -s title | content | all (default: all)`)
}

func (b *Bot) handleAdd(ctx context.Context, chatID int64, args string) {
	if args == "" {
		b.reply(chatID, "Usage: /add <url>")
		return
	}

	feed, err := b.fetcher.Fetch(ctx, args)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to fetch feed: %v", err))
		return
	}

	name := feed.Title
	if name == "" {
		name = args
	}

	f := &model.Feed{
		ChatID:          chatID,
		Name:            name,
		URL:             args,
		IntervalMinutes: 15,
		IsActive:        true,
	}
	if err := b.store.CreateFeed(ctx, f); err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to save feed: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf(`Feed added successfully!

#%d %s (every %d min)
URL: %s
No filters yet. Use /include, /exclude to add filters.`,
		f.Position, f.Name, f.IntervalMinutes, f.URL))
}

func (b *Bot) handleList(ctx context.Context, chatID int64) {
	feeds, err := b.store.ListFeeds(ctx, chatID)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	counts := make(map[int64][2]int)
	for _, f := range feeds {
		filters, err := b.store.ListFilters(ctx, f.ID)
		if err != nil {
			continue
		}
		var inc, exc int
		for _, fl := range filters {
			switch fl.Kind {
			case model.FilterInclude, model.FilterIncludeRe:
				inc++
			case model.FilterExclude, model.FilterExcludeRe:
				exc++
			}
		}
		counts[f.ID] = [2]int{inc, exc}
	}

	b.reply(chatID, FormatFeedList(feeds, counts))
}

func (b *Bot) handleInfo(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /info <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	b.reply(chatID, FormatFeedInfo(feed, filters))
}

func (b *Bot) handleRemove(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /remove <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	if err := b.store.DeleteFeed(ctx, feed.ID); err != nil {
		b.reply(chatID, fmt.Sprintf("Error deleting feed: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" deleted.", pos, feed.Name))
}

func (b *Bot) handleRename(ctx context.Context, chatID int64, args string) {
	pos, name, err := ParseRenameArgs(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	feed.Name = name
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d renamed to \"%s\".", pos, name))
}

func (b *Bot) handleInterval(ctx context.Context, chatID int64, args string) {
	pos, mins, err := ParseIntervalArgs(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	feed.IntervalMinutes = mins
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d interval set to %d min.", pos, mins))
}

func (b *Bot) handlePause(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /pause <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	feed.IsActive = false
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" paused.", pos, feed.Name))
}

func (b *Bot) handleResume(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /resume <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	feed.IsActive = true
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" resumed.", pos, feed.Name))
}

func (b *Bot) handleCheck(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /check <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	rssFeed, err := b.fetcher.Fetch(ctx, feed.URL)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Failed to fetch: %v", err))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	matched := fetcher.FilterItems(rssFeed.Items, filters)

	var newItems []fetcher.MatchedItem
	for _, item := range matched {
		seen, _ := b.store.IsSeen(ctx, feed.ID, item.GUID)
		if !seen {
			newItems = append(newItems, item)
		}
	}

	b.log.Info("manual check",
		"feed_id", feed.ID,
		"name", feed.Name,
		"total_items", len(rssFeed.Items),
		"matched", len(matched),
		"new", len(newItems),
		"chat_id", chatID,
	)

	if len(newItems) == 0 {
		b.reply(chatID, fmt.Sprintf("No new matching items in #%d \"%s\".", pos, feed.Name))
		return
	}

	for _, item := range newItems {
		msg := FormatNotificationShort(feed.Position, feed.Name, item)
		if msg.ImageURL != "" {
			b.SendPhotoWithCaption(chatID, msg.ImageURL, msg.Text, msg.Markup)
		} else if msg.Markup != nil {
			b.SendMessageWithKeyboard(chatID, msg.Text, msg.Markup)
		} else {
			b.reply(chatID, msg.Text)
		}
		_ = b.store.MarkSeen(ctx, feed.ID, item.GUID, item.Description)
	}
	now := time.Now()
	feed.LastCheckAt = &now
	b.store.UpdateFeed(ctx, feed)
	b.reply(chatID, fmt.Sprintf("Found %d new item(s) in #%d \"%s\".", len(newItems), pos, feed.Name))
}

func (b *Bot) handleFilters(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFeedArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /filters <number>")
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, pos)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", pos))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	b.reply(chatID, fmt.Sprintf("Filters for #%d \"%s\":\n\n%s", feed.Position, feed.Name, FormatFilterList(feed, filters)))
}

func (b *Bot) handleAddFilter(ctx context.Context, chatID int64, args string, kind string) {
	parsed, err := ParseFilterCommand(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeedByPosition(ctx, chatID, parsed.FeedPosition)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", parsed.FeedPosition))
		return
	}

	fk := model.FilterKind(kind)
	if fk == model.FilterIncludeRe || fk == model.FilterExcludeRe {
		if err := filter.ValidateRegex(parsed.Value); err != nil {
			b.reply(chatID, fmt.Sprintf("Invalid regex: %v", err))
			return
		}
	}

	f := &model.Filter{
		FeedID: feed.ID,
		Kind:   fk,
		Scope:  parsed.Scope,
		Value:  parsed.Value,
	}
	if err := b.store.CreateFilter(ctx, f); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	b.reply(chatID, fmt.Sprintf("Filter F%d added to #%d \"%s\".\n\nActual filters:\n\n%s", f.Position, feed.Position, feed.Name, FormatFilterList(feed, filters)))
}

func (b *Bot) handleRmFilter(ctx context.Context, chatID int64, args string) {
	pos, err := ParseFilterArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /rmfilter <filter_number>")
		return
	}

	feeds, err := b.store.ListFeeds(ctx, chatID)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	var targetFilter *model.Filter
	var targetFeed *model.Feed

	for i := range feeds {
		filters, err := b.store.ListFilters(ctx, feeds[i].ID)
		if err != nil {
			continue
		}
		for j := range filters {
			if filters[j].Position == pos {
				f := filters[j]
				targetFilter = &f
				fp := feeds[i]
				targetFeed = &fp
				break
			}
		}
		if targetFilter != nil {
			break
		}
	}

	if targetFilter == nil || targetFeed == nil {
		b.reply(chatID, fmt.Sprintf("Filter F%d not found.", pos))
		return
	}

	if err := b.store.DeleteFilter(ctx, targetFilter.ID); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	remaining, _ := b.store.ListFilters(ctx, targetFeed.ID)
	if len(remaining) > 0 {
		b.reply(chatID, fmt.Sprintf("Filter F%d removed from #%d \"%s\".\n\nActual filters:\n\n%s", pos, targetFeed.Position, targetFeed.Name, FormatFilterList(targetFeed, remaining)))
	} else {
		b.reply(chatID, fmt.Sprintf("Filter F%d removed from #%d \"%s\".\n\n%s", pos, targetFeed.Position, targetFeed.Name, FormatFilterList(targetFeed, remaining)))
	}
}
