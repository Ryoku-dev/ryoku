package look

import (
	"image"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Head is the ruled head of a page's part, as the Hub heads its sections: the
// title, a vein rule and a mark at the far end. Number and Of give the part's
// place among the page's parts ("02 / 05"); a board's group gives its Count
// instead ("03").
type Head struct {
	Title      string
	Number, Of int
	Count      int
}

// kept so a head at rest builds no string
var ordinals = map[[2]int]string{}

func (head Head) style() kit.SectionStyle {
	style := kit.SectionStyle{Title: head.Title, Ruled: true}
	switch {
	case head.Number > 0 && head.Of > 0:
		key := [2]int{head.Number, head.Of}
		mark, ok := ordinals[key]
		if !ok {
			mark = kit.Ordinal(head.Number, head.Of)
			ordinals[key] = mark
		}
		style.Mark = mark
	case head.Count > 0:
		style.Mark = kit.Pad2(head.Count)
	}
	return style
}

// SectionHead is the head alone, across the width it is given, for a part
// whose body the page places itself.
func SectionHead(th *Theme, head Head) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		// the kit lays a section out whole: measured free, then the gaps it keeps for a body are taken back
		free := gtx
		free.Constraints.Min = image.Point{}
		free.Constraints.Max.Y = 1 << 20
		dimensions := head.style().Layout(free, th.Theme, nil)
		height := dimensions.Size.Y - gtx.Dp(tokens.SizeSlabTitleGap) - gtx.Dp(tokens.SizeSlabBelow)
		return layout.Dimensions{Size: image.Pt(dimensions.Size.X, max(0, height)), Baseline: dimensions.Baseline}
	}
}

// Section is a part of a page: its head, the body under it and the gap a
// section keeps below.
func Section(gtx layout.Context, th *Theme, head Head, body layout.Widget) layout.Dimensions {
	return head.style().Layout(gtx, th.Theme, body)
}
