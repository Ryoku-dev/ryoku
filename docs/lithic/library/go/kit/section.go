package kit

import (
	"image"

	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Section places a section title above its body and reserves its trailing gap.
func Section(th *theme.Theme, title string, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return SectionStyle{Title: title}.Layout(gtx, th, body)
	}
}

// SectionStyle is a section with Section.qml's options. A ruled head draws a vein hairline
// from s3 after the title to s3 before the mark (or to the section's far edge), and Mark (an
// ordinal such as Ordinal(1, 8), a count) stands at the far end in the small mono, fg3: the
// head of a sheet's part, as the Hub numbers its sections (SettingsPage.qml:770-772).
// A ruled or marked section fills the width it is given; a plain one is as wide as its
// widest child, as before. The head allocates nothing at rest.
type SectionStyle struct {
	Title string
	Ruled bool
	Mark  string
}

// Layout draws the head, then body s3 below it (slabTitleGap) and slabBelow after it.
func (section SectionStyle) Layout(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	head, _ := section.head(gtx, th)
	y := head.Size.Y + gtx.Dp(tokens.SizeSlabTitleGap)
	width := head.Size.X
	if body != nil {
		child := gtx
		child.Constraints.Min.Y = 0
		child.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-y)
		offset := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dimensions := body(child)
		offset.Pop()
		y += dimensions.Size.Y
		width = max(width, dimensions.Size.X)
	}
	y += gtx.Dp(tokens.SizeSlabBelow)
	return layout.Dimensions{Size: gtx.Constraints.Constrain(image.Pt(width, y))}
}

// head draws the title, the mark and the rule (Section.qml:31-54), and returns the head's
// size and the rule it drew (empty when none). A plain head is the one-line Text it always
// was, eliding at the width given.
func (section SectionStyle) head(gtx layout.Context, th *theme.Theme) (layout.Dimensions, image.Rectangle) {
	gtx.Constraints.Min.Y = 0
	if !section.Ruled && section.Mark == "" {
		return Text(th, th.T.Section, th.R.Fg, section.Title)(gtx), image.Rectangle{}
	}
	// the title is measured at its own width: a caller that stretches its children (a
	// vertical Flex under Min.X = Max.X) would otherwise make the title as wide as the head
	// and leave the rule no room
	titleGtx := gtx
	titleGtx.Constraints.Min = image.Point{}
	title := layoutTracked(titleGtx, th, th.T.Section, disabledInk(gtx, th, th.R.Fg), 0, section.Title, false)
	if section.Title != "" {
		area := clip.Rect(image.Rectangle{Max: title.Size}).Push(gtx.Ops)
		semantic.LabelOp(section.Title).Add(gtx.Ops)
		area.Pop()
	}
	width := max(gtx.Constraints.Max.X, title.Size.X)
	end := width
	if section.Mark != "" {
		mark := trackedLine(gtx, th, th.T.MonoSm, 0, section.Mark, false)
		end = width - mark.width
		mastheadPaint(gtx, mark, th.T.MonoSm, disabledInk(gtx, th, th.R.Fg3), end, title.Size.Y/2)
		end -= gtx.Dp(th.Sp.S3)
	}
	var rule image.Rectangle
	if section.Ruled && section.Title != "" {
		start := title.Size.X + gtx.Dp(th.Sp.S3)
		hair := max(1, gtx.Dp(tokens.SizeHairline))
		top := (title.Size.Y - hair) / 2
		if end > start {
			rule = image.Rect(start, top, end, top+hair)
			paint.FillShape(gtx.Ops, th.R.Vein, clip.Rect(rule).Op())
		}
	}
	return layout.Dimensions{Size: image.Pt(width, title.Size.Y), Baseline: title.Baseline}, rule
}
