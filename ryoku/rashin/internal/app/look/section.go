package look

import (
	"strconv"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// SectionTitle names a section of a board in title, with its count in
// monoSm after it when there is one to tell.
func SectionTitle(th *Theme, title string, count int) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, title))}
		if count > 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, strconv.Itoa(count))),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx, children...)
	}
}
