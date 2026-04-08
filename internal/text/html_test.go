package text

import (
	"testing"
)

func TestIsHTML(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "empty string",
			content:  "",
			expected: false,
		},
		{
			name:     "plain text only",
			content:  "This is just plain text without any tags.",
			expected: false,
		},
		{
			name:     "plain with newlines",
			content:  "Line one\nLine two\nLine three",
			expected: false,
		},
		{
			name:     "with p tag",
			content:  "<p>This is a paragraph</p>",
			expected: true,
		},
		{
			name:     "with br tag",
			content:  "Line one<br>Line two",
			expected: true,
		},
		{
			name:     "with div tag",
			content:  "<div>Content</div>",
			expected: true,
		},
		{
			name:     "with anchor",
			content:  "Visit <a href=\"http://example.com\">this site</a>",
			expected: true,
		},
		{
			name:     "with ul",
			content:  "<ul><li>Item</li></ul>",
			expected: true,
		},
		{
			name:     "with img",
			content:  "<img src=\"image.jpg\" alt=\"img\"/>",
			expected: true,
		},
		{
			name:     "with strong",
			content:  "<strong>bold text</strong>",
			expected: true,
		},
		{
			name:     "with h2",
			content:  "<h2>Heading</h2>",
			expected: true,
		},
		{
			name:     "with table",
			content:  "<table><tr><td>cell</td></tr></table>",
			expected: true,
		},
		{
			name:     "multiple HTML tags",
			content:  "<p>First</p><ul><li>Item</li></ul>",
			expected: true,
		},
		{
			name:     "HTML-like but not real",
			content:  "<notreal>content</notreal>",
			expected: false,
		},
		{
			name:     "only less than symbol",
			content:  "text with < symbol only",
			expected: false,
		},
		{
			name:     "HTML entity amp",
			content:  "Tom &amp; Jerry",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHTML(tt.content)
			if got != tt.expected {
				t.Errorf("IsHTML(%q) = %v, want %v", tt.content, got, tt.expected)
			}
		})
	}
}

func TestParseHTMLToPlain_Basic(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		wantText string
	}{
		{
			name:     "simple paragraph",
			html:     "<p>Hello world</p>",
			wantText: "Hello world",
		},
		{
			name:     "multiple paragraphs",
			html:     "<p>First paragraph</p><p>Second paragraph</p>",
			wantText: "First paragraph\nSecond paragraph",
		},
		{
			name:     "bold text",
			html:     "<p>This is <strong>bold</strong> text</p>",
			wantText: "This is bold text",
		},
		{
			name:     "italic text",
			html:     "<p>This is <em>italic</em> text</p>",
			wantText: "This is italic text",
		},
		{
			name:     "br line breaks",
			html:     "Line one<br>Line two<br>Line three",
			wantText: "Line one\nLine two\nLine three",
		},
		{
			name:     "link",
			html:     "<p>Visit <a href=\"http://example.com\">this site</a></p>",
			wantText: "Visit this site",
		},
		{
			name:     "unordered list",
			html:     "<ul><li>Item one</li><li>Item two</li></ul>",
			wantText: "• Item one\n• Item two",
		},
		{
			name:     "ordered list",
			html:     "<ol><li>First</li><li>Second</li></ol>",
			wantText: "1. First\n2. Second",
		},
		{
			name:     "nested list",
			html:     "<ul><li>Parent</li><li>Child</li></ul>",
			wantText: "• Parent\n• Child",
		},
		{
			name:     "script tag removed",
			html:     "<p>Text</p><script>alert('x')</script><p>More</p>",
			wantText: "Text\nMore",
		},
		{
			name:     "style tag removed",
			html:     "<p>Text</p><style>.foo {}</style><p>More</p>",
			wantText: "Text\nMore",
		},
		{
			name:     "multiple spaces collapsed",
			html:     "<p>Text   with    spaces</p>",
			wantText: "Text with spaces",
		},
		{
			name:     "leading trailing whitespace",
			html:     "  <p>  Hello  </p>  ",
			wantText: "Hello",
		},
		{
			name:     "heading tags",
			html:     "<h1>Title</h1><p>Content</p>",
			wantText: "Title\nContent",
		},
		{
			name:     "table row",
			html:     "<table><tr><td>Cell</td></tr></table>",
			wantText: "Cell",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHTMLToPlain(tt.html)
			if result.Text != tt.wantText {
				t.Errorf("ParseHTMLToPlain got = %q, want %q", result.Text, tt.wantText)
			}
		})
	}
}

func TestParseHTMLToPlain_ImageExtraction(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantImage string
	}{
		{
			name:      "img with src",
			html:      "<p>Text</p><img src=\"https://example.com/image.jpg\"/><p>More</p>",
			wantImage: "https://example.com/image.jpg",
		},
		{
			name:      "img without src",
			html:      "<img alt=\"no src\"/>",
			wantImage: "",
		},
		{
			name:      "multiple images - first only",
			html:      "<img src=\"first.jpg\"/><img src=\"second.jpg\"/>",
			wantImage: "first.jpg",
		},
		{
			name:      "no image",
			html:      "<p>Just text</p>",
			wantImage: "",
		},
		{
			name:      "image in nested structure",
			html:      "<div><p><img src=\"nested.jpg\"/></p></div>",
			wantImage: "nested.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseHTMLToPlain(tt.html)
			if result.ImageURL != tt.wantImage {
				t.Errorf("ImageURL got = %q, want %q", result.ImageURL, tt.wantImage)
			}
		})
	}
}

func TestParseHTMLToPlain_PreservesContentNotHTML(t *testing.T) {
	plain := "This is plain text content without any HTML tags."
	result := ParseHTMLToPlain(plain)
	if result.Text != plain {
		t.Errorf("plain text not preserved: got %q, want %q", result.Text, plain)
	}
	if result.ImageURL != "" {
		t.Errorf("unexpected image URL: got %q", result.ImageURL)
	}
}

func TestParseHTMLToPlain_CDATA(t *testing.T) {
	plain := "This is just plain text with no HTML."
	result := ParseHTMLToPlain(plain)
	if result.Text != plain {
		t.Errorf("plain text should pass through: got %q", result.Text)
	}
}

func TestNormalizePlainText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "single line",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "multiple lines with empty",
			input: "Line 1\n\n\nLine 2",
			want:  "Line 1\nLine 2",
		},
		{
			name:  "leading trailing spaces",
			input: "   Hello   ",
			want:  "Hello",
		},
		{
			name:  "tabs and spaces",
			input: "Word1\t  Word2",
			want:  "Word1 Word2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePlainText(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePlainText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
