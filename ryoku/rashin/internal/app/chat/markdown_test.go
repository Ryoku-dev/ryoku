package chat

import (
	"slices"
	"testing"
)

func TestParseMarkdownNestedLists(t *testing.T) {
	blocks := parseMarkdown("1. First\n  - Child\n    1. Grandchild\n2. Second")
	if len(blocks) != 1 || blocks[0].kind != markdownList {
		t.Fatalf("blocks = %#v", blocks)
	}
	items := blocks[0].items
	if len(items) != 4 {
		t.Fatalf("items = %#v", items)
	}
	if !items[0].ordered || items[0].depth != 0 || items[0].number != 1 {
		t.Fatalf("first = %#v", items[0])
	}
	if items[1].ordered || items[1].depth != 1 || items[1].text != "Child" {
		t.Fatalf("child = %#v", items[1])
	}
	if !items[2].ordered || items[2].depth != 2 || items[2].number != 1 {
		t.Fatalf("grandchild = %#v", items[2])
	}
}

func TestParseMarkdownFencedCode(t *testing.T) {
	blocks := parseMarkdown("Before\n\n```go\nfmt.Println(\"x\")\n```\n\nAfter")
	if len(blocks) != 3 || blocks[1].kind != markdownCode {
		t.Fatalf("blocks = %#v", blocks)
	}
	if blocks[1].language != "go" || blocks[1].text != "fmt.Println(\"x\")" {
		t.Fatalf("code block = %#v", blocks[1])
	}
}

func TestParseMarkdownTable(t *testing.T) {
	blocks := parseMarkdown("| Name | State |\n| --- | :---: |\n| ally | Ready |\n| dev | Stopped |")
	if len(blocks) != 1 || blocks[0].kind != markdownTable {
		t.Fatalf("blocks = %#v", blocks)
	}
	if len(blocks[0].head) != 2 || blocks[0].head[0] != "Name" || len(blocks[0].rows) != 2 || blocks[0].rows[1][1] != "Stopped" {
		t.Fatalf("table = %#v", blocks[0])
	}
}

func TestARuleStandsBetweenParagraphs(t *testing.T) {
	blocks := parseMarkdown("Above\n\n---\n\nBelow\n* * *\nLast")
	kinds := make([]markdownKind, len(blocks))
	for index, block := range blocks {
		kinds[index] = block.kind
	}
	want := []markdownKind{markdownParagraph, markdownRule, markdownParagraph, markdownRule, markdownParagraph}
	if !slices.Equal(kinds, want) {
		t.Fatalf("blocks = %#v, want paragraphs with rules between", blocks)
	}
}

func TestAParagraphKeepsTheBreaksItAsksFor(t *testing.T) {
	blocks := parseMarkdown("Soft\nwrapped\nHard  \nbroken\\\nagain")
	if len(blocks) != 1 || blocks[0].text != "Soft wrapped Hard\nbroken\nagain" {
		t.Fatalf("paragraph = %#v", blocks)
	}
}

func TestQuotedLinesRunOnAndABareMarkStartsTheNextParagraph(t *testing.T) {
	blocks := parseMarkdown("> first line\n> goes on\n>\n> second")
	if len(blocks) != 1 || blocks[0].kind != markdownQuote || blocks[0].text != "first line goes on\n\nsecond" {
		t.Fatalf("quote = %#v", blocks)
	}
}
