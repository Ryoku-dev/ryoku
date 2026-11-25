package settings

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"

	"ryoku-rashin/internal/app/look"
)

func TestFoldedSettingsHeaderNamesItsRealInteractiveButton(t *testing.T) {
	var row foldRow
	var router input.Router
	var ops op.Ops
	theme := look.NewWithRoles(palette.Signature(true))
	now := time.Now()
	toggles := 0
	frame := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Now: now, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(700, 90))}
		if row.toggled(gtx) {
			toggles++
		}
		row.layoutHead(gtx, theme, false, foldHead{Title: "Example provider", Detail: "Connection details"})
		router.Frame(&ops)
		now = now.Add(16 * time.Millisecond)
	}
	frame()
	var button image.Rectangle
	for _, node := range router.AppendSemantics(nil) {
		if node.Desc.Class == semantic.Button && node.Desc.Label == "Example provider" && node.Desc.Gestures&input.ClickGesture != 0 {
			button = node.Desc.Bounds
			break
		}
	}
	if button.Empty() {
		t.Fatal("the visible fold title has no named interactive button for assistive/input consumers")
	}
	at := f32.Pt(float32(button.Min.X+button.Max.X)/2, float32(button.Min.Y+button.Max.Y)/2)
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: at}, pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: at})
	frame()
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: at})
	frame()
	frame()
	if toggles != 1 {
		t.Fatalf("the named button did not deliver exactly one real header toggle: %d", toggles)
	}
}
