package look

import (
	"image"
	"image/color"
	"strings"
	"unicode"
	"unicode/utf8"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"

	"ryoku-rashin/internal/app/look/logos"
)

// whose logo: a provider's or harness's id, and its name for the initial drawn where lobe-icons has no mark
type Mark struct {
	ID, Name string
}

// a mark size square in a role ink, never its brand colour: fg2 at rest, fg raised or chosen
func Logo(gtx layout.Context, th *Theme, mark Mark, size unit.Dp, ink color.NRGBA) layout.Dimensions {
	pixels := gtx.Dp(size)
	box := image.Pt(pixels, pixels)
	if !gtx.Source.Enabled() {
		ink = palette.WithAlpha(ink, tokens.AlphaDisabled)
	}
	if drawn, ok := logos.Image(mark.ID, pixels, ink); ok {
		area := clip.Rect{Max: box}.Push(gtx.Ops)
		drawn.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
		area.Pop()
		return layout.Dimensions{Size: box}
	}
	initial := monogram(mark)
	if initial == "" {
		return layout.Dimensions{Size: box}
	}
	// a capital in the row's heavier face stands about as tall and dark as a mark
	style := th.T.Row
	style.Size, style.Line = unit.Sp(float32(size)*0.9), unit.Sp(size)
	record := op.Record(gtx.Ops)
	child := gtx
	child.Constraints = layout.Constraints{Max: box}
	said := kit.RawText(th.Theme, style, ink, initial)(child)
	call := record.Stop()
	offset := op.Offset(image.Pt((pixels-said.Size.X)/2, (pixels-said.Size.Y)/2)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: box}
}

func monogram(mark Mark) string {
	for _, said := range []string{mark.Name, mark.ID} {
		said = strings.TrimSpace(said)
		if letter, _ := utf8.DecodeRuneInString(said); said != "" && letter != utf8.RuneError {
			return string(unicode.ToUpper(letter))
		}
	}
	return ""
}
