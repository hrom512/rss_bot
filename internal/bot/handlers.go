package bot

import (
	"context"
	"fmt"

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

	b.reply(chatID, fmt.Sprintf("Feed added successfully!\n#%d %s (every %d min)\nURL: %s\nNo filters yet. Use /include, /exclude to add filters.",
		f.ID, f.Name, f.IntervalMinutes, f.URL))
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
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /info <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}
	if feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	b.reply(chatID, FormatFeedInfo(feed, filters))
}

func (b *Bot) handleRemove(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /remove <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	if err := b.store.DeleteFeed(ctx, id); err != nil {
		b.reply(chatID, fmt.Sprintf("Error deleting feed: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" deleted.", id, feed.Name))
}

func (b *Bot) handleRename(ctx context.Context, chatID int64, args string) {
	id, name, err := ParseRenameArgs(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	feed.Name = name
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d renamed to \"%s\".", id, name))
}

func (b *Bot) handleInterval(ctx context.Context, chatID int64, args string) {
	id, mins, err := ParseIntervalArgs(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	feed.IntervalMinutes = mins
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d interval set to %d min.", id, mins))
}

func (b *Bot) handlePause(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /pause <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	feed.IsActive = false
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" paused.", id, feed.Name))
}

func (b *Bot) handleResume(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /resume <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	feed.IsActive = true
	if err := b.store.UpdateFeed(ctx, feed); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Feed #%d \"%s\" resumed.", id, feed.Name))
}

func (b *Bot) handleCheck(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /check <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
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

	if len(newItems) == 0 {
		b.reply(chatID, fmt.Sprintf("No new matching items in #%d \"%s\".", feed.ID, feed.Name))
		return
	}

	for _, item := range newItems {
		b.reply(chatID, FormatNotification(feed.Name, item))
		_ = b.store.MarkSeen(ctx, feed.ID, item.GUID)
	}
	b.reply(chatID, fmt.Sprintf("Found %d new item(s) in #%d \"%s\".", len(newItems), feed.ID, feed.Name))
}

func (b *Bot) handleFilters(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /filters <id>")
		return
	}

	feed, err := b.store.GetFeed(ctx, id)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
		return
	}

	filters, _ := b.store.ListFilters(ctx, feed.ID)
	b.reply(chatID, FormatFilterList(feed, filters))
}

func (b *Bot) handleAddFilter(ctx context.Context, chatID int64, args string, kind string) {
	parsed, err := ParseFilterCommand(args)
	if err != nil {
		b.reply(chatID, err.Error())
		return
	}

	feed, err := b.store.GetFeed(ctx, parsed.FeedID)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Feed #%d not found.", parsed.FeedID))
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
		FeedID: parsed.FeedID,
		Kind:   fk,
		Scope:  parsed.Scope,
		Value:  parsed.Value,
	}
	if err := b.store.CreateFilter(ctx, f); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}

	b.reply(chatID, fmt.Sprintf("Filter F%d added to #%d \"%s\": %s %s (%s)",
		f.ID, feed.ID, feed.Name, kind, parsed.Value, scopeLabel(parsed.Scope)))
}

func (b *Bot) handleRmFilter(ctx context.Context, chatID int64, args string) {
	id, err := ParseIDArg(args)
	if err != nil {
		b.reply(chatID, "Usage: /rmfilter <filter_id>")
		return
	}

	f, err := b.store.GetFilter(ctx, id)
	if err != nil {
		b.reply(chatID, fmt.Sprintf("Filter F%d not found.", id))
		return
	}

	feed, err := b.store.GetFeed(ctx, f.FeedID)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, fmt.Sprintf("Filter F%d not found.", id))
		return
	}

	if err := b.store.DeleteFilter(ctx, id); err != nil {
		b.reply(chatID, fmt.Sprintf("Error: %v", err))
		return
	}
	b.reply(chatID, fmt.Sprintf("Filter F%d removed from #%d \"%s\".", id, feed.ID, feed.Name))
}
