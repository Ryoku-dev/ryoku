package agent

import (
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// goalStrip stands above the composer while a session has a goal: what it is,
// where it stands, how much of its budget it used, and the controls to pause,
// resume, stop or put it away
type goalStrip struct {
	pause, stop kit.Plaque
}

func (strip *goalStrip) layout(gtx layout.Context, e *env.Env, info protocol.SessionInfo) layout.Dimensions {
	goal := info.Goal
	if goal == nil {
		return layout.Dimensions{}
	}
	th := e.Theme
	live := goal.Status == protocol.GoalActive || goal.Status == protocol.GoalPaused
	strip.pause.Kind, strip.pause.Small = kit.PlaqueLine, true
	strip.stop.Kind, strip.stop.Small = kit.PlaqueQuiet, true
	switch goal.Status {
	case protocol.GoalActive:
		strip.pause.Label = i18n.T("Pause")
	default:
		strip.pause.Label = i18n.T("Resume")
	}
	strip.stop.Label = i18n.T("Stop goal")
	if !live {
		strip.stop.Label = i18n.T("Clear")
	}
	call := func(op string) {
		e.Call(protocol.MSessionGoal, protocol.GoalParams{ID: info.ID, Op: op}, new(protocol.SessionInfo), report(e))
	}
	if strip.pause.Clicked(gtx) && goal.Status != protocol.GoalDone {
		if goal.Status == protocol.GoalActive {
			call(protocol.GoalPause)
		} else {
			call(protocol.GoalResume)
		}
	}
	if strip.stop.Clicked(gtx) {
		if live {
			call(protocol.GoalStop)
		} else {
			call(protocol.GoalClear)
		}
	}
	if goal.Status == protocol.GoalActive {
		gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(time.Minute - gtx.Now.Sub(goal.Started)%time.Minute)})
	}
	tone, word := stripClaim(goal.Status)
	facts := goalFacts(goal, gtx.Now)
	reason := ""
	if goal.Status != protocol.GoalActive {
		reason = strings.TrimSpace(goal.Reason)
	}
	record := op.Record(gtx.Ops)
	dimensions := layout.Inset{Top: 10, Bottom: 10, Left: 13, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		head := func(gtx layout.Context) layout.Dimensions {
			buttons := []layout.FlexChild{}
			if goal.Status != protocol.GoalDone {
				buttons = append(buttons,
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return strip.pause.Layout(gtx, th.Theme) }),
				)
			}
			buttons = append(buttons,
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return strip.stop.Layout(gtx, th.Theme) }),
			)
			row := []layout.FlexChild{
				layout.Rigid(look.IconWidget(look.Icon.Goal, 16, th.R.Fg2)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(kit.Claim(th.Theme, tone, word)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, kit.RawText(th.Theme, th.T.Label, th.R.Fg, firstGoalLine(goal.Objective))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, facts)),
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, append(row, buttons...)...)
		}
		if reason == "" {
			return head(gtx)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(head),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 27}.Layout(gtx, kit.RawText(th.Theme, th.T.Caption, th.R.Fg2, reason))
			}),
		)
	})
	content := record.Stop()
	kit.FillCut(gtx.Ops, image.Rectangle{Max: dimensions.Size}, gtx.Dp(th.Cut.Mid), th.R.Stone1)
	content.Add(gtx.Ops)
	return dimensions
}

func stripClaim(status string) (palette.Tone, string) {
	switch status {
	case protocol.GoalPaused:
		return palette.Paused, i18n.T("Paused")
	case protocol.GoalDone:
		return palette.Alive, i18n.T("Done")
	case protocol.GoalBlocked:
		return palette.Attention, i18n.T("Blocked")
	case protocol.GoalStopped:
		return palette.Idle, i18n.T("Stopped")
	}
	return palette.Becoming, i18n.T("Goal")
}

// "turn 3 of 30 · 4 min", the time running until the goal ends
func goalFacts(goal *protocol.Goal, now time.Time) string {
	end := now
	if goal.Ended != nil {
		end = *goal.Ended
	}
	minutes := int(end.Sub(goal.Started).Minutes())
	turns := i18n.Tf("turn %d of %d", min(goal.Turns+1, goal.MaxTurns), goal.MaxTurns)
	if goal.Status != protocol.GoalActive {
		turns = i18n.Tf("%d of %d turns", goal.Turns, goal.MaxTurns)
	}
	if minutes < 1 {
		return turns
	}
	return turns + " · " + i18n.Tf("%d min", minutes)
}

func firstGoalLine(objective string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(objective), "\n")
	return line
}

// sessionCommands puts the slash commands Rashin answers itself ahead of the
// harness's own; a harness command of the same name would never be reached
func sessionCommands(harness []protocol.Command) []protocol.Command {
	commands := []protocol.Command{{Name: "goal", Description: i18n.T("Keep the agent working until an objective is met"), Hint: i18n.T("objective, or pause, resume, stop")}}
	for _, command := range harness {
		if strings.TrimPrefix(command.Name, "/") != "goal" {
			commands = append(commands, command)
		}
	}
	return commands
}

// goalFact leads a session row's facts while its goal is live
func goalFact(info protocol.SessionInfo) string {
	goal := info.Goal
	if goal == nil || (goal.Status != protocol.GoalActive && goal.Status != protocol.GoalPaused) {
		return ""
	}
	if goal.Status == protocol.GoalPaused {
		return i18n.T("Goal paused") + " · "
	}
	return i18n.Tf("Goal, turn %d of %d", min(goal.Turns+1, goal.MaxTurns), goal.MaxTurns) + " · "
}
