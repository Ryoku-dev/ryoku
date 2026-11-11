package look

import (
	"gioui.org/layout"
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
	widgets := make([]layout.Widget, len(rows))
	for index, row := range rows {
		widgets[index] = func(gtx layout.Context) layout.Dimensions { return layoutSetting(gtx, th, row) }
	}
	return kit.SlabRows(gtx, th.Theme, widgets...)
}

func layoutSetting(gtx layout.Context, th *Theme, row Setting) layout.Dimensions {
	words := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(Paragraph(th, th.T.Row, th.R.Fg, row.Title))}
		if row.Detail != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(Paragraph(th, th.T.Detail, th.R.Fg2, row.Detail)),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
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
}
