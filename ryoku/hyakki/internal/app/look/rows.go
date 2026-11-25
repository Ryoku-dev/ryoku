package look

import (
	"image"
	"image/color"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// RowWash paints what the Hub puts under a row: the selection wash when it is
// the open one, else the hover wash by hover with the press wash taking over
// by press. The accent never fills a row.
func RowWash(gtx layout.Context, th *Theme, bounds image.Rectangle, hover, press float32, selected bool) {
	cut := gtx.Dp(th.Cut.Small)
	if selected {
		kit.Fill(gtx, th.Theme, bounds, cut, th.R.Select())
		if press > 0 {
			kit.Fill(gtx, th.Theme, bounds, cut, palette.WithAlpha(th.R.Press(), min(max(press, 0), 1)*.16))
		}
		return
	}
	wash := palette.Mix(palette.WithAlpha(th.R.Hover(), min(max(hover, 0), 1)), th.R.Press(), min(max(press, 0), 1))
	if wash.A > 0 {
		kit.Fill(gtx, th.Theme, bounds, cut, wash)
	}
}

// RowInk is the ink of a row's title and of its quiet second line, on the
// selection wash or off it.
func RowInk(th *Theme, selected bool) (title, quiet color.NRGBA) {
	if !selected {
		return th.R.Fg, th.R.Fg3
	}
	title = th.R.OnContainer
	if title.A == 0 {
		title = th.R.Fg
	}
	quiet = th.R.Fg3
	// an opaque container from the desktop's palette takes a quiet ink of its own
	if wash := th.R.Select(); wash.A == 255 {
		quiet = palette.Mix(title, wash, tokens.AlphaOnSel2)
	}
	return title, quiet
}

// SelectedText is the plane under marked text in an editor or a selectable:
// the accent at 60 %, solved over the pane so it reads as it does in the Hub.
func SelectedText(th *Theme) color.NRGBA {
	return palette.SRGBAlpha(th.R.Sel, th.Planes().Pane, .6)
}
