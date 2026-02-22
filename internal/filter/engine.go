// Package filter implements the feed item matching engine.
package filter

import (
	"fmt"
	"regexp"
	"strings"

	"rss_bot/internal/model"
)

// FeedItem represents an RSS item to be matched against filters.
type FeedItem struct {
	Title       string
	Description string
}

// Match checks whether an item passes the given set of filters.
// If no filters are provided, the item always passes.
// Include filters use OR logic (at least one must match).
// Exclude filters use AND logic (none must match).
func Match(item FeedItem, filters []model.Filter) bool {
	if len(filters) == 0 {
		return true
	}

	hasIncludes := false
	anyIncludeMatched := false

	for _, f := range filters {
		switch f.Kind {
		case model.FilterInclude, model.FilterIncludeRe:
			hasIncludes = true
			if matchesFilter(item, f) {
				anyIncludeMatched = true
			}
		case model.FilterExclude, model.FilterExcludeRe:
			if matchesFilter(item, f) {
				return false
			}
		}
	}

	if hasIncludes && !anyIncludeMatched {
		return false
	}
	return true
}

func matchesFilter(item FeedItem, f model.Filter) bool {
	text := textForScope(item, f.Scope)
	switch f.Kind {
	case model.FilterInclude, model.FilterExclude:
		return strings.Contains(text, strings.ToLower(f.Value))
	case model.FilterIncludeRe, model.FilterExcludeRe:
		re, err := regexp.Compile("(?i)" + f.Value)
		if err != nil {
			return false
		}
		return re.MatchString(text)
	}
	return false
}

func textForScope(item FeedItem, scope model.FilterScope) string {
	switch scope {
	case model.ScopeTitle:
		return strings.ToLower(item.Title)
	case model.ScopeContent:
		return strings.ToLower(item.Description)
	default:
		return strings.ToLower(item.Title + " " + item.Description)
	}
}

// ValidateRegex checks whether a pattern is a valid regular expression.
func ValidateRegex(pattern string) error {
	_, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return fmt.Errorf("invalid regex: %w", err)
	}
	return nil
}
