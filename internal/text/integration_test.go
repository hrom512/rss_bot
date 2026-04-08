package text

import (
	"os"
	"testing"

	"github.com/mmcdole/gofeed"

	"rss_bot/internal/fetcher"
)

const testImageURL = "https://images.example.com/photo1.jpg"

func loadTestData(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../internal/testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return string(data)
}

func TestIntegration_RSSParserToFormattedMessage(t *testing.T) {
	parser := gofeed.NewParser()

	t.Run("HTML content feed - full conversion", func(t *testing.T) {
		xml := loadTestData(t, "html_content.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		if feed.Title != "HTML Content Feed" {
			t.Errorf("feed title = %q, want %q", feed.Title, "HTML Content Feed")
		}

		if len(feed.Items) != 3 {
			t.Fatalf("items count = %d, want %d", len(feed.Items), 3)
		}

		item := feed.Items[0]
		if item.Title != "Article with Lists and Formatting" {
			t.Errorf("item title = %q, want %q", item.Title, "Article with Lists and Formatting")
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)
		if len(matchedItems) != 3 {
			t.Errorf("matched items = %d, want %d", len(matchedItems), 3)
		}

		result := FormatNotification("HTML Content Feed", matchedItems[0])

		expected := `[HTML Content Feed]

Article with Lists and Formatting

This is a bold and italic text.

Here is a list:

• First item
• Second item
• Third item

And a numbered list:

1. Step one
2. Step two
3. Step three

Check out this link.

https://html.example.com/article-1`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("HTML content with image - image URL extracted", func(t *testing.T) {
		xml := loadTestData(t, "html_content.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		formatted := FormatNotificationShort(123, "Test Feed", matchedItems[1])

		if formatted.ImageURL != "https://example.com/image.jpg" {
			t.Errorf("ImageURL = %q, want %q", formatted.ImageURL, "https://example.com/image.jpg")
		}

		expectedText := `[Test Feed]

Article with Image

This article has an image:

Some description below the image.

https://html.example.com/article-2`

		if formatted.Text != expectedText {
			t.Errorf("text mismatch:\nwant:\n%s\ngot:\n%s", expectedText, formatted.Text)
		}
	})

	t.Run("plain text feed - preserved as-is", func(t *testing.T) {
		xml := loadTestData(t, "plain_text.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		result := FormatNotification("Plain Text Feed", matchedItems[0])

		expected := `[Plain Text Feed]

Simple Text Article

This is a plain text description without any HTML tags. Just regular text content.

https://plain.example.com/article-1`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("plain text multiline - preserved", func(t *testing.T) {
		xml := loadTestData(t, "plain_text.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		result := FormatNotification("Plain Text Feed", matchedItems[1])

		expected := `[Plain Text Feed]

Multiline Plain Text

First line of the description.
Second line of the description.
Third line with some more content here.

https://plain.example.com/article-2`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("feed with enclosure image - image URL from enclosure", func(t *testing.T) {
		xml := loadTestData(t, "with_enclosure.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		if len(matchedItems) == 0 {
			t.Fatal("no matched items")
		}

		if matchedItems[0].ImageURL != testImageURL {
			t.Errorf("ImageURL = %q, want %q", matchedItems[0].ImageURL, testImageURL)
		}

		formatted := FormatNotificationShort(1, "Feed with Images", matchedItems[0])

		expectedText := `[Feed with Images]

Article with Enclosure Image

Description for enclosure image article.

https://images.example.com/article-1`

		if formatted.Text != expectedText {
			t.Errorf("text mismatch:\nwant:\n%s\ngot:\n%s", expectedText, formatted.Text)
		}

		if formatted.ImageURL != testImageURL {
			t.Errorf("ImageURL = %q, want %q", formatted.ImageURL, testImageURL)
		}
	})

	t.Run("content takes priority over description", func(t *testing.T) {
		xml := loadTestData(t, "with_enclosure.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		if len(matchedItems) < 4 {
			t.Skip("not enough items")
		}

		item := matchedItems[3]

		if item.Content == "" {
			t.Error("content should be populated")
		}

		result := FormatNotification("Test Feed", item)

		expected := `[Test Feed]

Article with Content and Image

This is HTML content with an image:

https://images.example.com/article-4`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("script tag removed from HTML", func(t *testing.T) {
		xml := loadTestData(t, "html_content.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		result := FormatNotification("Test Feed", matchedItems[2])

		if result == "" {
			t.Fatal("result should not be empty")
		}

		expected := `[Test Feed]

Article with Script (should be removed)

Normal paragraph.

Another paragraph.

https://html.example.com/article-3`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})

	t.Run("plain text with special characters", func(t *testing.T) {
		xml := loadTestData(t, "plain_text.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse RSS: %v", err)
		}

		matchedItems := fetcher.FilterItems(feed.Items, nil)

		result := FormatNotification("Plain Text Feed", matchedItems[2])

		expected := `[Plain Text Feed]

Plain Text with Special Characters

Contains & ampersand, < less than, > greater than, " quotes and more text here.

https://plain.example.com/article-3`

		if result != expected {
			t.Errorf("result mismatch:\nwant:\n%s\ngot:\n%s", expected, result)
		}
	})
}
