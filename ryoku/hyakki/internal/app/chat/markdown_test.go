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

// a wrapped item goes on under its marker, as SKILL.md files and answers both write them
func TestAListItemCarriesOnOverAnIndentedLine(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		want   []markdownKind
		items  []string
	}{
		"under a bullet": {
			source: "- **In the request** (\"make the icon cornflower\n  blue\"). Do it.\n- Next",
			want:   []markdownKind{markdownList},
			items:  []string{"**In the request** (\"make the icon cornflower blue\"). Do it.", "Next"},
		},
		"under a numbered item": {
			source: "1. Lay out the approaches in a message of their\n   own, and have them pick one.\n2. Write the design doc.",
			want:   []markdownKind{markdownList},
			items:  []string{"Lay out the approaches in a message of their own, and have them pick one.", "Write the design doc."},
		},
		"a nested list stays a list": {
			source: "- Parent\n  - Child",
			want:   []markdownKind{markdownList},
			items:  []string{"Parent", "Child"},
		},
		"an indented fence stays code": {
			source: "- Run it:\n  ```sh\n  make\n  ```",
			want:   []markdownKind{markdownList, markdownCode},
			items:  []string{"Run it:"},
		},
		"a blank line and a paragraph end the list": {
			source: "- Item\n\nAfter the list",
			want:   []markdownKind{markdownList, markdownParagraph},
			items:  []string{"Item"},
		},
	} {
		blocks := parseMarkdown(test.source)
		kinds := make([]markdownKind, len(blocks))
		for index, block := range blocks {
			kinds[index] = block.kind
		}
		if !slices.Equal(kinds, test.want) {
			t.Errorf("%s: blocks = %#v, want kinds %v", name, blocks, test.want)
			continue
		}
		texts := make([]string, len(blocks[0].items))
		for index, item := range blocks[0].items {
			texts[index] = item.text
		}
		if !slices.Equal(texts, test.items) {
			t.Errorf("%s: items = %q, want %q", name, texts, test.items)
		}
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
