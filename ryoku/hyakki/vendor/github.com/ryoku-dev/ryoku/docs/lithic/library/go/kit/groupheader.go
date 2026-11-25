package kit

import (
	"strconv"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// GroupHeader lays out a populated fleet group title and count.
func GroupHeader(th *theme.Theme, title string, count int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		if count <= 0 {
			return layout.Dimensions{}
		}
		return layout.Inset{
			Top: tokens.SizeGroupHeaderTop, Right: tokens.SizeGroupHeaderRight,
			Bottom: tokens.SizeGroupHeaderBottom, Left: tokens.SizeGroupHeaderLeft,
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, Text(th, th.T.Group, th.R.Fg3, title)),
				layout.Rigid(Text(th, th.T.Caption, th.R.Fg3, strconv.Itoa(count))),
			)
		})
	}
}
