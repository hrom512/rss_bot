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
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return
	}

	b.log.Info("callback",
		"action", action,
		"id", id,
		"chat_id", chatID,
		"user_id", cb.From.ID,
		"username", cb.From.UserName,
	)

	switch action {
	case cmdFilters:
		b.handleFilters(ctx, chatID, idStr)
	case cmdCheck:
		b.handleCheck(ctx, chatID, idStr)
	case "delete_confirm":
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
