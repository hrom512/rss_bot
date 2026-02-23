// Package model defines the domain types used across the application.
package model

import "time"

// Feed represents an RSS feed subscription.
type Feed struct {
	ID              int64
	ChatID          int64
	Name            string
	URL             string
	IntervalMinutes int
	IsActive        bool
	LastCheckAt     *time.Time
	CreatedAt       time.Time
}

// FilterKind defines the type of filter rule.
type FilterKind string

// Supported filter kinds.
const (
	FilterInclude   FilterKind = "include"
	FilterExclude   FilterKind = "exclude"
	FilterIncludeRe FilterKind = "include_re"
	FilterExcludeRe FilterKind = "exclude_re"
)

// FilterScope defines which part of the RSS item a filter matches against.
type FilterScope string

// Supported filter scopes.
const (
	ScopeTitle   FilterScope = "title"
	ScopeContent FilterScope = "content"
	ScopeAll     FilterScope = "all"
)

// Filter represents a single filtering rule attached to a feed.
type Filter struct {
	ID        int64
	FeedID    int64
	Kind      FilterKind
	Scope     FilterScope
	Value     string
	CreatedAt time.Time
}

// SeenItem tracks an RSS item that has already been processed.
type SeenItem struct {
	FeedID int64
	GUID   string
	SeenAt time.Time
}
