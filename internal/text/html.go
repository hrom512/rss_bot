package text

import (
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const (
	tagUnorderedList = "ul"
	tagOrderedList   = "ol"
)

// IsHTML detects if content contains HTML markup.
func IsHTML(content string) bool {
	if content == "" {
		return false
	}
	lower := strings.ToLower(content)
	htmlTags := []string{"<p>", "<br", "<div", "<a ", "<ul>", "<ol>", "<li>", "<b>", "<strong>", "<i>", "<em>", "<h1", "<h2", "<h3", "<h4", "<h5", "<h6", "<table", "<tr>", "<td>", "<th>", "<span", "<img"}
	count := 0
	for _, tag := range htmlTags {
		if strings.Contains(lower, tag) {
			count++
		}
	}
	return count >= 1
}

// ParseResult holds parsed text content and extracted image URL.
type ParseResult struct {
	Text     string
	ImageURL string
}

// ParseHTMLToPlain converts HTML content to plain text with structure preserved.
func ParseHTMLToPlain(htmlContent string) ParseResult {
	if !IsHTML(htmlContent) {
		return ParseResult{Text: strings.TrimSpace(htmlContent)}
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ParseResult{Text: strings.TrimSpace(htmlContent)}
	}

	var imageURL string
	text := extractText(doc, &imageURL)

	text = cleanText(text)

	return ParseResult{
		Text:     text,
		ImageURL: imageURL,
	}
}

func extractText(n *html.Node, imageURL *string) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	if n.Type == html.ElementNode && n.Data == "img" {
		for _, attr := range n.Attr {
			if attr.Key == "src" && *imageURL == "" {
				*imageURL = attr.Val
				break
			}
		}
		return ""
	}

	if n.Type == html.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "iframe" || n.Data == "noscript") {
		return ""
	}

	var result strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result.WriteString(extractText(c, imageURL))
	}

	if n.Type == html.ElementNode {
		tag := n.Data

		if tag == "br" {
			return "\n"
		}

		if tag == tagUnorderedList || tag == tagOrderedList {
			var items []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && c.Data == "li" {
					items = append(items, extractText(c, imageURL))
				}
			}
			var sb strings.Builder
			for i, item := range items {
				item = strings.TrimSpace(item)
				if item != "" {
					if tag == "ol" {
						sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
					} else {
						sb.WriteString(fmt.Sprintf("• %s\n", item))
					}
				}
			}
			return sb.String()
		}

		blockTags := []string{"p", "div", "li", "h1", "h2", "h3", "h4", "h5", "h6", "tr", "td", "th"}
		isBlock := false
		for _, bt := range blockTags {
			if tag == bt {
				isBlock = true
				break
			}
		}
		if isBlock && result.Len() > 0 {
			if !strings.HasSuffix(result.String(), "\n") {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

func cleanText(text string) string {
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")

	spaceRegex := regexp.MustCompile(`[ \t]+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)

	return text
}

// NormalizePlainText cleans up plain text by removing extra whitespace.
func NormalizePlainText(text string) string {
	text = strings.TrimSpace(text)

	spaceRegex := regexp.MustCompile(`[\t ]+`)
	text = spaceRegex.ReplaceAllString(text, " ")

	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}

	var result strings.Builder
	for i, line := range cleaned {
		result.WriteString(line)
		if i < len(cleaned)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
