package chat

import (
	"image"
	"io"
	"sort"
	"strings"
	"unicode"

	"gioui.org/gesture"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"golang.org/x/image/math/fixed"
)

// anchor stays where the press began, head follows the pointer
type inlineSelection struct {
	click        gesture.Click
	drag         gesture.Drag
	anchor, head int
	dragging     bool
	focused      bool
}

func (selection *inlineSelection) stretch() (from, to int) {
	return min(selection.anchor, selection.head), max(selection.anchor, selection.head)
}

func (selection *inlineSelection) clear() { selection.head = selection.anchor }

func (selection *inlineSelection) update(gtx layout.Context, set *inlineSet) (changed, released bool) {
	// a paragraph still arriving can be shorter than what was marked in it
	selection.anchor, selection.head = min(selection.anchor, len(set.text)), min(selection.head, len(set.text))
	anchor, head := selection.anchor, selection.head
	for {
		pressed, ok := selection.click.Update(gtx.Source)
		if !ok {
			break
		}
		mouse := pressed.Source == pointer.Mouse
		if (pressed.Kind != gesture.KindPress || !mouse) && (pressed.Kind != gesture.KindClick || mouse) {
			continue
		}
		gtx.Execute(key.FocusCmd{Tag: selection})
		selection.dragging = false
		switch at := set.gapAt(pressed.Position); {
		case pressed.NumClicks == 2:
			selection.anchor, selection.head = set.wordAt(pressed.Position)
		case pressed.NumClicks > 2:
			selection.anchor, selection.head = set.lineAround(pressed.Position)
		case pressed.Modifiers == key.ModShift:
			selection.head, selection.dragging = at, true
		default:
			selection.anchor, selection.head, selection.dragging = at, at, true
		}
	}
	for {
		moved, ok := selection.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch moved.Kind {
		case pointer.Drag, pointer.Release:
			if selection.dragging {
				selection.head = set.gapAt(moved.Position.Round())
			}
			if moved.Kind == pointer.Release {
				selection.dragging, released = false, true
			}
		case pointer.Cancel:
			selection.dragging = false
		}
	}
	for {
		event, ok := gtx.Event(
			key.FocusFilter{Target: selection},
			key.Filter{Focus: selection, Name: "C", Required: key.ModShortcut},
			key.Filter{Focus: selection, Name: "X", Required: key.ModShortcut},
			key.Filter{Focus: selection, Name: "A", Required: key.ModShortcut},
		)
		if !ok {
			break
		}
		switch event := event.(type) {
		case key.FocusEvent:
			selection.focused = event.Focus
		case key.Event:
			if event.State != key.Press {
				break
			}
			if event.Name == "A" {
				selection.anchor, selection.head = 0, len(set.text)
				break
			}
			if from, to := selection.stretch(); to > from {
				gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(string(set.text[from:to])))})
			}
		}
	}
	return anchor != selection.anchor || head != selection.head, released
}

func (set *inlineSet) lineAt(y int) *inlineLine {
	return &set.lines[min(max(y/set.pitch, 0), len(set.lines)-1)]
}

// the place between two runes nearest the point
func (set *inlineSet) gapAt(at image.Point) int {
	line, x := set.lineAt(at.Y), fixed.I(at.X)
	last := len(line.edges) - 1
	// the break that ends a line is not a place on it
	if line.end > line.start && set.text[line.end-1] == '\n' {
		last--
	}
	gap := min(sort.Search(len(line.edges), func(index int) bool { return line.edges[index] >= x }), last)
	if gap > 0 && x-line.edges[gap-1] < line.edges[gap]-x {
		gap--
	}
	return line.start + gap
}

// a second press takes a code span whole, since that is what gets copied
// anything else is read the way a terminal reads a word, so a path or an address comes as one piece
func (set *inlineSet) wordAt(at image.Point) (from, to int) {
	line, x := set.lineAt(at.Y), fixed.I(at.X)
	under := line.start + min(max(sort.Search(len(line.edges), func(index int) bool { return line.edges[index] > x })-1, 0), len(line.edges)-2)
	if under < line.start || under >= len(set.text) {
		return line.end, line.end
	}
	span := sort.SearchInts(set.bounds, under+1) - 1
	if set.spans[span].style&inlineCode != 0 {
		return set.bounds[span], set.bounds[span+1]
	}
	if !wordRune(set.text[under]) {
		return under, under + 1
	}
	from, to = under, under+1
	for from > 0 && wordRune(set.text[from-1]) {
		from--
	}
	for to < len(set.text) && wordRune(set.text[to]) {
		to++
	}
	// the stop that ends a sentence is not part of the name before it
	for to-1 > under && strings.ContainsRune(".,:;!?", set.text[to-1]) {
		to--
	}
	return from, to
}

func wordRune(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsDigit(character) || unicode.IsMark(character) || strings.ContainsRune("_-./~:@+=%#&?", character)
}

func (set *inlineSet) lineAround(at image.Point) (from, to int) {
	line := set.lineAt(at.Y)
	from, to = line.start, line.end
	for to > from && unicode.IsSpace(set.text[to-1]) {
		to--
	}
	return from, to
}
