package look

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

// Link is a word in caption ink that brightens under the pointer and opens
// what it names.
func Link(gtx layout.Context, th *Theme, click *kit.Click, value string) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		ink := th.R.Fg3
		if click.Hovered() {
			ink = th.R.Fg
		}
		return kit.Label{Style: th.T.Caption, Color: ink, MaxLines: 1}.Layout(gtx, th.Theme, value)
	})
}
