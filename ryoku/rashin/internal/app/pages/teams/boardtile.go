package teams

import (
	"image"
	"image/color"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	// the crew strip shows this many marks, the lead's first
	crewMarks         = 5
	crewMark  unit.Dp = 24
	teamMark  unit.Dp = 40
)

// a board tile's face: it lights on hover and darkens on press as a tablet does
type slab struct {
	click        kit.Click
	hover, press anim.Tween
}

// a team's tile: its whole face opens the team, its button gives the team a task
type teamTile struct {
	slab
	give kit.Plaque
}

// a tile lifts off the pane as a slab does, a quiet one is only its vein until hovered; a notch ink marks the corner of a team that needs the user
func (s *slab) layout(gtx layout.Context, th *look.Theme, quiet bool, notch color.NRGBA, body layout.Widget) layout.Dimensions {
	lit := look.Transition(gtx, &s.hover, s.click.Hovered(), th.Motion.HoverIn, th.Motion.HoverOut)
	press := look.Transition(gtx, &s.press, s.click.Pressed(), th.Motion.Press, th.Motion.Press)
	return s.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		record := op.Record(gtx.Ops)
		dimensions := layout.UniformInset(th.Sp.S3).Layout(gtx, body)
		call := record.Stop()
		bounds := image.Rectangle{Max: dimensions.Size}
		cut := gtx.Dp(th.Cut.Mid)
		fill := palette.Mix(th.R.Slab, th.R.Fg, tokens.AlphaHoverPlane*lit)
		if quiet {
			fill = palette.WithAlpha(th.R.Slab, lit)
		}
		kit.FillCut(gtx.Ops, bounds, cut, palette.Mix(fill, th.R.Fg, tokens.AlphaPressRow*press))
		kit.StrokeCut(gtx.Ops, bounds, cut, max(1, gtx.Dp(1)), th.R.Vein)
		kit.Notch(gtx.Ops, image.Point{}, gtx.Dp(th.Cut.Notch), notch)
		call.Add(gtx.Ops)
		return dimensions
	})
}

// who the team is and how it stands, what it is for, its crew, what it is at, its month, and its one action at the foot
func (b *board) layoutTile(gtx layout.Context, e *env.Env, snapshot appstate.Snapshot, working []protocol.TeamTask, team protocol.Team) layout.Dimensions {
	th := e.Theme
	tile := b.tile(team.ID)
	word, tone, busy := teamsui.TeamStanding(team)
	var notch color.NRGBA
	if tone == palette.Attention {
		notch = th.R.Stone(tone)
	}
	now, active := nowLine(gtx.Now, snapshot, working, team)
	nowInk := th.R.Fg2
	if active {
		nowInk = th.R.Fg
	}
	week := tasksThisWeek(snapshot, team.ID, gtx.Now)
	top := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(teamsui.Mark(th, team.Name, teamMark, false)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(1, kit.Text(th.Theme, th.T.Title, th.R.Fg, team.Name)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(teamsui.Claim(th, word, tone, busy)),
				)
			}),
		}
		if team.Purpose != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 2}.Layout(gtx, th.Theme, team.Purpose)
				}),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(crewStrip(th, team.Members)),
		)
		if runs := enginesCaption(snapshot, team.Members); runs != "" {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Label{Style: th.T.Detail, Color: th.R.Fg2, MaxLines: 1}.Layout(gtx, th.Theme, runs)
				}),
			)
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Body, nowInk, now)),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	foot := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(teamsui.MonthMeter(th, team.Stats.MonthTokens, team.Budget.MonthTokens, team.Budget.WarnPercent)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return tile.give.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						given := i18n.Tf("%s this week", tasksWord(week))
						return kit.Label{Style: th.T.Caption, Color: th.R.Fg3, MaxLines: 1, Align: text.End}.Layout(gtx, th.Theme, given)
					}),
				)
			}),
		)
	}
	// the grid gives a row's tiles one height, the foot keeps to the bottom so meters and buttons line up
	return tile.layout(gtx, th, false, notch, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(top),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(foot),
		)
	})
}

// the quiet tile after the teams: another team, started from a template
func (b *board) layoutCreate(gtx layout.Context, th *look.Theme) layout.Dimensions {
	return b.create.layout(gtx, th, true, color.NRGBA{}, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceSides, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(look.IconWidget(look.Icon.Add, 20, th.R.Fg2)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("New team"))),
				)
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Start from a template"))),
		)
	})
}

// the crew in small marks with the lead's first, then who leads and how many they are
func crewStrip(th *look.Theme, members []protocol.Member) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lead, leadIndex := teamsui.Lead(members)
		if leadIndex < 0 {
			return kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("No member yet"))(gtx)
		}
		shown := []protocol.Member{lead}
		for index, member := range members {
			if index != leadIndex && len(shown) < crewMarks {
				shown = append(shown, member)
			}
		}
		children := make([]layout.FlexChild, 0, 2*len(shown)+1)
		for index, member := range shown {
			if index > 0 {
				children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout))
			}
			children = append(children, layout.Rigid(teamsui.Mark(th, member.Name, crewMark, member.Lead)))
		}
		led := i18n.Tf("Led by %s · %s", lead.Name, membersWord(len(members)))
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Flexed(1, kit.Text(th.Theme, th.T.Detail, th.R.Fg2, led)),
		)
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	}
}

// what the team is at now, else its next run, else when it last reported; true while a task works
func nowLine(now time.Time, snapshot appstate.Snapshot, working []protocol.TeamTask, team protocol.Team) (string, bool) {
	for _, task := range working {
		if task.Team != team.ID {
			continue
		}
		if member := teamsui.MemberName(team, task.Member); member != "" {
			return i18n.Tf("Now: %s · %s", task.Title, member), true
		}
		return i18n.Tf("Now: %s", task.Title), true
	}
	var next *protocol.Routine
	if !team.Paused {
		for index, routine := range snapshot.Routines {
			if routine.Team == team.ID && routine.Enabled && routine.NextRun != nil && (next == nil || routine.NextRun.Before(*next.NextRun)) {
				next = &snapshot.Routines[index]
			}
		}
	}
	if next != nil {
		return i18n.Tf("Next: %s %s", next.Title, look.Until(now, *next.NextRun)), false
	}
	if last := team.Stats.LastReport; last != nil {
		return i18n.Tf("Idle, last report %s", look.Ago(now, *last)), false
	}
	return i18n.T("Nothing yet"), false
}
