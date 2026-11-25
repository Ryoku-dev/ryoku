package kit

import (
	"image/color"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Span is one run in a Sentence.
type Span struct {
	Text  string
	Mono  bool
	Color color.NRGBA
}

// Sentence composes text runs, each in the sentence's type or mono, and in the
// sentence's ink unless the run carries its own.
func Sentence(gtx layout.Context, th *theme.Theme, style theme.TextStyle, ink color.NRGBA, spans []Span) layout.Dimensions {
	if len(spans) == 0 {
		return layout.Dimensions{}
	}
	children := make([]layout.FlexChild, 0, len(spans))
	for _, span := range spans {
		span := span
		spanStyle := style
		if span.Mono {
			spanStyle.Face = th.T.Mono.Face
		}
		spanInk := ink
		if span.Color.A != 0 {
			spanInk = span.Color
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return Label{Style: spanStyle, Color: spanInk}.Layout(gtx, th, span.Text)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
}
