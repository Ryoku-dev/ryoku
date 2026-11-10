package projects

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func (a *activityState) layout(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	if a.loaded != page.project && !a.loading {
		a.loading = true
		var result protocol.ActivityResult
		e.Call(
			protocol.MActivityList,
			protocol.ActivityParams{Project: page.project, Limit: 300},
			&result,
			func(err error) {
				a.loading = false
				if err != nil {
					page.error = err.Error()
					return
				}
				a.items, a.loaded = result.Activity, page.project
			},
		)
	}
	if len(a.items) == 0 {
		empty := kit.Empty{
			Title: i18n.T("No activity yet"),
			Body:  i18n.T("Git, worktree, review and task changes will appear here."),
		}
		return empty.Layout(gtx, e.Theme.Theme)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutActivityColumns(
				gtx,
				e,
				i18n.T("Time"),
				i18n.T("Actor"),
				i18n.T("Action"),
				i18n.T("Target"),
				i18n.T("Result"),
				true,
				palette.Idle,
			)
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.scroll.Layout(gtx, e.Theme, len(a.items), func(
				gtx layout.Context,
				index int,
			) layout.Dimensions {
				return layoutActivityRow(gtx, e, a.items[index])
			})
		}),
	)
}

func layoutActivityRow(
	gtx layout.Context,
	e *env.Env,
	item protocol.Activity,
) layout.Dimensions {
	result, tone := i18n.T("Done"), palette.Alive
	if !item.OK {
		result, tone = i18n.T("Failed"), palette.Failed
	}
	target := item.Target
	if item.Detail != "" {
		target += " · " + item.Detail
	}
	return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Theme.R.Stone1, e.Theme.Cut.Small, func(
			gtx layout.Context,
		) layout.Dimensions {
			return layout.Inset{Top: 8, Right: 10, Bottom: 8, Left: 10}.Layout(
				gtx,
				func(gtx layout.Context) layout.Dimensions {
					return layoutActivityColumns(
						gtx,
						e,
						item.Time.Local().Format("2006-01-02 15:04"),
						item.Actor,
						item.Action,
						target,
						result,
						false,
						tone,
					)
				},
			)
		})
	})
}

func layoutActivityColumns(
	gtx layout.Context,
	e *env.Env,
	timeValue string,
	actor string,
	action string,
	target string,
	result string,
	header bool,
	tone palette.Tone,
) layout.Dimensions {
	timeWidth, actorWidth, actionWidth := gtx.Dp(126), gtx.Dp(120), gtx.Dp(170)
	if gtx.Constraints.Max.X < gtx.Dp(760) {
		timeWidth, actorWidth, actionWidth = gtx.Dp(96), gtx.Dp(82), gtx.Dp(120)
	}
	style, ink := e.Theme.T.Body, e.Theme.R.Fg2
	if header {
		style, ink = e.Theme.T.Group, e.Theme.R.Fg3
	}
	cell := func(width int, value string, mono bool) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			textStyle := style
			if mono {
				textStyle = e.Theme.T.MonoSm
			}
			label := kit.Label{Style: textStyle, Color: ink, MaxLines: 1}
			return label.Layout(gtx, e.Theme.Theme, value)
		})
	}
	resultCell := func(gtx layout.Context) layout.Dimensions {
		if header {
			label := kit.Label{Style: style, Color: ink, MaxLines: 1}
			return label.Layout(gtx, e.Theme.Theme, result)
		}
		return kit.Claim(e.Theme.Theme, tone, result)(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		cell(timeWidth, timeValue, true),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		cell(actorWidth, actor, false),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		cell(actionWidth, action, false),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := kit.Label{Style: style, Color: ink, MaxLines: 1}
			return label.Layout(gtx, e.Theme.Theme, target)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(resultCell),
	)
}
