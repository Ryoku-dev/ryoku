package look

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func TestChoiceUsesCompactIntrinsicAnchor(t *testing.T) {
	var operations op.Ops
	gtx := layout.Context{
		Ops: &operations, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(500, 200)},
	}
	var choice Choice
	dimensions := choice.Layout(gtx, New(palette.Dark), new(Overlays), []string{"Auto", "Fast"}, 0)
	if dimensions.Size.X != tokens.SizePickerWidthMin || dimensions.Size.Y != tokens.SizePickerHeight {
		t.Fatalf("choice size = %v, want %dx%d", dimensions.Size, tokens.SizePickerWidthMin, tokens.SizePickerHeight)
	}
}

func TestSummaryRowHeights(t *testing.T) {
	var operations op.Ops
	gtx := layout.Context{
		Ops: &operations, Now: time.Unix(1, 0), Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(320, 100)},
	}
	row := SummaryRow{Title: "Build graph", State: "Working", Meta: "hyperion"}
	if dimensions := row.Layout(gtx, New(palette.Dark), false); dimensions.Size.Y != 56 {
		t.Fatalf("two-line row height = %d, want 56", dimensions.Size.Y)
	}
	operations.Reset()
	gtx.Ops = &operations
	gtx.Now = gtx.Now.Add(time.Millisecond)
	row.Excerpt = "Reading commit lanes"
	if dimensions := row.Layout(gtx, New(palette.Dark), false); dimensions.Size.Y != 76 {
		t.Fatalf("three-line row height = %d, want 76", dimensions.Size.Y)
	}
}
