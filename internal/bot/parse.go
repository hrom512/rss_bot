package bot

import (
	"fmt"
	"strconv"
	"strings"

	"rss_bot/internal/model"
)

// FilterArgs holds the parsed arguments of a filter command.
type FilterArgs struct {
	FeedPosition int
	Scope        model.FilterScope
	Value        string
}

// ParseFilterCommand parses arguments for /include, /exclude, etc.
// Format: <feed_position> [-s title|content|all] <value...>
func ParseFilterCommand(args string) (FilterArgs, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return FilterArgs{}, fmt.Errorf("usage: <feed_number> [-s title|content|all] <value>")
	}

	feedPos, err := strconv.Atoi(parts[0])
	if err != nil {
		return FilterArgs{}, fmt.Errorf("invalid feed number %q", parts[0])
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
		FeedPosition: feedPos,
		Scope:        scope,
		Value:        strings.Join(rest, " "),
	}, nil
}

// ParseFeedArg extracts a local feed number from a command argument string.
func ParseFeedArg(args string) (int, error) {
	s := strings.TrimSpace(args)
	if s == "" {
		return 0, fmt.Errorf("feed number is required")
	}
	n, err := strconv.Atoi(strings.Fields(s)[0])
	if err != nil {
		return 0, fmt.Errorf("invalid feed number %q", s)
	}
	return n, nil
}

// ParseFilterArg extracts a local filter number from a command argument string.
func ParseFilterArg(args string) (int, error) {
	s := strings.TrimSpace(args)
	if s == "" {
		return 0, fmt.Errorf("filter number is required")
	}
	n, err := strconv.Atoi(strings.Fields(s)[0])
	if err != nil {
		return 0, fmt.Errorf("invalid filter number %q", s)
	}
	return n, nil
}

// ParseRenameArgs extracts a feed number and new name from command arguments.
func ParseRenameArgs(args string) (int, string, error) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		return 0, "", fmt.Errorf("usage: /rename <number> <new_name>")
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", fmt.Errorf("invalid feed number %q", parts[0])
	}
	name := strings.TrimSpace(parts[1])
	if name == "" {
		return 0, "", fmt.Errorf("new name cannot be empty")
	}
	return n, name, nil
}

// ParseIntervalArgs extracts a feed number and interval in minutes.
func ParseIntervalArgs(args string) (int, int, error) {
	parts := strings.Fields(args)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("usage: /interval <number> <minutes>")
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid feed number %q", parts[0])
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil || mins < 1 || mins > 1440 {
		return 0, 0, fmt.Errorf("interval must be between 1 and 1440 minutes")
	}
	return n, mins, nil
}
