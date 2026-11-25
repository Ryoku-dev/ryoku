package kit

import (
	"image"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
)

// Folio geometry (Folio.qml:31-65): the number in mono at 168 with tracking -6, the kanji in
// the title face at 96, 18 to its left and 0.12 of the number's line above its foot.
const (
	folioSize      = 168
	folioTracking  = -6
	folioGlossSize = 96
	folioGlossGap  = 18
)

// Folio is the page's folio as the Hub sets it (components/sheet/Folio.qml), read by no style:
// the page's number huge and faint in the sheet's lower right corner, behind the page, so a
// short page's empty foot still reads as part of the sheet, with the page's kanji beside it
// while glosses are on. A new number rolls in over the old one: 480 ms (twice th.Motion.Move)
// on OutCubic, travelling half the number's height; under reduced motion it swaps at once.
//
// The number's label stands 0.8 of its height above the foot, so its figures sit on the
// foot. Qt keeps a label at least as tall as its glyphs whatever the fixed line height (the
// mono role's 16 becomes 202 at 168), so the height here is the glyphs'. Ink is fg at .05
// dark and .07 light.
type Folio struct {
	shown   string
	leaving string
	start   time.Time
}

// Layout fills and clips to gtx.Constraints.Max. number is the page's number ("01.3"),
// gloss its kanji, empty while glosses are off. At rest it allocates nothing and asks for no
// frame.
func (folio *Folio) Layout(gtx layout.Context, th *theme.Theme, number, gloss string) layout.Dimensions {
	size := gtx.Constraints.Max
	if number != folio.shown {
		if anim.Reduced() || folio.shown == "" {
			folio.leaving, folio.start = "", time.Time{}
		} else {
			folio.leaving, folio.start = folio.shown, gtx.Now
		}
		folio.shown = number
	}
	travel := float32(0)
	if !folio.start.IsZero() {
		roll := 2 * th.Motion.Move
		elapsed := gtx.Now.Sub(folio.start)
		if anim.Reduced() || roll <= 0 || elapsed >= roll {
			folio.leaving, folio.start = "", time.Time{}
		} else {
			travel = 1 - anim.EaseOutCubic(float32(elapsed)/float32(roll))
			gtx.Execute(op.InvalidateCmd{})
		}
	}

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	style := th.T.Mono
	style.Size = folioSize
	share := folioShare(th.R)

	if folio.shown != "" {
		run := trackedLine(gtx, th, style, folioTracking, folio.shown, false)
		height := float32(run.lineHeight(gtx, style))
		y := float32(size.Y) - height*0.8 + travel*height*0.5
		at := palette.SRGBAlpha(th.R.Fg, th.R.Ground, share*(1-travel))
		run.paint(gtx, image.Pt(size.X-run.width, folioRound(y)), at)
		if gloss != "" {
			kanji := th.T.Title
			kanji.Size = folioGlossSize
			mark := trackedLine(gtx, th, kanji, 0, gloss, false)
			bottom := y + height - height*0.12
			x := size.X - run.width - gtx.Dp(folioGlossGap) - mark.width
			mark.paint(gtx, image.Pt(x, folioRound(bottom-float32(mark.lineHeight(gtx, kanji)))), at)
		}
	}
	if folio.leaving != "" {
		run := trackedLine(gtx, th, style, folioTracking, folio.leaving, false)
		height := float32(run.lineHeight(gtx, style))
		y := float32(size.Y) - height*0.8 - (1-travel)*height*0.5
		run.paint(gtx, image.Pt(size.X-run.width, folioRound(y)), palette.SRGBAlpha(th.R.Fg, th.R.Ground, share*travel))
	}
	return layout.Dimensions{Size: size}
}

// folioShare is the folio's alpha of fg, .05 dark and .07 light (Folio.qml:64), which
// palette.SRGBAlpha solves over the ground so it reads as faint as in the Hub.
func folioShare(r palette.Roles) float32 {
	if r.Dark {
		return 0.05
	}
	return 0.07
}

// folioRound rounds a fractional place to the nearest pixel.
func folioRound(v float32) int {
	if v < 0 {
		return -int(-v + 0.5)
	}
	return int(v + 0.5)
}
