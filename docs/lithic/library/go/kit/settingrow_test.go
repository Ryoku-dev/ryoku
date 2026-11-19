package kit

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
)

func TestSettingRowStackedGeometry(t *testing.T) {
	for _, width := range []int{320, 600, 960} {
		for _, changed := range []bool{false, true} {
			row := SettingRow{Stacked: true, Changed: changed}
			got := row.positions(width, 40, image.Pt(width, 120), 44, 32, 8, 13)
			if got.height != 173 || got.text != (image.Point{}) || got.control != image.Pt(0, 53) || got.reset != image.Pt(width-32, 0) {
				t.Errorf("width=%d changed=%v: geometry %+v", width, changed, got)
			}
		}
	}
	row := SettingRow{Stacked: true, Changed: true}
	if got := row.positions(320, 20, image.Pt(320, 80), 44, 32, 8, 13); got.control.Y != 45 || got.height != 125 {
		t.Errorf("short title must reserve reset height: %+v", got)
	}
}

func TestSettingRowExplicitTopAlignment(t *testing.T) {
	row := SettingRow{TopAligned: true}
	got := row.positions(600, 40, image.Pt(320, 120), 44, 32, 8, 13)
	if got.height != 120 || got.text.Y != 0 || got.control != image.Pt(280, 0) || got.reset != image.Pt(240, 0) {
		t.Errorf("top-aligned geometry %+v", got)
	}
	row.TopAligned = false
	got = row.positions(600, 40, image.Pt(320, 120), 44, 32, 8, 13)
	if got.text.Y != 40 || got.control.Y != 0 || got.reset.Y != 44 {
		t.Errorf("ordinary controls must retain centring: %+v", got)
	}
}

func TestSettingRowStackedControlConstraints(t *testing.T) {
	th := partsTh()
	for _, width := range []int{320, 600, 960} {
		var router input.Router
		var ops op.Ops
		gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(width, 1000))
		row := SettingRow{Title: "Highlight actions", Detail: "Choose their order", Stacked: true, First: true, Changed: true}
		dims := row.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
			if gtx.Constraints.Min.X != width || gtx.Constraints.Max.X != width {
				t.Errorf("width=%d: stacked constraints %v", width, gtx.Constraints)
			}
			return layout.Dimensions{Size: image.Pt(width, 120)}
		})
		if dims.Size.X != width || dims.Size.Y <= 120+13 {
			t.Errorf("width=%d: stacked dimensions %v", width, dims)
		}
	}
}
