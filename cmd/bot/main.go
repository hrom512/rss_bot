package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"rss_bot/internal/bot"
	"rss_bot/internal/config"
	"rss_bot/internal/scheduler"
	"rss_bot/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)

	if dir := filepath.Dir(cfg.DatabasePath); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Error("create data directory", "path", dir, "error", err)
			os.Exit(1)
		}
	}

	store, err := storage.NewSQLite(cfg.DatabasePath)
	if err != nil {
		log.Error("open database", "path", cfg.DatabasePath, "error", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	b, err := bot.New(cfg.TelegramBotToken, store, cfg, log)
	if err != nil {
		log.Error("create bot", "error", err)
		os.Exit(1)
	}

	sched := scheduler.New(store, b, log)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("starting bot")

	go sched.Run(ctx)

	b.Run(ctx)

	log.Info("bot stopped")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
