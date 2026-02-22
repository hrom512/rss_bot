// Package config handles application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds the application configuration.
type Config struct {
	TelegramBotToken string
	DatabasePath     string
	LogLevel         string
	AllowedUsers     []int64
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./data/bot.db"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	var allowedUsers []int64
	if raw := os.Getenv("ALLOWED_USERS"); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			uid, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid user ID %q in ALLOWED_USERS: %w", s, err)
			}
			allowedUsers = append(allowedUsers, uid)
		}
	}

	return &Config{
		TelegramBotToken: token,
		DatabasePath:     dbPath,
		LogLevel:         logLevel,
		AllowedUsers:     allowedUsers,
	}, nil
}

// IsUserAllowed checks whether a user ID is in the allow list.
// Returns true if the allow list is empty (all users permitted).
func (c *Config) IsUserAllowed(userID int64) bool {
	if len(c.AllowedUsers) == 0 {
		return true
	}
	for _, id := range c.AllowedUsers {
		if id == userID {
			return true
		}
	}
	return false
}
