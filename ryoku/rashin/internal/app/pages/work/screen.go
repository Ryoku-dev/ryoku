package work

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	agentpage "ryoku-rashin/internal/app/pages/agent"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the member's own screen at the top of a task's details: watched, taken with Take
// control, and while the user drives it a strip that hands it back to the member
type taskScreen struct {
	view       agentpage.ScreenView
	take, back kit.Plaque
}

// the screen of the task's session, while its team lets agents use one and one is up
func screenOf(snapshot state.Snapshot, task protocol.TeamTask) (protocol.ScreenState, bool) {
	team, _ := teamsui.TeamByID(snapshot.Teams, task.Team)
	session := taskSession(task)
	if !team.Tools.Screen || session == "" {
		return protocol.ScreenState{}, false
	}
	screen, ok := snapshot.Screens[session]
	return screen, ok
}

// gold is false while a need card on the page holds the view's one gold action
func (s *taskScreen) layout(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, screen protocol.ScreenState, gold bool) layout.Dimensions {
	th := e.Theme
	yours := screen.Holder == protocol.HolderUser
	control := func(verb string) {
		e.Call(protocol.MScreenControl, protocol.ScreenControlParams{ID: screen.Session, Op: verb}, new(protocol.ScreenState), e.Report)
	}
	member := teamsui.MemberOf(snapshot, task.Team, task.Member)
	s.take.Kind, s.take.Small, s.take.Label = kit.PlaqueTonal, true, i18n.T("Take control")
	// what the member asks to be done on its screen makes taking it the one thing to do
	if screen.Ask != "" && gold {
		s.take.Kind = kit.PlaqueGold
	}
	s.back.Kind, s.back.Small, s.back.Label = kit.PlaqueTonal, true, i18n.T("Hand back")
	if gold {
		s.back.Kind = kit.PlaqueGold
	}
	if member != "" {
		s.back.Label = i18n.Tf("Hand back to %s", member)
	}
	if s.take.Clicked(gtx) && !yours && screen.State == protocol.ScreenRunning {
		control(protocol.ScreenTake)
	}
	if s.back.Clicked(gtx) && yours {
		control(protocol.ScreenRelease)
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.N.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return s.view.Layout(gtx, e, screen) })
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	}
	if yours {
		driving := func(gtx layout.Context) layout.Dimensions { return s.layoutDriving(gtx, e) }
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, append(children, layout.Rigid(driving))...)
	}
	switch {
	case screen.Ask != "":
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Detail, Color: th.R.Fg, MaxLines: 3}.Layout(gtx, th.Theme, screen.Ask)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		)
	case screen.Action != "":
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 2}.Layout(gtx, th.Theme, screen.Action)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		)
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.take.Layout(gtx, th.Theme) }))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// the strip under a screen the user drives: that they do, and the way to give it back
func (s *taskScreen) layoutDriving(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	return teamsui.NotchedSlab(gtx, th, palette.Attention, th.Sp.S2, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Label{Style: th.T.Label, Color: th.R.Fg, MaxLines: 1}.Layout(gtx, th.Theme, i18n.T("You're driving"))
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return s.back.Layout(gtx, th.Theme) }),
		)
	})
}
