package kit

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// SlabContent lays out a standard stone1 slab with its canonical inset.
func SlabContent(gtx layout.Context, th *theme.Theme, body layout.Widget) layout.Dimensions {
	return Slab(gtx, th.R.Stone1, th.Cut.Mid, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(tokens.SizeSlabInset).Layout(gtx, body)
	})
}
