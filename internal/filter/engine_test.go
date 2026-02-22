package filter

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"rss_bot/internal/model"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		item    FeedItem
		filters []model.Filter
		want    bool
	}{
		{
			name:    "no filters passes everything",
			item:    FeedItem{Title: "anything", Description: "whatever"},
			filters: nil,
			want:    true,
		},
		{
			name: "include word matches",
			item: FeedItem{Title: "Kubernetes 1.32 released", Description: "New features"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "include word no match",
			item: FeedItem{Title: "Python update", Description: "New features"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			want: false,
		},
		{
			name: "include is case insensitive",
			item: FeedItem{Title: "KUBERNETES release", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "exclude word blocks match",
			item: FeedItem{Title: "Job vacancy at Google", Description: "Apply now"},
			filters: []model.Filter{
				{Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
			},
			want: false,
		},
		{
			name: "exclude word does not block non-match",
			item: FeedItem{Title: "Kubernetes update", Description: "New features"},
			filters: []model.Filter{
				{Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
			},
			want: true,
		},
		{
			name: "include + exclude: include matches, exclude does not",
			item: FeedItem{Title: "Kubernetes 1.32", Description: "sidecar support"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
				{Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
			},
			want: true,
		},
		{
			name: "include + exclude: both match, exclude wins",
			item: FeedItem{Title: "Kubernetes vacancy", Description: "Apply now"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
				{Kind: model.FilterExclude, Scope: model.ScopeAll, Value: "vacancy"},
			},
			want: false,
		},
		{
			name: "multiple includes OR logic: first matches",
			item: FeedItem{Title: "Docker update", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "docker"},
			},
			want: true,
		},
		{
			name: "multiple includes OR logic: none match",
			item: FeedItem{Title: "Python news", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "docker"},
			},
			want: false,
		},
		{
			name: "regex include matches",
			item: FeedItem{Title: "Helm chart v3.15", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterIncludeRe, Scope: model.ScopeAll, Value: "helm|docker"},
			},
			want: true,
		},
		{
			name: "regex exclude blocks",
			item: FeedItem{Title: "Online course on K8s training", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterExcludeRe, Scope: model.ScopeAll, Value: "course.*training"},
			},
			want: false,
		},
		{
			name: "invalid regex in filter is skipped (no match)",
			item: FeedItem{Title: "anything", Description: ""},
			filters: []model.Filter{
				{Kind: model.FilterIncludeRe, Scope: model.ScopeAll, Value: "[invalid"},
			},
			want: false,
		},
		{
			name: "unicode cyrillic include",
			item: FeedItem{Title: "Деплой в Kubernetes", Description: "Руководство"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "деплой"},
			},
			want: true,
		},
		{
			name: "scope title: word in title matches",
			item: FeedItem{Title: "Kubernetes release", Description: "Nothing here"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeTitle, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "scope title: word only in description does not match",
			item: FeedItem{Title: "Release notes", Description: "Kubernetes update"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeTitle, Value: "kubernetes"},
			},
			want: false,
		},
		{
			name: "scope content: word in description matches",
			item: FeedItem{Title: "Release notes", Description: "Kubernetes sidecar support"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeContent, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "scope content: word only in title does not match",
			item: FeedItem{Title: "Kubernetes 1.32", Description: "General improvements"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeContent, Value: "kubernetes"},
			},
			want: false,
		},
		{
			name: "scope all: matches word in title",
			item: FeedItem{Title: "Kubernetes release", Description: "Improvements"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "scope all: matches word in description",
			item: FeedItem{Title: "Release notes", Description: "Kubernetes sidecar"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeAll, Value: "kubernetes"},
			},
			want: true,
		},
		{
			name: "mixed scopes: title include + content exclude",
			item: FeedItem{Title: "Kubernetes release", Description: "Sponsored promo content"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeTitle, Value: "kubernetes"},
				{Kind: model.FilterExclude, Scope: model.ScopeContent, Value: "promo"},
			},
			want: false,
		},
		{
			name: "mixed scopes: title include + content exclude (no exclude hit)",
			item: FeedItem{Title: "Kubernetes release", Description: "Great improvements"},
			filters: []model.Filter{
				{Kind: model.FilterInclude, Scope: model.ScopeTitle, Value: "kubernetes"},
				{Kind: model.FilterExclude, Scope: model.ScopeContent, Value: "promo"},
			},
			want: true,
		},
		{
			name: "exclude scope content: word in title is not excluded",
			item: FeedItem{Title: "Promo for Kubernetes", Description: "Great article"},
			filters: []model.Filter{
				{Kind: model.FilterExclude, Scope: model.ScopeContent, Value: "promo"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.item, tt.filters)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Match() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateRegex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{name: "valid simple", pattern: "hello", wantErr: false},
		{name: "valid alternation", pattern: "k8s|docker|helm", wantErr: false},
		{name: "valid group", pattern: "(?i)release.*v\\d+", wantErr: false},
		{name: "invalid unclosed bracket", pattern: "[invalid", wantErr: true},
		{name: "invalid bad repetition", pattern: "*bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRegex(tt.pattern)
			gotErr := err != nil
			if diff := cmp.Diff(tt.wantErr, gotErr); diff != "" {
				t.Errorf("ValidateRegex() error mismatch (-want +got):\n%s\nerr: %v", diff, err)
			}
		})
	}
}
