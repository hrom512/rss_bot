package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	cmdCheck    = "check"
	cmdFilters  = "filters"
	cmdRmFilter = "rmfilter"
	cmdShowMore = "show_more"
)

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	data := cb.Data
	chatID := cb.Message.Chat.ID

	callback := tgbotapi.NewCallback(cb.ID, "")
	if _, err := b.api.Send(callback); err != nil {
		b.log.Error("send callback ack", "error", err)
	}

	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return
	}

	action := parts[0]
	idStr := parts[1]

	b.log.Info("callback",
		"action", action,
		"id", idStr,
		"chat_id", chatID,
		"user_id", cb.From.ID,
		"username", cb.From.UserName,
	)

	if action != cmdShowMore {
		if _, err := strconv.ParseInt(idStr, 10, 64); err != nil {
			return
		}
	}

	switch action {
	case cmdShowMore:
		b.handleShowMore(ctx, chatID, idStr)
	case cmdFilters:
		b.handleFilters(ctx, chatID, idStr)
	case cmdCheck:
		b.handleCheck(ctx, chatID, idStr)
	case "delete_confirm":
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		feed, err := b.store.GetFeed(ctx, id)
		if err != nil || feed.ChatID != chatID {
			b.reply(chatID, fmt.Sprintf("Feed #%d not found.", id))
			return
		}
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Delete #%d \"%s\"? This cannot be undone.", id, feed.Name))
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Yes, delete", fmt.Sprintf("delete:%d", id)),
				tgbotapi.NewInlineKeyboardButtonData("Cancel", "noop:0"),
			),
		)
		if _, err := b.api.Send(msg); err != nil {
			b.log.Error("send delete confirmation", "error", err)
		}
	case "delete":
		b.handleRemove(ctx, chatID, idStr)
	case cmdRmFilter:
		b.handleRmFilter(ctx, chatID, idStr)
	}
}

func (b *Bot) handleShowMore(ctx context.Context, chatID int64, data string) {
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		b.reply(chatID, "Invalid request.")
		return
	}

	feedIDStr, guid := parts[0], parts[1]
	feedID, err := strconv.ParseInt(feedIDStr, 10, 64)
	if err != nil {
		b.reply(chatID, "Invalid feed ID.")
		return
	}

	feed, err := b.store.GetFeed(ctx, feedID)
	if err != nil || feed.ChatID != chatID {
		b.reply(chatID, "Feed not found.")
		return
	}

	content, err := b.store.GetFullContent(ctx, feedID, guid)
	if err != nil {
		b.reply(chatID, "Could not retrieve content.")
		return
	}

	if content == "" {
		b.reply(chatID, "Full content not available.")
		return
	}

	processed := content
	if IsHTML(content) {
		processed = ParseHTMLToPlain(content).Text
	}

	fullMsg := FormatNotificationFull(feed.Name, struct {
		Title       string
		Description string
		Content     string
		Link        string
		GUID        string
		ImageURL    string
	}{
		Title:       feed.Name,
		Description: processed,
		Content:     "",
		Link:        "",
		GUID:        guid,
		ImageURL:    "",
	})
	b.reply(chatID, fullMsg)
}
