package bot

import (
	"fmt"
	"strconv"
	"strings"

	"rss_bot/internal/model"
)

// FilterArgs holds the parsed arguments of a filter command.
type FilterArgs struct {
	FeedID int64
	Scope  model.FilterScope
	Value  string
}

// ParseFilterCommand parses arguments for /include, /exclude, etc.
// Format: <feed_id> [-s title|content|all] <value...>
func ParseFilterCommand(args string) (FilterArgs, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return FilterArgs{}, fmt.Errorf("usage: <feed_id> [-s title|content|all] <value>")
	}

	feedID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return FilterArgs{}, fmt.Errorf("invalid feed ID %q", parts[0])
	}

	scope := model.ScopeAll
	rest := parts[1:]

	if len(rest) >= 2 && rest[0] == "-s" {
		switch rest[1] {
		case "title":
			scope = model.ScopeTitle
		case "content":
			scope = model.ScopeContent
		case "all":
			scope = model.ScopeAll
		default:
			return FilterArgs{}, fmt.Errorf("invalid scope %q, use: title, content, all", rest[1])
		}
		rest = rest[2:]
	}

	if len(rest) == 0 {
		return FilterArgs{}, fmt.Errorf("filter value is required")
	}

	return FilterArgs{
		FeedID: feedID,
		Scope:  scope,
		Value:  strings.Join(rest, " "),
	}, nil
}

// ParseIDArg extracts a numeric ID from a command argument string.
func ParseIDArg(args string) (int64, error) {
	s := strings.TrimSpace(args)
	if s == "" {
		return 0, fmt.Errorf("feed ID is required")
	}
	id, err := strconv.ParseInt(strings.Fields(s)[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid feed ID %q", s)
	}
	return id, nil
}

// ParseRenameArgs extracts a feed ID and new name from command arguments.
func ParseRenameArgs(args string) (int64, string, error) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("usage: /rename <id> <new_name>")
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid feed ID %q", parts[0])
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return 0, "", fmt.Errorf("new name cannot be empty")
	}
	return id, name, nil
}

// ParseIntervalArgs extracts a feed ID and interval in minutes.
func ParseIntervalArgs(args string) (int64, int, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("usage: /interval <id> <minutes>")
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid feed ID %q", parts[0])
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil || mins < 1 || mins > 1440 {
		return 0, 0, fmt.Errorf("interval must be between 1 and 1440 minutes")
	}
	return id, mins, nil
}
