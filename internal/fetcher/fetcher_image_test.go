package fetcher

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/mmcdole/gofeed"
)

func loadTestXML(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../internal/testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata %s: %v", name, err)
	}
	return string(data)
}

func TestExtractImageURL(t *testing.T) {
	parser := gofeed.NewParser()

	t.Run("from enclosure image", func(t *testing.T) {
		xml := loadTestXML(t, "with_enclosure.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		if len(feed.Items) == 0 {
			t.Fatal("no items")
		}

		url := extractImageURL(feed.Items[0])
		if diff := cmp.Diff("https://images.example.com/photo1.jpg", url); diff != "" {
			t.Errorf("ImageURL mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("from non-image enclosure returns empty", func(t *testing.T) {
		xml := loadTestXML(t, "with_enclosure.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		url := extractImageURL(feed.Items[1])
		if url != "" {
			t.Errorf("expected empty for non-image enclosure, got %q", url)
		}
	})

	t.Run("from RSS image element", func(t *testing.T) {
		// RSS <image> in item is not supported by gofeed, this is a parser limitation
		// So we skip - in real feeds images come via enclosure
		t.Skip("gofeed does not parse <image> element in item")
	})

	t.Run("item without image returns empty", func(t *testing.T) {
		xml := loadTestXML(t, "sample.xml")
		feed, err := parser.ParseString(xml)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		url := extractImageURL(feed.Items[0])
		if url != "" {
			t.Errorf("expected empty, got %q", url)
		}
	})
}

func TestFilterItems_WithContent(t *testing.T) {
	xml := loadTestXML(t, "html_content.xml")
	parser := gofeed.NewParser()
	feed, err := parser.ParseString(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	matched := FilterItems(feed.Items, nil)

	if len(matched) != 3 {
		t.Errorf("expected 3 items, got %d", len(matched))
	}

	if matched[1].ImageURL != "https://example.com/image.jpg" {
		t.Errorf("expected image URL from HTML, got %q", matched[1].ImageURL)
	}
}

func TestFilterItems_WithPlainText(t *testing.T) {
	xml := loadTestXML(t, "plain_text.xml")
	parser := gofeed.NewParser()
	feed, err := parser.ParseString(xml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	matched := FilterItems(feed.Items, nil)

	if len(matched) != 3 {
		t.Errorf("expected 3 items, got %d", len(matched))
	}

	if matched[0].Content != "" {
		t.Error("expected empty content for plain text feed")
	}
}
