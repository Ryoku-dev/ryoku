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

// a team's or a member's mark: its initials on a stone4 slab cut like a tile,
// the lead's ring in fg2 so a crew reads from the top
func Mark(th *look.Theme, name string, size unit.Dp, lead bool) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		side := gtx.Dp(size)
		bounds := image.Rect(0, 0, side, side)
		cut := max(2, side/8)
		if lead {
			kit.FillCut(gtx.Ops, bounds, cut, th.R.Fg2)
			kit.FillCut(gtx.Ops, bounds.Inset(max(1, gtx.Dp(1.5))), cut, th.R.Stone4)
		} else {
			kit.FillCut(gtx.Ops, bounds, cut, th.R.Stone4)
		}
		style := th.T.Label
		if size >= 40 {
			style = th.T.Title
		}
		gtx.Constraints = layout.Exact(bounds.Size())
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.Label{Style: style, Color: th.R.Fg, Align: text.Middle, MaxLines: 1}.Layout(gtx, th.Theme, Initials(name))
		})
		return layout.Dimensions{Size: bounds.Size()}
	}
}
