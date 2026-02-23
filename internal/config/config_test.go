package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    *Config
		wantErr bool
	}{
		{
			name:    "missing token",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name: "token only, defaults applied",
			env:  map[string]string{"TELEGRAM_BOT_TOKEN": "test-token"},
			want: &Config{
				TelegramBotToken: "test-token",
				DatabasePath:     "./data/bot.db",
				LogLevel:         "info",
				AllowedUsers:     nil,
			},
		},
		{
			name: "all values set",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "tok",
				"DATABASE_PATH":      "/tmp/bot.db",
				"LOG_LEVEL":          "debug",
				"ALLOWED_USERS":      "111,222,333",
			},
			want: &Config{
				TelegramBotToken: "tok",
				DatabasePath:     "/tmp/bot.db",
				LogLevel:         "debug",
				AllowedUsers:     []int64{111, 222, 333},
			},
		},
		{
			name: "allowed users with spaces",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "tok",
				"ALLOWED_USERS":      " 10 , 20 , ",
			},
			want: &Config{
				TelegramBotToken: "tok",
				DatabasePath:     "./data/bot.db",
				LogLevel:         "info",
				AllowedUsers:     []int64{10, 20},
			},
		},
		{
			name: "invalid user id",
			env: map[string]string{
				"TELEGRAM_BOT_TOKEN": "tok",
				"ALLOWED_USERS":      "123,abc",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear relevant env vars
			for _, key := range []string{"TELEGRAM_BOT_TOKEN", "DATABASE_PATH", "LOG_LEVEL", "ALLOWED_USERS"} {
				t.Setenv(key, "")
			}
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			got, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Load() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []int64
		userID       int64
		want         bool
	}{
		{
			name:         "empty list allows everyone",
			allowedUsers: nil,
			userID:       42,
			want:         true,
		},
		{
			name:         "user in list",
			allowedUsers: []int64{10, 20, 30},
			userID:       20,
			want:         true,
		},
		{
			name:         "user not in list",
			allowedUsers: []int64{10, 20, 30},
			userID:       99,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AllowedUsers: tt.allowedUsers}
			got := cfg.IsUserAllowed(tt.userID)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("IsUserAllowed() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
