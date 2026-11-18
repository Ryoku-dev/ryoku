package kit

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
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

// menuRing reports whether item index of the picker's menu shows the
// keyboard focus ring.
func (f *pickerFixture) menuRing(index int) bool {
	shown := false
	f.harness.frame(func(gtx layout.Context) {
		shown = f.picker.menu.focus[index].Value(gtx) > 0
	})
	return shown
}

func (f *pickerFixture) click() {
	at := f32.Pt(40, 40)
	f.harness.router.Queue(
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at},
	)
	f.harness.settle()
}

func TestPickerKeyboardRingFollowsTheKeys(t *testing.T) {
	f := newPickerFixture(t, 1)
	f.click()
	if !f.picker.Open() {
		t.Fatal("a click on the anchor did not open the picker")
	}
	if !f.harness.focused(&f.picker.menu.clicks[1].Clickable) {
		t.Fatal("a pointer-opened menu did not start on the current choice")
	}
	if f.menuRing(1) {
		t.Fatal("a pointer-opened menu drew the keyboard ring on its current item")
	}
	f.harness.press(key.NameDownArrow)
	if !f.menuRing(2) || f.menuRing(1) {
		t.Fatal("an arrow key in a pointer-opened menu did not move the ring to the next item")
	}
	f.harness.press(key.NameEscape)
	f.harness.press(key.NameDownArrow)
	if !f.picker.Open() {
		t.Fatal("Down on the anchor did not reopen the picker")
	}
	if !f.menuRing(1) {
		t.Fatal("a menu opened from the keys did not draw the ring on the current choice")
	}
	f.harness.press(key.NameEscape)
	f.click()
	if f.menuRing(1) {
		t.Fatal("a click after a key open drew the keyboard ring")
	}
}

func TestLongPickerLaysOutOnlyVisibleRowsAndRevealsKeyboardFocus(t *testing.T) {
	th := theme.New(palette.Signature(true))
	options := make([]string, 400)
	for index := range options {
		options[index] = fmt.Sprintf("Choice %03d", index)
	}
	var picker Picker
	selected := 350
	harness := newKeyHarness(t, func(gtx layout.Context) {
		picker.Layout(gtx, th, "Long choices", options, selected)
		if index, ok := picker.Changed(gtx); ok {
			selected = index
		}
	})
	harness.settle()
	harness.frame(func(gtx layout.Context) { picker.Focus(gtx) })
	harness.press(key.NameDownArrow)
	assertVisible := func(index int) {
		t.Helper()
		if !harness.focused(&picker.menu.clicks[index].Clickable) {
			t.Fatalf("choice %d does not hold keyboard focus", index)
		}
		position := picker.menu.list.Position
		top := picker.menu.offsets[position.First] + position.Offset
		if picker.menu.offsets[index] < top || picker.menu.offsets[index+1] > top+picker.menu.viewport {
			t.Fatalf("focused choice %d is outside the viewport: position=%+v height=%d", index, position, picker.menu.viewport)
		}
		if position.Count > 12 {
			t.Fatalf("400 choices laid out %d visible rows", position.Count)
		}
		shown := make(map[string]struct{})
		for _, node := range harness.router.AppendSemantics(nil) {
			if node.Desc.Class == semantic.Button && len(node.Desc.Label) >= len("Choice ") && node.Desc.Label[:len("Choice ")] == "Choice " {
				shown[node.Desc.Label] = struct{}{}
			}
		}
		if len(shown) > 14 {
			t.Fatalf("400 choices registered %d row labels, want only the viewport and overscan", len(shown))
		}
	}
	if got, want := picker.popover.contentRect.Dy(), int(tokens.SizeMenuVisibleItemsCount*tokens.SizeMenuItem+2*tokens.SizeMenuPaddingVertical); got != want {
		t.Fatalf("menu height = %d, want %d", got, want)
	}
	assertVisible(350)
	harness.press(key.NameHome)
	assertVisible(0)
	harness.press(key.NamePageDown)
	assertVisible(10)
	harness.press(key.NamePageUp)
	assertVisible(0)
	harness.press(key.NameEnd)
	assertVisible(399)
	harness.press(key.NameReturn)
	if selected != 399 || picker.Open() {
		t.Fatalf("End, Return selected %d, open=%v", selected, picker.Open())
	}
}

func TestLongMenuWheelAndThumbKeepTheirOwnViewport(t *testing.T) {
	th := theme.New(palette.Signature(true))
	items := make([]MenuItem, 400)
	for index := range items {
		items[index].Label = fmt.Sprintf("Command %03d", index)
	}
	var popover Popover
	var menu Menu
	harness := newKeyHarness(t, func(gtx layout.Context) {
		popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(200, 40)}
		}, func(gtx layout.Context) layout.Dimensions {
			return menu.Layout(gtx, th, items)
		})
	})
	popover.Open = true
	harness.settle()
	at := f32.Pt(float32(popover.contentRect.Min.X+40), float32(popover.contentRect.Min.Y+80))
	harness.router.Queue(pointer.Event{Kind: pointer.Scroll, Source: pointer.Mouse, Position: at, Scroll: f32.Pt(0, 240)})
	harness.settle()
	if menu.list.Position.First == 0 {
		t.Fatal("the wheel did not scroll the long menu")
	}
	if !harness.focused(&menu.clicks[0].Clickable) {
		t.Fatal("scrolling the focused row out of view lost overlay focus")
	}
	top, height, ok := rowListThumb(menu.viewport, menu.list.Position.Length, menu.list.Position.First, menu.list.Position.Offset, len(items), 12)
	if !ok {
		t.Fatal("the long menu has no scroll thumb")
	}
	x := float32(popover.contentRect.Max.X - 1)
	y := float32(popover.contentRect.Min.Y + int(tokens.SizeMenuPaddingVertical) + top + height/2)
	harness.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, y)})
	harness.settle()
	harness.router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(x, float32(popover.contentRect.Max.Y-10))})
	harness.settle()
	harness.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(x, float32(popover.contentRect.Max.Y-10))})
	harness.settle()
	if menu.list.Position.First < 300 {
		t.Fatalf("dragging the thumb to the bottom stopped at row %d", menu.list.Position.First)
	}
	harness.press(key.NameHome)
	if menu.list.Position.First != 0 || menu.list.Position.Offset != 0 {
		t.Fatalf("Home did not reveal the first row after pointer scrolling: %+v", menu.list.Position)
	}
}

func TestMenuTextKeepsMixedScriptsOnThePrimaryBaseline(t *testing.T) {
	th := theme.New(palette.Signature(true))
	var baselines []int
	harness := newKeyHarness(t, func(gtx layout.Context) {
		gtx.Constraints = layout.Exact(image.Pt(300, 40))
		baselines = baselines[:0]
		for _, label := range []string{"English", "日本語", "العربية", "हिन्दी", "Русский"} {
			dimensions := menuText(th, th.T.Label, th.R.Fg, label)(gtx)
			baselines = append(baselines, dimensions.Size.Y-dimensions.Baseline)
		}
	})
	harness.frame(nil)
	for index, baseline := range baselines {
		if baseline != baselines[0] {
			t.Fatalf("script %d baseline = %d, primary baseline = %d", index, baseline, baselines[0])
		}
	}
}

func TestLongPopoverMeasuresItsMenuWithinTheLargerSide(t *testing.T) {
	th := theme.New(palette.Signature(true))
	items := make([]MenuItem, 400)
	for index := range items {
		items[index].Label = fmt.Sprintf("Command %03d", index)
	}
	var popover Popover
	var menu Menu
	origin := image.Pt(20, 300)
	measuredHeight := 0
	harness := newKeyHarness(t, func(gtx layout.Context) {
		popover.Viewport = image.Rectangle{Max: gtx.Constraints.Max}.Sub(origin)
		offset := op.Offset(origin).Push(gtx.Ops)
		popover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(200, 40)}
		}, func(gtx layout.Context) layout.Dimensions {
			measuredHeight = gtx.Constraints.Max.Y
			return menu.Layout(gtx, th, items)
		})
		offset.Pop()
	})
	popover.Open = true
	harness.settle()
	if measuredHeight != 284 || popover.contentRect.Dy() != 284 {
		t.Fatalf("menu measured at %d and drew %d high, want the larger side's 284 pixels", measuredHeight, popover.contentRect.Dy())
	}
	bounds := popover.contentRect.Add(origin)
	anchor := image.Rect(20, 300, 220, 340)
	if !bounds.In(image.Rect(8, 8, 792, 592)) || bounds.Max.Y != anchor.Min.Y-8 {
		t.Fatalf("shrinking menu %v did not stay above anchor %v inside the window", bounds, anchor)
	}
}
