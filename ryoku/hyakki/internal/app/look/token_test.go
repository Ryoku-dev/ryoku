package look

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func TestChipLabelsUseMeasuredBounds(t *testing.T) {
	th := NewWithRoles(palette.Signature(true))
	for _, test := range []struct {
		name string
		draw layout.Widget
	}{
		{"token", TokenChip(th, "Light")},
		{"trait", Chip(th, nil, "Tools", palette.Idle)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ops op.Ops
			gtx := layout.Context{
				Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Constraints{Max: image.Pt(706, 1<<20)},
			}
			box := test.draw(gtx).Size
			var router input.Router
			router.Frame(&ops)
			count := 0
			var visit func(input.SemanticNode)
			visit = func(node input.SemanticNode) {
				if node.Desc.Label != "" {
					count++
					if got := node.Desc.Bounds; got != (image.Rectangle{Max: box}) {
						t.Errorf("%q semantic bounds = %v, want chip bounds %v", node.Desc.Label, got, box)
					}
				}
				for _, child := range node.Children {
					visit(child)
				}
			}
			visit(router.AppendSemantics(nil)[0])
			if count != 1 {
				t.Errorf("chip has %d labels, want one", count)
			}
		})
	}
}
