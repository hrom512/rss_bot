# RSSift Bot

Telegram bot for subscribing to RSS feeds with whitelist/blacklist filtering. Get notified only about the items that match your criteria.

## Features

- Multiple RSS feeds per user
- Per-feed check interval (1-1440 minutes)
- Filter by word/phrase or regex
- Whitelist (include) and blacklist (exclude) filters
- Per-filter scope: title only, content only, or both
- Pause/resume individual feeds
- Force check on demand

## Quick Start

### Prerequisites

- Go 1.21+
- Telegram bot token from [@BotFather](https://t.me/BotFather)

### Run locally

```bash
cp .env.example .env
# Edit .env and set your TELEGRAM_BOT_TOKEN

export $(cat .env | xargs)
go run ./cmd/bot/
```

### Run with Docker Compose

```bash
cp .env.example .env
# Edit .env and set your TELEGRAM_BOT_TOKEN

docker compose up -d
```

## Configuration

| Variable | Required | Default | Description |
|---|---|---|---|
| `TELEGRAM_BOT_TOKEN` | yes | — | Bot token from @BotFather |
| `DATABASE_PATH` | no | `./data/bot.db` | Path to SQLite database |
| `LOG_LEVEL` | no | `info` | debug, info, warn, error |
| `ALLOWED_USERS` | no | — | Comma-separated Telegram user IDs; empty = allow all |

## Bot Commands

### Feed Management

| Command | Description |
|---|---|
| `/add <url>` | Add a new RSS feed |
| `/list` | Show all feeds |
| `/info <id>` | Feed details and filters |
| `/remove <id>` | Delete a feed |
| `/rename <id> <name>` | Rename a feed |
| `/interval <id> <min>` | Set check interval (1-1440) |
| `/pause <id>` | Pause checking |
| `/resume <id>` | Resume checking |
| `/check <id>` | Force check now |

### Filter Management

| Command | Description |
|---|---|
| `/filters <id>` | Show all filters for a feed |
| `/include <id> [-s scope] <word>` | Add whitelist word/phrase |
| `/exclude <id> [-s scope] <word>` | Add blacklist word/phrase |
| `/include_re <id> [-s scope] <regex>` | Add whitelist regex |
| `/exclude_re <id> [-s scope] <regex>` | Add blacklist regex |
| `/rmfilter <filter_id>` | Remove a filter |

### Scope Flag

The `-s` flag controls which part of the RSS item the filter matches against:

- `-s title` — match only the item title
- `-s content` — match only the item description
- `-s all` — match both (default)

### Filter Logic

- **Whitelist**: if any include filters exist, at least one must match
- **Blacklist**: if any exclude filter matches, the item is rejected
- Result: item passes whitelist AND passes blacklist

### Examples

```
/add https://habr.com/ru/rss/flows/develop/all/
/include 1 kubernetes
/include 1 -s title deploy
/exclude 1 vacancy
/exclude_re 1 -s content (?i)promo|partner
/filters 1
/check 1
```

## Development

```bash
# Run tests
make test

# Build binary
make build

# Lint
make lint
```

## Project Structure

```
cmd/bot/main.go          — entry point
internal/
  config/                — environment config
  model/                 — domain models (Feed, Filter)
  storage/               — SQLite storage layer
  filter/                — filter matching engine
  fetcher/               — RSS fetch and parse
  scheduler/             — periodic feed checker
  bot/                   — Telegram bot handlers
migrations/              — SQL schema
testdata/                — RSS XML fixtures
```
