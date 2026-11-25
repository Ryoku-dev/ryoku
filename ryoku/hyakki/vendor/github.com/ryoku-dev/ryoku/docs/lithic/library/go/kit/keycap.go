package kit

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Keycap returns one key's face on its plate, as ChordField draws a recorded
// shortcut: the face in label on a field plate with an edge hairline, the
// token cut, 24 tall at the default type size. A round design turns the plate
// into its own corner. Use it wherever a key is named: a shortcut list, a
// hint beside a command. The face is shown as given; ChordFaces gives the
// faces of a stored chord.
func Keycap(th *theme.Theme, face string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		size, _ := layoutKeycap(gtx, th, face, 0)
		return layout.Dimensions{Size: size}
	}
}

// Keycaps returns a chord's faces as keycaps on one line, s1 apart, in the
// order given. A face that does not fit the width ends the line; the rest are
// left out rather than clipped.
func Keycaps(th *theme.Theme, faces ...string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layoutKeycaps(gtx, th, faces)
	}
}

func layoutKeycaps(gtx layout.Context, th *theme.Theme, faces []string) layout.Dimensions {
	gap := gtx.Dp(th.Sp.S1)
	x, height := 0, 0
	for index, face := range faces {
		at := x
		if index > 0 {
			at += gap
		}
		size, fits := layoutKeycap(gtx, th, face, at)
		height = size.Y
		if !fits {
			break
		}
		x = at + size.X
	}
	return layout.Dimensions{Size: image.Pt(x, height)}
}

// layoutKeycap draws one keycap at x and returns its size. It draws nothing
// and reports false when the cap would pass the width it was given.
func layoutKeycap(gtx layout.Context, th *theme.Theme, face string, x int) (image.Point, bool) {
	padX, padY := gtx.Dp(th.Sp.S2), gtx.Dp(th.Sp.S1)
	line := gtx.Sp(th.T.Label.Line)
	height := line + 2*padY
	record := op.Record(gtx.Ops)
	textGTX := gtx
	// measured whole, so a face that does not fit is left out, never cut short
	textGTX.Constraints = layout.Constraints{Max: image.Pt(1<<16, line)}
	text := rawText(th, th.T.Label, disabledInk(gtx, th, th.R.Fg), face)(textGTX)
	call := record.Stop()
	width := text.Size.X + 2*padX
	if x+width > gtx.Constraints.Max.X {
		return image.Pt(width, height), false
	}
	bounds := image.Rect(x, 0, x+width, height)
	cut := gtx.Dp(th.Cut.Token)
	Fill(gtx, th, bounds, cut, disabledInk(gtx, th, th.R.Field))
	Stroke(gtx, th, bounds, cut, max(1, gtx.Dp(unit.Dp(tokens.SizeHairline))), disabledInk(gtx, th, th.R.Edge))
	offset := op.Offset(image.Pt(x+padX, padY)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return image.Pt(width, height), true
}
