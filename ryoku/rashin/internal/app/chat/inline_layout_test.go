package chat

import (
	"image"
	"testing"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

func setAt(theme *look.Theme, width int, spans ...inlineSpan) *inlineSet {
	faces := inlineFaces{base: theme.T.Proof, mono: theme.T.Mono}
	return setInline(theme.Shaper, unit.Metric{PxPerDp: 1, PxPerSp: 1}, system.Locale{}, faces, spans, width)
}

// The words of an answer are not the reader's to lose: whatever the width, a
// paragraph sets every rune it has on some line, and no line runs past the
// edge. A long address and text without spaces used to be cut at one line.
func TestEveryRuneIsSetAndNoLineRunsPastTheEdge(t *testing.T) {
	theme := look.New(0)
	for name, source := range map[string]string{
		"words":   "A **short** sentence that wraps over several lines because the column is `narrow` here.",
		"address": "See **this** https://example.com/a/very/long/path/that/does/not/have/any/spaces/in/it/and/keeps/going/for/a/while/longer.html now",
		"command": "Run `systemctl --user restart rashind && journalctl --user -u rashind -f --since yesterday --no-pager` and wait.",
		"no gaps": "这是一个**没有空格**的很长的句子用来测试换行是否正常工作这是一个没有空格的很长的句子用来测试换行是否正常工作",
		"breaks":  "one\ntwo **three**\n\nfour",
	} {
		for _, width := range []int{120, 300, 640} {
			set := setAt(theme, width, parseInline(source)...)
			next := 0
			for index, line := range set.lines {
				if line.start != next || len(line.edges) != line.end-line.start+1 {
					t.Fatalf("%s at %d: line %d holds runes %d..%d with %d edges after rune %d", name, width, index, line.start, line.end, len(line.edges), next)
				}
				for edge := 1; edge < len(line.edges); edge++ {
					if line.edges[edge] < line.edges[edge-1] {
						t.Fatalf("%s at %d: line %d steps back at rune %d", name, width, index, line.start+edge)
					}
				}
				for _, run := range line.runs {
					if run.to.Ceil() > width {
						t.Fatalf("%s at %d: line %d runs to %d", name, width, index, run.to.Ceil())
					}
				}
				next = line.end
			}
			if next != len(set.text) {
				t.Fatalf("%s at %d: %d of %d runes are set", name, width, next, len(set.text))
			}
			if want := set.baseline + (len(set.lines)-1)*set.pitch; set.size.Y <= want || set.size.X > width {
				t.Fatalf("%s at %d: %d lines stand in %v", name, width, len(set.lines), set.size)
			}
		}
	}
}

// A paragraph with a mark in it and one without stand side by side in every
// answer, so they have to break and space their lines alike.
func TestAStyledParagraphStandsLikeAPlainOne(t *testing.T) {
	frame := newChatFrame(image.Pt(800, 600))
	const text = "The cache remains safe while this runs, and the incremental watcher picks up where the last scan left off once the graph settles."
	for _, width := range []int{180, 320, 470, 700} {
		gtx := frame.context()
		gtx.Constraints = layout.Constraints{Max: image.Pt(width, 600)}
		plain := kit.Label{Style: frame.theme.T.Proof}.Layout(gtx, frame.theme.Theme, text)
		// the same words in two spans go through the setter and come out unstyled
		set := setAt(frame.theme, width, inlineSpan{text: text[:40]}, inlineSpan{text: text[40:], url: "x"})
		if set.size.Y != plain.Size.Y {
			t.Fatalf("at %d the set paragraph is %d tall in %d lines, the plain one %d", width, set.size.Y, len(set.lines), plain.Size.Y)
		}
	}
}

// Code in a sentence takes no room of its own. Room either side of it pushed
// a bracket or a comma off the name it belongs to, and read as stray blanks.
func TestCodeSitsFlushWithWhatSurroundsIt(t *testing.T) {
	set := setAt(look.New(0), 640, parseInline("the rules (`AGENTS.md`, `prowlrc`) apply")...)
	if len(set.lines) != 1 || len(set.lines[0].runs) != 5 {
		t.Fatalf("lines = %#v", set.lines)
	}
	runs := set.lines[0].runs
	for index := 1; index < len(runs); index++ {
		if runs[index].glyphs[0].X != runs[index-1].to {
			t.Fatalf("run %d starts at %v, the one before it ends at %v", index, runs[index].glyphs[0].X, runs[index-1].to)
		}
	}
}

// Half a command at the end of a line reads as a different command.
func TestCodeThatFitsOnALineIsNeverParted(t *testing.T) {
	theme := look.New(0)
	spans := parseInline("`id -u` gives the number in the per-user runtime directory for sockets, typically `/run/user/$(id -u)` on this machine.")
	for width := 180; width <= 520; width += 17 {
		set := setAt(theme, width, spans...)
		for index, line := range set.lines[:len(set.lines)-1] {
			for span := range spans {
				if spans[span].style&inlineCode != 0 && line.end > set.bounds[span] && line.end < set.bounds[span+1] {
					t.Fatalf("at %d line %d ends inside %q", width, index, spans[span].text)
				}
			}
		}
	}
}

func TestABreakTheTextAsksForStartsALine(t *testing.T) {
	set := setAt(look.New(0), 640, parseInline("one\ntwo **three**\n\nfour")...)
	if len(set.lines) != 4 || len(set.lines[2].runs) != 0 {
		t.Fatalf("%d lines, want one, two three, an empty one and four: %#v", len(set.lines), set.lines)
	}
}
