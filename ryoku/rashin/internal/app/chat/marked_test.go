package chat

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"

	"ryoku-rashin/internal/protocol"
)

// answered is a conversation with one question and the answer to it
func answered(t *testing.T, answer string) *conversation {
	t.Helper()
	c := newConversation(image.Pt(760, 700))
	c.model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "What changed?"}))
	c.model.Apply(testEvent(t, 2, protocol.EvText, protocol.TextData{Message: "m", Delta: answer}))
	c.layout()
	c.layout()
	return c
}

// before is where a reader points to stand just before a rune of the text
// that reads as given, wherever the transcript drew it
func (c *conversation) before(t *testing.T, text string, rune int) f32.Point {
	t.Helper()
	origin, found := image.Point{}, false
	for _, node := range c.frame.router.AppendSemantics(nil) {
		if node.Desc.Label == text {
			origin, found = node.Desc.Bounds.Min, true
		}
	}
	if !found {
		t.Fatalf("no text on screen reads %q", text)
	}
	for _, inline := range c.view.state.inlines {
		if inline.set == nil || inline.set.label != text {
			continue
		}
		for index, line := range inline.set.lines {
			if rune >= line.start && rune <= line.end {
				return f32.Pt(float32(origin.X+line.edges[rune-line.start].Round()), float32(origin.Y+index*inline.set.pitch+inline.set.pitch/2))
			}
		}
	}
	for _, selectable := range c.view.state.selectables {
		if selectable.Text() != text {
			continue
		}
		if regions := selectable.Regions(rune, rune+1, nil); len(regions) > 0 {
			bounds := regions[0].Bounds
			return f32.Pt(float32(origin.X+bounds.Min.X), float32(origin.Y+(bounds.Min.Y+bounds.Max.Y)/2))
		}
	}
	t.Fatalf("%q has no rune %d", text, rune)
	return f32.Point{}
}

func (c *conversation) point(kind pointer.Kind, at f32.Point, buttons pointer.Buttons) {
	c.clock += 20 * time.Millisecond
	c.frame.router.Queue(pointer.Event{Kind: kind, Source: pointer.Mouse, PointerID: 1, Position: at, Buttons: buttons, Time: c.clock})
	c.layout()
}

func (c *conversation) drag(from, to f32.Point) {
	c.clock += time.Second
	c.point(pointer.Move, from, 0)
	c.point(pointer.Press, from, pointer.ButtonPrimary)
	c.point(pointer.Move, f32.Pt((from.X+to.X)/2, (from.Y+to.Y)/2), pointer.ButtonPrimary)
	c.point(pointer.Move, to, pointer.ButtonPrimary)
	c.point(pointer.Release, to, 0)
	c.layout()
}

func (c *conversation) press(at f32.Point, times int) {
	c.clock += time.Second
	c.point(pointer.Move, at, 0)
	for range times {
		c.point(pointer.Press, at, pointer.ButtonPrimary)
		c.point(pointer.Release, at, 0)
	}
	c.layout()
}

func (c *conversation) copied() string {
	c.frame.router.Queue(key.Event{Name: "C", Modifiers: key.ModShortcut, State: key.Press})
	c.layout()
	_, content, _ := c.frame.router.WriteClipboard()
	return string(content)
}

func released(actions []Action) bool {
	for _, action := range actions {
		if action.Kind == ActRelease {
			return true
		}
	}
	return false
}

const styledLine = "Set the port with rashin api on and read /etc/rashin/api.conf."

func styledAnswer(t *testing.T) *conversation {
	return answered(t, "Set the **port** with `rashin api on` and read /etc/rashin/api.conf.\n\nA plain closing line.")
}

// A paragraph with a mark in it used to be the one kind of text in an answer
// that could not be selected.
func TestDraggingOverAStyledParagraphMarksItsWordsForCopying(t *testing.T) {
	c := styledAnswer(t)
	c.drag(c.before(t, styledLine, 4), c.before(t, styledLine, 31))
	if marked := c.view.state.markedText(); marked != "the port with rashin api on" {
		t.Fatalf("a drag over the paragraph marked %q", marked)
	}
	if copied := c.copied(); copied != "the port with rashin api on" {
		t.Fatalf("Ctrl+C copied %q", copied)
	}
	if released(c.view.Actions()) {
		t.Fatal("the transcript gave the keyboard away while text in it was marked")
	}
}

func TestASecondPressTakesACommandWholeAndAPathWhole(t *testing.T) {
	c := styledAnswer(t)
	c.press(c.before(t, styledLine, 27), 2)
	if marked := c.view.state.markedText(); marked != "rashin api on" {
		t.Fatalf("a second press inside the code marked %q", marked)
	}
	c.press(c.before(t, styledLine, 50), 2)
	if marked := c.view.state.markedText(); marked != "/etc/rashin/api.conf" {
		t.Fatalf("a second press inside the path marked %q", marked)
	}
}

func TestMarkingInOnePlaceLetsGoOfTheLast(t *testing.T) {
	c := styledAnswer(t)
	const plain = "A plain closing line."
	c.drag(c.before(t, styledLine, 0), c.before(t, styledLine, 3))
	c.drag(c.before(t, plain, 2), c.before(t, plain, 7))
	if marked := c.view.state.markedText(); marked != "plain" {
		t.Fatalf("after marking in the second paragraph the transcript holds %q", marked)
	}
	for _, inline := range c.view.state.inlines {
		if from, to := inline.selection.stretch(); to > from {
			t.Fatalf("the first paragraph still has runes %d..%d marked", from, to)
		}
	}
	c.drag(c.before(t, styledLine, 0), c.before(t, styledLine, 3))
	for _, selectable := range c.view.state.selectables {
		if selectable.SelectionLen() != 0 {
			t.Fatalf("the plain paragraph still has %q marked", selectable.SelectedText())
		}
	}
}

// Marks that stay lit after the keyboard has gone look like something Ctrl+C
// would copy, and it would not.
func TestMarksGoWhenTheKeyboardGoesElsewhere(t *testing.T) {
	for name, text := range map[string]string{"styled": styledLine, "plain": "A plain closing line."} {
		c := styledAnswer(t)
		c.drag(c.before(t, text, 2), c.before(t, text, 9))
		if c.view.state.markedText() == "" {
			t.Fatalf("a drag over the %s paragraph marked nothing", name)
		}
		elsewhere := new(int)
		c.frame.router.Source().Execute(key.FocusCmd{Tag: elsewhere})
		c.layout()
		c.layout()
		if marked := c.view.state.markedText(); marked != "" {
			t.Fatalf("with the keyboard elsewhere the %s paragraph still shows %q marked", name, marked)
		}
	}
}

func TestEscapeLetsGoOfTheMarksAndOfTheKeyboard(t *testing.T) {
	c := styledAnswer(t)
	c.drag(c.before(t, styledLine, 0), c.before(t, styledLine, 3))
	c.view.Actions()
	c.frame.router.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	c.layout()
	if marked := c.view.state.markedText(); marked != "" {
		t.Fatalf("after Escape the transcript still holds %q", marked)
	}
	if !released(c.view.Actions()) {
		t.Fatal("Escape did not hand the keyboard back")
	}
}

// Text takes the keyboard when it is pressed, so Ctrl+C has somewhere to go.
// A press that marked nothing has no use for it.
func TestAPressThatMarksNothingHandsTheKeyboardBack(t *testing.T) {
	for name, text := range map[string]string{"styled": styledLine, "plain": "A plain closing line."} {
		c := styledAnswer(t)
		c.press(c.before(t, text, 5), 1)
		c.layout()
		c.layout()
		if !released(c.view.Actions()) {
			t.Fatalf("a press on the %s paragraph kept the keyboard", name)
		}
	}
}

func TestALinkOpensOnAPressButNotWhenItsWordsAreDraggedOver(t *testing.T) {
	const line = "Read the index notes first."
	opened := func(actions []Action) string {
		for _, action := range actions {
			if action.Kind == ActOpenURL {
				return action.Value
			}
		}
		return ""
	}
	c := answered(t, "Read [the index notes](https://ryoku.dev/docs/rashin#index) first.")
	c.press(c.before(t, line, 9), 1)
	if url := opened(c.view.Actions()); url != "https://ryoku.dev/docs/rashin#index" {
		t.Fatalf("a press on the link opened %q", url)
	}
	c.drag(c.before(t, line, 6), c.before(t, line, 14))
	if url := opened(c.view.Actions()); url != "" {
		t.Fatalf("a drag across the link opened %q", url)
	}
	if marked := c.view.state.markedText(); marked != "he index" {
		t.Fatalf("the drag marked %q", marked)
	}
}
