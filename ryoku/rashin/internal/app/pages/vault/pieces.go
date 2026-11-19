package vault

import (
	"gioui.org/layout"

	"ryoku-rashin/internal/app/look"
)

func chip(th *look.Theme, value string) layout.Widget { return look.TokenChip(th, value) }

func chips(th *look.Theme, values []string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}.Layout(gtx, len(values), func(gtx layout.Context, index int) layout.Dimensions {
			return chip(th, values[index])(gtx)
		})
	}
}
