package shell

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func TestRouterRailSwitchesTabsThroughInputRouter(t *testing.T) {
	var operations op.Ops
	var inputRouter input.Router
	router := NewRouter(protocol.TabAsk, "", nil)
	theme := look.New(0)
	now := time.Unix(100, 0)
	frame := func() {
		operations.Reset()
		gtx := layout.Context{Ops: &operations, Source: inputRouter.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(88, 400))}
		router.LayoutRail(gtx, theme, 2)
		inputRouter.Frame(&operations)
		now = now.Add(time.Millisecond)
	}
	frame()
	point := f32.Pt(44, 72)
	inputRouter.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: point})
	frame()
	inputRouter.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: point})
	frame()
	frame()
	if router.Tab != protocol.TabAgent {
		t.Fatalf("tab = %q, want %q", router.Tab, protocol.TabAgent)
	}
}

func TestRouterRailSwitchesToProjectsAndBack(t *testing.T) {
	var operations op.Ops
	var inputRouter input.Router
	router := NewRouter(protocol.TabAsk, "", nil)
	theme := look.New(0)
	now := time.Unix(150, 0)
	frame := func() {
		operations.Reset()
		gtx := layout.Context{Ops: &operations, Source: inputRouter.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(88, 480))}
		router.LayoutRail(gtx, theme, 2)
		inputRouter.Frame(&operations)
		now = now.Add(time.Millisecond)
	}
	click := func(point f32.Point) {
		inputRouter.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: point})
		frame()
		inputRouter.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: point})
		frame()
		frame()
	}
	frame()
	click(f32.Pt(44, 120))
	if router.Tab != protocol.TabProjects {
		t.Fatalf("tab = %q, want %q", router.Tab, protocol.TabProjects)
	}
	click(f32.Pt(44, 24))
	if router.Tab != protocol.TabAsk {
		t.Fatalf("tab = %q, want %q", router.Tab, protocol.TabAsk)
	}
}

func TestRouterEscapeClosesPalette(t *testing.T) {
	var operations op.Ops
	var inputRouter input.Router
	router := NewRouter(protocol.TabAsk, "", nil)
	overlays := new(look.Overlays)
	now := time.Unix(200, 0)
	frame := func() {
		operations.Reset()
		gtx := layout.Context{Ops: &operations, Source: inputRouter.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(800, 600))}
		router.HandleKeys(gtx, overlays)
		inputRouter.Frame(&operations)
		now = now.Add(time.Millisecond)
	}
	frame()
	inputRouter.Queue(key.Event{Name: "K", Modifiers: key.ModShortcut, State: key.Press})
	frame()
	if !router.PaletteOpen {
		t.Fatal("Ctrl+K did not open the command palette")
	}
	inputRouter.Queue(key.Event{Name: key.NameEscape, State: key.Press})
	frame()
	if router.PaletteOpen {
		t.Fatal("Escape did not close the command palette")
	}
}

func TestNormalizeProjectViewDefaultsToOverview(t *testing.T) {
	if got := normalizeProjectView(""); got != "overview" {
		t.Fatalf("normalizeProjectView(\"\") = %q, want overview", got)
	}
	if got := normalizeProjectView("changes"); got != "changes" {
		t.Fatalf("normalizeProjectView(\"changes\") = %q, want changes", got)
	}
}
