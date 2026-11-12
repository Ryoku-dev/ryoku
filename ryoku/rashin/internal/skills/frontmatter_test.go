package skills

import "testing"

func TestFrontmatterReadsEveryScalarForm(t *testing.T) {
	text := "---\r\n" +
		"name: \"speckit-plan\"\r\n" +
		"single: 'it''s'\r\n" +
		"plain: runs on # a comment\r\n" +
		"folded: >-\r\n" +
		"  one\r\n" +
		"  two\r\n" +
		"\r\n" +
		"  three\r\n" +
		"literal: |\r\n" +
		"  first\r\n" +
		"  second\r\n" +
		"kept: |+\r\n" +
		"  last\r\n" +
		"\r\n" +
		"metadata:\r\n" +
		"  author: someone\r\n" +
		"wrapped: a plain value\r\n" +
		"  that wraps\r\n" +
		"manual: true\r\n" +
		"---\r\n" +
		"# Body\r\n" +
		"name: not frontmatter\r\n"
	keys := frontmatter(text)
	want := map[string]string{
		"name":    "speckit-plan",
		"single":  "it's",
		"plain":   "runs on",
		"folded":  "one two\nthree",
		"literal": "first\nsecond\n",
		"kept":    "last\n\n",
		"wrapped": "a plain value that wraps",
		"manual":  "true",
	}
	for key, value := range want {
		if keys[key] != value {
			t.Errorf("%s = %q, want %q", key, keys[key], value)
		}
	}
	if _, ok := keys["metadata"]; ok {
		t.Errorf("a nested map read as a scalar: %q", keys["metadata"])
	}
	if _, ok := keys["author"]; ok {
		t.Error("a nested key read as a top-level one")
	}
	if !truthy(keys["manual"]) || truthy("false") {
		t.Error("booleans misread")
	}
}

func TestFrontmatterNeedsItsFence(t *testing.T) {
	for _, text := range []string{"# no frontmatter\nname: x\n", "---\nname: never closed\n"} {
		if keys := frontmatter(text); len(keys) != 0 {
			t.Errorf("%q gave %v", text, keys)
		}
	}
}

func TestFrontmatterEscapesInDoubleQuotes(t *testing.T) {
	keys := frontmatter("---\ndescription: \"says \\\"hi\\\" then: stops\"\n---\n")
	if got := keys["description"]; got != `says "hi" then: stops` {
		t.Errorf("description = %q", got)
	}
}
