package kit

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// keyHarness drives real frames through a Gio input router, so focus moves
// and key delivery behave as they do in a window.
type keyHarness struct {
	router input.Router
	ops    op.Ops
	now    time.Time
	layout func(gtx layout.Context)
}

func newKeyHarness(t *testing.T, layout func(gtx layout.Context)) *keyHarness {
	t.Helper()
	anim.SetReduce(true)
	t.Cleanup(func() { anim.SetReduce(false) })
	return &keyHarness{now: time.Unix(1000, 0), layout: layout}
}

func (h *keyHarness) frame(extra func(gtx layout.Context)) {
	h.ops.Reset()
	h.now = h.now.Add(time.Second)
	gtx := layout.Context{
		Ops: &h.ops, Now: h.now, Source: h.router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(800, 600)),
	}
	h.layout(gtx)
	if extra != nil {
		extra(gtx)
	}
	h.router.Frame(&h.ops)
}

func (h *keyHarness) settle() {
	for range 3 {
		h.frame(nil)
	}
}

func (h *keyHarness) press(names ...key.Name) {
	for _, name := range names {
		h.router.Queue(key.Event{Name: name, State: key.Press}, key.Event{Name: name, State: key.Release})
		h.settle()
	}
}

func (h *keyHarness) focused(tag any) bool {
	focused := false
	h.frame(func(gtx layout.Context) { focused = gtx.Focused(tag) })
	return focused
}

type pickerFixture struct {
	picker   Picker
	selected int
	changes  int
	harness  *keyHarness
}

func newPickerFixture(t *testing.T, selected int) *pickerFixture {
	th := theme.New(palette.Signature(true))
	options := []string{"Follow", "Dark", "Light", "High contrast"}
	f := &pickerFixture{selected: selected}
	f.harness = newKeyHarness(t, func(gtx layout.Context) {
		f.picker.Layout(gtx, th, "Mode", options, f.selected)
		if index, ok := f.picker.Changed(gtx); ok {
			f.selected = index
			f.changes++
		}
	})
	f.harness.settle()
	f.harness.frame(func(gtx layout.Context) { f.picker.Focus(gtx) })
	f.harness.settle()
	return f
}

func TestPickerKeyboardChoosesWithArrowsAndReturn(t *testing.T) {
	f := newPickerFixture(t, 0)
	f.harness.press(key.NameDownArrow)
	if !f.picker.Open() {
		t.Fatal("Down on the anchor did not open the picker")
	}
	f.harness.press(key.NameDownArrow, key.NameDownArrow, key.NameReturn)
	if f.selected != 2 || f.changes != 1 {
		t.Fatalf("Down, Down, Return chose %d after %d changes, want 2 after 1", f.selected, f.changes)
	}
	if f.picker.Open() {
		t.Error("choosing left the picker open")
	}
	if !f.harness.focused(&f.picker.button.Clickable) {
		t.Error("choosing did not return focus to the anchor")
	}
}

func TestPickerKeyboardStartsOnCurrentChoiceAndWraps(t *testing.T) {
	f := newPickerFixture(t, 3)
	f.harness.press(key.NameSpace)
	if !f.picker.Open() {
		t.Fatal("Space on the anchor did not open the picker")
	}
	// from the current choice, the last option, Down wraps to the first
	f.harness.press(key.NameDownArrow, key.NameReturn)
	if f.selected != 0 {
		t.Fatalf("Down from the current last choice then Return chose %d, want 0", f.selected)
	}
	f.harness.press(key.NameReturn)
	f.harness.press(key.NameEnd, key.NameEnter)
	if f.selected != 3 {
		t.Fatalf("End then Enter chose %d, want 3", f.selected)
	}
}

func TestPickerEscapeKeepsTheValue(t *testing.T) {
	f := newPickerFixture(t, 1)
	f.harness.press(key.NameReturn)
	if !f.picker.Open() {
		t.Fatal("Return on the anchor did not open the picker")
	}
	f.harness.press(key.NameDownArrow, key.NameEscape)
	if f.selected != 1 || f.changes != 0 {
		t.Fatalf("Escape changed the value to %d", f.selected)
	}
	if f.picker.Open() {
		t.Error("Escape left the picker open")
	}
	if !f.harness.focused(&f.picker.button.Clickable) {
		t.Error("Escape did not return focus to the anchor")
	}
}

func TestMenuKeyboardSkipsDisabledItemsAndDividers(t *testing.T) {
	th := theme.New(palette.Signature(true))
	items := []MenuItem{
		{Label: "Open ubuntu-24.04", Disabled: true},
		{Label: "View activity"},
		{Divider: true},
		{Label: "GPU passthrough unavailable", Disabled: true},
		{Label: "Stop scratch-arch", Danger: true},
	}
	var popover Popover
	var anchor Plaque
	var menu Menu
	chosen := -1
	harness := newKeyHarness(t, func(gtx layout.Context) {
		popover.Opener = &anchor.Clickable
		popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			anchor.Label = "Machine actions"
			return anchor.Layout(gtx, th)
		}, func(gtx layout.Context) layout.Dimensions {
			return menu.Layout(gtx, th, items)
		})
		if index, ok := menu.Clicked(gtx); ok {
			chosen = index
			popover.Close(gtx)
		}
	})
	harness.settle()
	popover.Open = true
	harness.settle()
	if !harness.focused(&menu.clicks[1].Clickable) {
		t.Fatal("an open menu did not start on its first enabled item")
	}
	harness.press(key.NameDownArrow)
	if !harness.focused(&menu.clicks[4].Clickable) {
		t.Fatal("Down did not skip the divider and the disabled item")
	}
	harness.press(key.NameDownArrow)
	if !harness.focused(&menu.clicks[1].Clickable) {
		t.Fatal("Down from the last item did not wrap past the disabled first item")
	}
	harness.press(key.NameUpArrow, key.NameReturn)
	if chosen != 4 {
		t.Fatalf("Up then Return chose %d, want 4", chosen)
	}
	if popover.Open || !harness.focused(&anchor.Clickable) {
		t.Error("choosing did not close the popover and return focus to its opener")
	}
}

func TestMenuMove(t *testing.T) {
	// 0 enabled, 1 disabled, 2 divider, 3 enabled, 4 enabled
	enabled := func(index int) bool { return index == 0 || index == 3 || index == 4 }
	cases := []struct {
		from int
		name key.Name
		to   int
		ok   bool
	}{
		{0, key.NameDownArrow, 3, true},
		{4, key.NameDownArrow, 0, true},
		{0, key.NameUpArrow, 4, true},
		{3, key.NameUpArrow, 0, true},
		{3, key.NameHome, 0, true},
		{0, key.NameEnd, 4, true},
		{0, key.NameLeftArrow, 0, false},
	}
	for _, test := range cases {
		to, ok := menuMove(5, enabled, test.from, test.name)
		if to != test.to || ok != test.ok {
			t.Errorf("menuMove(from %d, %v) = %d %v, want %d %v", test.from, test.name, to, ok, test.to, test.ok)
		}
	}
	if _, ok := menuMove(3, func(int) bool { return false }, 0, key.NameDownArrow); ok {
		t.Error("a menu with nothing enabled moved")
	}
	if start, ok := menuStart(5, enabled, 1); !ok || start != 0 {
		t.Errorf("menuStart from a disabled item = %d %v, want the first enabled item", start, ok)
	}
	if start, ok := menuStart(5, enabled, 3); !ok || start != 3 {
		t.Errorf("menuStart on an enabled item = %d %v, want 3", start, ok)
	}
}
