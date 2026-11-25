package teamsui

import (
	"image"
	"strings"
	"unicode"

	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/app/look"
)

// a mark from markLarge takes the title face and the wider cut, as a team's does on its tile;
// one under markSmall, a face in a crew strip, takes the token cut so its letters keep their room
const (
	markLarge unit.Dp = 40
	markSmall unit.Dp = 28
)

// the lead's ring, wide enough to read on a mark of two letters
const markRing unit.Dp = 1.5

// up to two letters that stand for a name: the first of its first two words
func Initials(name string) string {
	var letters []rune
	for _, word := range strings.Fields(name) {
		for _, r := range word {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters = append(letters, unicode.ToUpper(r))
				break
			}
		}
		if len(letters) == 2 {
			break
		}
	}
	return string(letters)
}

// a team's or a member's mark: its initials on the field plane in the design's corner, cut in
// Lithic and round elsewhere; the lead wears a ring in fg2 so a crew reads from the top
func Mark(th *look.Theme, name string, size unit.Dp, lead bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		side := gtx.Dp(size)
		bounds := image.Rect(0, 0, side, side)
		style, cut := th.T.Label, gtx.Dp(th.Cut.Small)
		switch {
		case size >= markLarge:
			style, cut = th.T.Title, gtx.Dp(th.Cut.Mid)
		case size < markSmall:
			cut = gtx.Dp(th.Cut.Token)
		}
		kit.Fill(gtx, th.Theme, bounds, cut, th.R.Field)
		if lead {
			kit.Stroke(gtx, th.Theme, bounds, cut, max(1, gtx.Dp(markRing)), th.R.Fg2)
		}
		gtx.Constraints = layout.Exact(bounds.Size())
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: style, Color: th.R.Fg, Align: text.Middle, MaxLines: 1}.Layout(gtx, th.Theme, Initials(name))
		})
		return layout.Dimensions{Size: bounds.Size()}
	}
}
