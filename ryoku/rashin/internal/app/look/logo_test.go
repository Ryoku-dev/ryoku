package look

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

func TestLogoKeepsItsBoxWithOrWithoutAMark(t *testing.T) {
	th := New(palette.Dark, palette.Look{})
	for _, mark := range []Mark{{ID: "openai", Name: "OpenAI"}, {ID: "ovh", Name: "OVHcloud"}, {ID: "x"}, {}} {
		gtx := layout.Context{Ops: new(op.Ops), Metric: unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}, Constraints: layout.Exact(image.Pt(400, 400))}
		if got := Logo(gtx, th, mark, 20, th.R.Fg2).Size; got != image.Pt(30, 30) {
			t.Errorf("%+v takes %v, want 30x30", mark, got)
		}
	}
	for mark, want := range map[Mark]string{{ID: "ovh", Name: "OVHcloud"}: "O", {ID: "llamacpp", Name: " llama.cpp"}: "L", {ID: "omp"}: "O", {}: ""} {
		if got := monogram(mark); got != want {
			t.Errorf("monogram(%+v) = %q, want %q", mark, got, want)
		}
	}
}
