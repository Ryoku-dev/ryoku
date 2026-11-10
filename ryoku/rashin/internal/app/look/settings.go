package look

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
)

const (
	SettingsColumn  unit.Dp = 760
	SettingsControl unit.Dp = 320
)

type Setting struct {
	Title, Detail string
	Control       layout.Widget
	// puts the control under the text, for one too wide to stand beside it
	Below bool
}

func SettingsSection(gtx layout.Context, th *Theme, title string, rows []Setting) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, title)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return SettingsSlab(gtx, th, rows) }),
	)
}

func SettingsSlab(gtx layout.Context, th *Theme, rows []Setting) layout.Dimensions {
	width := gtx.Constraints.Max.X
	record := op.Record(gtx.Ops)
	inset := gtx.Dp(21)
	y := gtx.Dp(8)
	for index, row := range rows {
		if index > 0 {
			FillRect(gtx, image.Rect(inset, y, width-inset, y+max(1, gtx.Dp(1))), th.R.Vein)
		}
		offset := op.Offset(image.Pt(inset, y)).Push(gtx.Ops)
		child := gtx
		child.Constraints = layout.Constraints{Min: image.Pt(width-2*inset, 0), Max: image.Pt(width-2*inset, gtx.Constraints.Max.Y)}
		dimensions := layoutSetting(child, th, row)
		offset.Pop()
		y += dimensions.Size.Y
	}
	y += gtx.Dp(8)
	call := record.Stop()
	kit.FillCut(gtx.Ops, image.Rect(0, 0, width, y), gtx.Dp(th.Cut.Mid), th.R.Stone1)
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, y)}
}

func layoutSetting(gtx layout.Context, th *Theme, row Setting) layout.Dimensions {
	words := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(Paragraph(th, th.T.Row, th.R.Fg, row.Title))}
		if row.Detail != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 2}.Layout),
				layout.Rigid(Paragraph(th, th.T.Caption, th.R.Fg3, row.Detail)),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if row.Control == nil {
			return words(gtx)
		}
		if row.Below {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(words),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(row.Control),
			)
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, words),
			layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = 0
				gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(SettingsControl))
				return row.Control(gtx)
			}),
		)
	})
}
