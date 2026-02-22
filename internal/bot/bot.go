package bot

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"rss_bot/internal/config"
	"rss_bot/internal/fetcher"
	"rss_bot/internal/storage"
)

type telegramAPI interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	StopReceivingUpdates()
}

// Bot is the Telegram bot that handles user commands and sends notifications.
type Bot struct {
	api     telegramAPI
	store   storage.Storage
	cfg     *config.Config
	fetcher *fetcher.Fetcher
	log     *slog.Logger
}

// New creates a Bot with the given Telegram token, storage, and config.
func New(token string, store storage.Storage, cfg *config.Config, log *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create bot api: %w", err)
	}

	return &Bot{
		api:     api,
		store:   store,
		cfg:     cfg,
		fetcher: fetcher.New(http.DefaultClient),
		log:     log,
	}, nil
}

// Run starts the bot's long-polling loop, blocking until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case update := <-updates:
			if update.CallbackQuery != nil {
				b.handleCallback(ctx, update.CallbackQuery)
				continue
			}
			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}
			if !b.cfg.IsUserAllowed(update.Message.From.ID) {
				b.reply(update.Message.Chat.ID, "Access denied.")
				continue
			}
			b.handleCommand(ctx, update.Message)
		}
	}
}

// SendMessage sends a text message to the given chat.
func (b *Bot) SendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.DisableWebPagePreview = true
	if _, err := b.api.Send(msg); err != nil {
		b.log.Error("send message", "chat_id", chatID, "error", err)
	}
}

func (b *Bot) reply(chatID int64, text string) {
	b.SendMessage(chatID, text)
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	cmd := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())
	chatID := msg.Chat.ID

	b.log.Debug("command", "cmd", cmd, "args", args, "chat_id", chatID)

	switch cmd {
	case "start":
		b.handleStart(chatID)
	case "help":
		b.handleHelp(chatID)
	case "add":
		b.handleAdd(ctx, chatID, args)
	case "list":
		b.handleList(ctx, chatID)
	case "info":
		b.handleInfo(ctx, chatID, args)
	case "remove":
		b.handleRemove(ctx, chatID, args)
	case "rename":
		b.handleRename(ctx, chatID, args)
	case "interval":
		b.handleInterval(ctx, chatID, args)
	case "pause":
		b.handlePause(ctx, chatID, args)
	case "resume":
		b.handleResume(ctx, chatID, args)
	case cmdCheck:
		b.handleCheck(ctx, chatID, args)
	case cmdFilters:
		b.handleFilters(ctx, chatID, args)
	case "include":
		b.handleAddFilter(ctx, chatID, args, "include")
	case "exclude":
		b.handleAddFilter(ctx, chatID, args, "exclude")
	case "include_re":
		b.handleAddFilter(ctx, chatID, args, "include_re")
	case "exclude_re":
		b.handleAddFilter(ctx, chatID, args, "exclude_re")
	case cmdRmFilter:
		b.handleRmFilter(ctx, chatID, args)
	default:
		b.reply(chatID, "Unknown command. Use /help for a list of commands.")
	}
}
