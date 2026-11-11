package chat

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// a turn Rashin started to keep the agent on its goal reads as a quiet step,
// not as something the reader said
func (state *viewState) layoutGoalTurn(gtx layout.Context, th *look.Theme, text string) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(look.IconWidget(look.Icon.Goal, 16, th.R.Fg3)),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Flexed(1, kit.RawText(th.Theme, th.T.Label, th.R.Fg2, text)),
	)
}

// where a goal stands when it changes: set, resumed, done, blocked, paused
// or stopped, with the agent's summary or Rashin's reason
func (state *viewState) layoutGoalEvent(gtx layout.Context, th *look.Theme, goal protocol.Goal, key string) layout.Dimensions {
	tone, word := goalClaim(goal)
	// the objective itself is the message right after a goal is set, so the event names the budget
	detail := strings.TrimSpace(goal.Reason)
	if goal.Status == protocol.GoalActive {
		detail = i18n.Tf("Up to %d turns", goal.MaxTurns)
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(look.IconWidget(look.Icon.Goal, 16, th.R.Fg3)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(kit.Claim(th.Theme, tone, word)),
			)
		}),
	}
	if detail != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: workIndent}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return state.selectable(gtx, th, key, th.T.Body, th.R.Fg2, detail)
				})
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// the tone and word a goal's state is claimed with
func goalClaim(goal protocol.Goal) (palette.Tone, string) {
	switch goal.Status {
	case protocol.GoalDone:
		return palette.Alive, i18n.T("Goal done")
	case protocol.GoalBlocked:
		return palette.Attention, i18n.T("Goal blocked")
	case protocol.GoalPaused:
		return palette.Paused, i18n.T("Goal paused")
	case protocol.GoalStopped:
		return palette.Idle, i18n.T("Goal stopped")
	}
	if goal.Turns > 0 {
		return palette.Becoming, i18n.T("Goal resumed")
	}
	return palette.Becoming, i18n.T("Goal set")
}
