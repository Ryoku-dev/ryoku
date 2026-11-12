package kit

import (
	"image"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

func factsContext(ops *op.Ops, width int) layout.Context {
	return layout.Context{
		Ops:         ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(width, 4000)},
	}
}

// measureFactsText lays out one side the way Facts does: wrapping, at a fixed width.
func measureFactsText(th *theme.Theme, style theme.TextStyle, width int, value string) int {
	var ops op.Ops
	gtx := factsContext(&ops, width)
	gtx.Constraints = layout.Constraints{Max: image.Pt(width, 1<<24)}
	return Label{Style: style, Color: th.R.Fg}.Layout(gtx, th, value).Size.Y
}

func TestFactsRowHeight(t *testing.T) {
	th := theme.New(palette.Signature(true))
	const width = 600
	labelWidth := int(tokens.SizeFactsLabelWidth)
	valueWidth := width - labelWidth - int(th.Sp.S4)
	line := measureFactsText(th, th.T.Body, labelWidth, "Server")
	longLabel := "Ryoku shortcuts left as they were before the import"
	labelHeight := measureFactsText(th, th.T.Body, labelWidth, longLabel)
	if labelHeight <= tokens.SizeFactsRowHeight || labelHeight < 2*line {
		t.Fatalf("the long label should wrap past one row: %d high, line %d", labelHeight, line)
	}
	longValue := strings.Repeat("/var/lib/ryoport/snapshots/ubuntu-24.04", 8)
	monoLine := measureFactsText(th, th.T.Mono, valueWidth, "ally")
	valueHeight := measureFactsText(th, th.T.Mono, valueWidth, longValue)
	if valueHeight <= tokens.SizeFactsRowHeight || valueHeight < 2*monoLine {
		t.Fatalf("the long value should wrap past one row: %d high, line %d", valueHeight, monoLine)
	}

	cases := []struct {
		name  string
		facts []Fact
		want  int
	}{
		{"short rows keep 44", []Fact{{Label: "Server", Value: "ally"}, {Label: "Address", Value: "192.0.2.18", Mono: true}}, 2 * tokens.SizeFactsRowHeight},
		{"a long label grows its row", []Fact{{Label: "Server", Value: "ally"}, {Label: longLabel, Value: "1"}}, tokens.SizeFactsRowHeight + labelHeight},
		{"a long value grows its row", []Fact{{Label: "Backup", Value: longValue, Mono: true}}, max(tokens.SizeFactsRowHeight, valueHeight)},
		{"an absent value still takes a row", []Fact{{Label: "Last error"}}, tokens.SizeFactsRowHeight},
	}
	for _, test := range cases {
		var ops op.Ops
		got := Facts(th, test.facts)(factsContext(&ops, width)).Size
		if got.Y != test.want || got.X != width {
			t.Errorf("%s: Facts is %v, want %dx%d", test.name, got, width, test.want)
		}
	}
}
