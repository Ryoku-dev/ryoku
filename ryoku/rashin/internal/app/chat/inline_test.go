package chat

import (
	"slices"
	"testing"
)

func spansOf(t *testing.T, source string, want ...inlineSpan) {
	t.Helper()
	if got := parseInline(source); !slices.Equal(got, want) {
		t.Fatalf("%q\n  parsed %#v\n  want   %#v", source, got, want)
	}
}

// A system answer is full of names that carry the marks emphasis is written
// with. None of them is emphasis.
func TestNamesAndPatternsKeepTheirMarks(t *testing.T) {
	for _, source := range []string{
		"Set XDG_RUNTIME_DIR and DBUS_SESSION_BUS_ADDRESS before you start it.",
		"Edit my_service_name.service in /etc/systemd/system.",
		"Run ls *.go and *.md to list them.",
		"2 * 3 * 4 is 24",
		"See [1] and [2] below.",
		"A lone ` tick and a lone * star.",
	} {
		spansOf(t, source, inlineSpan{text: source})
	}
}

func TestEmphasisStrongCodeAndLinks(t *testing.T) {
	spansOf(t, "Plain *soft* **strong** _also soft_ `code` [Ryoku](https://ryoku.dev)",
		inlineSpan{text: "Plain "},
		inlineSpan{text: "soft", style: inlineEmphasis},
		inlineSpan{text: " "},
		inlineSpan{text: "strong", style: inlineStrong},
		inlineSpan{text: " "},
		inlineSpan{text: "also soft", style: inlineEmphasis},
		inlineSpan{text: " "},
		inlineSpan{text: "code", style: inlineCode},
		inlineSpan{text: " "},
		inlineSpan{text: "Ryoku", url: "https://ryoku.dev"},
	)
}

func TestStylesNest(t *testing.T) {
	spansOf(t, "**bold with *soft* inside** and ***both***",
		inlineSpan{text: "bold with ", style: inlineStrong},
		inlineSpan{text: "soft", style: inlineStrong | inlineEmphasis},
		inlineSpan{text: " inside", style: inlineStrong},
		inlineSpan{text: " and "},
		inlineSpan{text: "both", style: inlineStrong | inlineEmphasis},
	)
	spansOf(t, "**`rashin api on`** turns it on",
		inlineSpan{text: "rashin api on", style: inlineStrong | inlineCode},
		inlineSpan{text: " turns it on"},
	)
	spansOf(t, "**[the docs](https://ryoku.dev/docs)**",
		inlineSpan{text: "the docs", style: inlineStrong, url: "https://ryoku.dev/docs"},
	)
}

func TestMarksInsideCodeAreCode(t *testing.T) {
	spansOf(t, "Use `**kwargs` and ``a ` b``",
		inlineSpan{text: "Use "},
		inlineSpan{text: "**kwargs", style: inlineCode},
		inlineSpan{text: " and "},
		inlineSpan{text: "a ` b", style: inlineCode},
	)
}

// Text written without spaces between words has only the marks themselves to
// say where emphasis starts and stops.
func TestStrongNeedsNoSpaceAroundIt(t *testing.T) {
	spansOf(t, "这是**重点**内容",
		inlineSpan{text: "这是"},
		inlineSpan{text: "重点", style: inlineStrong},
		inlineSpan{text: "内容"},
	)
	spansOf(t, "**「重点」**内容",
		inlineSpan{text: "「重点」", style: inlineStrong},
		inlineSpan{text: "内容"},
	)
	spansOf(t, "**Note:**this",
		inlineSpan{text: "Note:", style: inlineStrong},
		inlineSpan{text: "this"},
	)
}

func TestALinkTakesOnlyItsOwnBrackets(t *testing.T) {
	spansOf(t, "See [1] and [the docs](https://ryoku.dev/docs \"Docs\") now.",
		inlineSpan{text: "See [1] and "},
		inlineSpan{text: "the docs", url: "https://ryoku.dev/docs"},
		inlineSpan{text: " now."},
	)
	spansOf(t, "![the bar](https://ryoku.dev/bar.png)",
		inlineSpan{text: "the bar", url: "https://ryoku.dev/bar.png"},
	)
}

func TestAnAddressInTheTextIsALinkWithoutTheStopAfterIt(t *testing.T) {
	spansOf(t, "Read https://ryoku.dev/docs_(v2). Then <https://ryoku.dev> (or http://localhost:3600/v1).",
		inlineSpan{text: "Read "},
		inlineSpan{text: "https://ryoku.dev/docs_(v2)", url: "https://ryoku.dev/docs_(v2)"},
		inlineSpan{text: ". Then "},
		inlineSpan{text: "https://ryoku.dev", url: "https://ryoku.dev"},
		inlineSpan{text: " (or "},
		inlineSpan{text: "http://localhost:3600/v1", url: "http://localhost:3600/v1"},
		inlineSpan{text: ")."},
	)
	spansOf(t, "the path/https://not.a.link", inlineSpan{text: "the path/"}, inlineSpan{text: "https://not.a.link", url: "https://not.a.link"})
	spansOf(t, "xhttps://glued", inlineSpan{text: "xhttps://glued"})
}

func TestABackslashKeepsAMarkAsWritten(t *testing.T) {
	spansOf(t, `\*not soft\* and \[not a link\](x) and C:\Users`, inlineSpan{text: `*not soft* and [not a link](x) and C:\Users`})
}

func TestBlanksFoldToOneOutsideCode(t *testing.T) {
	spansOf(t, "  one   two `a  b`  three  ",
		inlineSpan{text: "one two "},
		inlineSpan{text: "a  b", style: inlineCode},
		inlineSpan{text: " three"},
	)
	spansOf(t, "first <br> second  \nthird", inlineSpan{text: "first\nsecond\nthird"})
}
