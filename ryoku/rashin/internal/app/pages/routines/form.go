package routines

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the editor's form column: what the routine is, when it runs, and Save
func (ed *editor) layoutForm(gtx layout.Context, e *env.Env, snapshot state.Snapshot, routine protocol.Routine) layout.Dimensions {
	th := e.Theme
	ed.title.Label, ed.title.Placeholder = i18n.T("Title"), i18n.T("Morning review")
	ed.brief.Lines, ed.brief.Label = 4, i18n.T("What to do")
	ed.brief.Placeholder = i18n.T("Review the pull requests opened since the last run and tell me which need me.")
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return ed.title.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return ed.layoutTeam(gtx, e, snapshot) },
		func(gtx layout.Context) layout.Dimensions { return ed.layoutMember(gtx, e, snapshot) },
		func(gtx layout.Context) layout.Dimensions { return ed.brief.Layout(gtx, th) },
		func(gtx layout.Context) layout.Dimensions { return ed.when.layout(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return ed.layoutMore(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return ed.layoutOn(gtx, e, snapshot, routine) },
	}
	if ed.creating {
		ed.runNow.Label = i18n.T("Run it once now to see what it does")
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return ed.runNow.Layout(gtx, th.Theme) })
	}
	if ed.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Caption, th.R.Amber, ed.problem))
	}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions { return ed.layoutFoot(gtx, e, snapshot) })
	children := make([]layout.FlexChild, 0, 2*len(rows))
	for index, row := range rows {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout))
		}
		children = append(children, layout.Rigid(row))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a routine stays with its team, so only a new one picks it
func (ed *editor) layoutTeam(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	if len(snapshot.Teams) == 0 {
		return look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("A routine belongs to a team, so start one first."))(gtx)
	}
	names, selected := make([]string, len(snapshot.Teams)), 0
	for index, team := range snapshot.Teams {
		names[index] = team.Name
		if team.ID == ed.teamID {
			selected = index
		}
	}
	ed.teamID = snapshot.Teams[selected].ID
	ed.team.Label = i18n.T("Team")
	if !ed.creating {
		gtx = gtx.Disabled()
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := ed.team.Layout(gtx, th, e.Overlays, names, selected)
	if index, changed := ed.team.Changed(); changed && ed.creating && index < len(snapshot.Teams) {
		if next := snapshot.Teams[index].ID; next != ed.teamID {
			ed.teamID, ed.memberID = next, ""
		}
	}
	return dimensions
}

// the member who takes each run; the lead decides by default
func (ed *editor) layoutMember(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	team, _ := teamsui.TeamByID(snapshot.Teams, ed.teamID)
	labels, ids, selected := []string{i18n.T("The lead decides")}, []string{""}, 0
	for _, member := range team.Members {
		if member.ID == ed.memberID {
			selected = len(ids)
		}
		labels, ids = append(labels, member.Name), append(ids, member.ID)
	}
	ed.member.Label = i18n.T("Who takes each run")
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dimensions := ed.member.Layout(gtx, th, e.Overlays, labels, selected)
	if index, changed := ed.member.Changed(); changed && index < len(ids) {
		ed.memberID = ids[index]
	}
	return dimensions
}

// what happens to a run that is missed or overlaps, kept behind a plaque
func (ed *editor) layoutMore(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	ed.moreToggle.Kind, ed.moreToggle.Small = kit.PlaqueQuiet, true
	ed.moreToggle.Label, ed.moreToggle.Icon = i18n.T("If a run is missed or still working"), look.Icon.ExpandMore
	if ed.more.Open {
		ed.moreToggle.Icon = look.Icon.ExpandLess
	}
	if ed.moreToggle.Clicked(gtx) {
		ed.more.Open = !ed.more.Open
	}
	choice := func(selection *look.Select, label string, options, values []string, value *string) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			selected := 0
			for index, candidate := range values {
				if candidate == *value {
					selected = index
				}
			}
			selection.Label = label
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := selection.Layout(gtx, th, e.Overlays, options, selected)
			if index, changed := selection.Changed(); changed && index < len(values) {
				*value = values[index]
			}
			return dimensions
		}
	}
	missed := choice(&ed.missed, i18n.T("If the computer was off or asleep"),
		[]string{i18n.T("Skip it"), i18n.T("Run once when it wakes")},
		[]string{protocol.MissedSkip, protocol.MissedOnce}, &ed.missedValue)
	overlap := choice(&ed.overlap, i18n.T("If the last run is still working"),
		[]string{i18n.T("Skip this run"), i18n.T("Run anyway")},
		[]string{protocol.OverlapSkip, protocol.OverlapQueue}, &ed.overlapValue)
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.moreToggle.Layout(gtx, th.Theme) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return ed.more.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(missed),
						layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
						layout.Rigid(overlap),
					)
				})
			})
		}),
	)
}

// an open routine turns on and off at once; a new one is created on or off
func (ed *editor) layoutOn(gtx layout.Context, e *env.Env, snapshot state.Snapshot, routine protocol.Routine) layout.Dimensions {
	th := e.Theme
	full := teamsui.EnabledRoutines(snapshot) >= protocol.RoutinesEnabledMax
	note := th.R.Fg3
	toggle := func(gtx layout.Context) layout.Dimensions {
		if ed.creating {
			ed.on.Label = i18n.T("On")
			if full && !ed.on.Value {
				gtx = gtx.Disabled()
			}
			return ed.on.Layout(gtx, th.Theme)
		}
		ed.enable.Toggle.Label = i18n.T("On")
		return ed.enable.Layout(gtx, e, routine, i18n.T("On"))
	}
	if full && (ed.creating || !routine.Enabled) {
		note = th.R.Amber
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(toggle),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Caption, note, i18n.T("Up to 20 routines can be on at once."))),
	)
}

// Save stays where it is, waiting until something changed, so saving does not
// move the form under the pointer; a new routine has Create
func (ed *editor) layoutFoot(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	changed := ed.creating || len(ed.patch()) > 0
	ready := changed && ed.ready(snapshot) && !ed.busy
	ed.save.Kind, ed.save.Label, ed.save.Busy = kit.PlaqueGold, i18n.T("Save"), ed.busy
	ed.discard.Kind, ed.discard.Label = kit.PlaqueQuiet, i18n.T("Discard changes")
	if ed.creating {
		ed.save.Label, ed.discard.Label = i18n.T("Create routine"), i18n.T("Cancel")
	}
	if ed.save.Clicked(gtx) && ready {
		ed.submit(e)
	}
	if changed && ed.discard.Clicked(gtx) {
		if ed.creating {
			openBoard(e)
		} else {
			ed.load(ed.base)
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return ed.save.Layout(gtx, th.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !changed {
				gtx = gtx.Disabled()
			}
			return ed.discard.Layout(gtx, th.Theme)
		}),
	)
}
