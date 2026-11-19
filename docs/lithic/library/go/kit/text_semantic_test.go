package kit

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

func TestOneLineTextBoundsSemanticViewport(t *testing.T) {
	th := theme.New(palette.Signature(true))
	for _, test := range []struct {
		name string
		draw layout.Widget
	}{
		{"Text", Text(th, th.T.Row, th.R.Fg, "Model")},
		{"RawText", RawText(th, th.T.Row, th.R.Fg, "Model")},
		{"Label", func(gtx layout.Context) layout.Dimensions {
			return Label{Style: th.T.Row, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th, "Model")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var ops op.Ops
			gtx := layout.Context{
				Ops: &ops, Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
				Constraints: layout.Constraints{Max: image.Pt(706, 1<<20)},
			}
			test.draw(gtx)
			var router input.Router
			router.Frame(&ops)
			found := false
			var visit func(input.SemanticNode)
			visit = func(node input.SemanticNode) {
				if node.Desc.Label == "Model" {
					found = true
					if height := node.Desc.Bounds.Dy(); height > 2*gtx.Sp(th.T.Row.Size) {
						t.Errorf("semantic label height = %d, want one glyph-safe line", height)
					}
				}
				for _, child := range node.Children {
					visit(child)
				}
			}
			visit(router.AppendSemantics(nil)[0])
			if !found {
				t.Error("text lost its screen-reader label")
			}
		})
	}
}
