package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Section places a section title above its body and reserves its trailing gap.
func Section(th *theme.Theme, title string, body layout.Widget) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(Text(th, th.T.Section, th.R.Fg, title)),
			layout.Rigid(layout.Spacer{Height: tokens.SizeSlabTitleGap}.Layout),
		}
		if body != nil {
			children = append(children, layout.Rigid(body))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: tokens.SizeSlabBelow}.Layout))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}
