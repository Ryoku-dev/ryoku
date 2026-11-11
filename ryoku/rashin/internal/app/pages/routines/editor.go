package routines

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the routine editor: an open routine, or a new one when creating
type editor struct {
	id       string
	creating bool
	// base is the routine as the daemon last told it, what Save compares against
	base   protocol.Routine
	loaded bool

	header look.PaneHeader
	scroll look.Scroller

	title         kit.TextField
	brief         look.MultiLine
	team, member  look.Select
	teamID        string
	memberID      string
	when          whenBuilder
	more          kit.Disclosure
	moreToggle    kit.Plaque
	missed        look.Select
	overlap       look.Select
	missedValue   string
	overlapValue  string
	enable        teamsui.RoutineSwitch
	on, runNow    kit.Toggle
	save, discard kit.Plaque
	busy          bool
	problem       string
	aside         asideState
}

func (ed *editor) active() bool { return ed.creating || ed.id != "" }

func (ed *editor) close() { ed.id, ed.creating, ed.loaded = "", false, false }

// a new routine for team, or for the first team when it names none
func (ed *editor) startNew(snapshot state.Snapshot, team string) {
	if _, ok := teamsui.TeamByID(snapshot.Teams, team); !ok && len(snapshot.Teams) > 0 {
		team = snapshot.Teams[0].ID
	}
	ed.close()
	ed.creating, ed.base, ed.teamID, ed.memberID = true, protocol.Routine{}, team, ""
	ed.busy, ed.problem, ed.more.Open = false, "", false
	ed.title.SetText("")
	ed.brief.SetText("")
	ed.when.set(defaultPlan())
	ed.missedValue, ed.overlapValue = protocol.MissedSkip, protocol.OverlapSkip
	ed.on.Value = teamsui.EnabledRoutines(snapshot) < protocol.RoutinesEnabledMax
	ed.runNow.Value = true
}

func (ed *editor) open(id string) {
	if ed.id == id && !ed.creating {
		return
	}
	ed.close()
	ed.id = id
}

func (ed *editor) load(routine protocol.Routine) {
	ed.base, ed.loaded, ed.problem = routine, true, ""
	ed.title.SetText(routine.Title)
	ed.brief.SetText(routine.Brief)
	ed.teamID, ed.memberID = routine.Team, routine.Member
	ed.when.set(readPlan(routine.Schedule))
	ed.missedValue, ed.overlapValue = missedOf(routine), overlapOf(routine)
}

// follows the daemon's routine: edits in progress are kept, an untouched form follows it
func (ed *editor) sync(snapshot state.Snapshot) (protocol.Routine, bool) {
	routine, ok := routineByID(snapshot, ed.id)
	if !ok {
		return routine, false
	}
	switch {
	case !ed.loaded:
		ed.load(routine)
	case !routine.Updated.Equal(ed.base.Updated):
		if len(ed.patch()) == 0 {
			ed.load(routine)
		} else {
			ed.base = routine
		}
	}
	return routine, true
}

// the fields that differ from the routine, as a merge patch
func (ed *editor) patch() map[string]any {
	patch := make(map[string]any)
	if title := strings.TrimSpace(ed.title.Text()); title != ed.base.Title {
		patch["title"] = title
	}
	if brief := strings.TrimSpace(ed.brief.Text()); brief != ed.base.Brief {
		patch["brief"] = brief
	}
	if ed.memberID != ed.base.Member {
		patch["member"] = ed.memberID
	}
	if expression, _ := ed.when.expression(); expression != strings.Join(strings.Fields(ed.base.Schedule), " ") {
		patch["schedule"] = expression
	}
	if ed.missedValue != missedOf(ed.base) {
		patch["missed"] = ed.missedValue
	}
	if ed.overlapValue != overlapOf(ed.base) {
		patch["overlap"] = ed.overlapValue
	}
	return patch
}

// whether the form makes a routine the daemon will take
func (ed *editor) ready(snapshot state.Snapshot) bool {
	_, team := teamsui.TeamByID(snapshot.Teams, ed.teamID)
	return team && strings.TrimSpace(ed.title.Text()) != "" && strings.TrimSpace(ed.brief.Text()) != "" &&
		ed.when.reading.problem == "" && ed.when.reading.ready
}

func (ed *editor) submit(e *env.Env) {
	ed.busy, ed.problem = true, ""
	if !ed.creating {
		id := ed.id
		teamsui.UpdateRoutine(e, id, ed.patch(), func(err error) {
			ed.busy = false
			if err != nil && ed.id == id {
				ed.problem = err.Error()
			}
		})
		return
	}
	expression, _ := ed.when.expression()
	params := protocol.RoutineCreateParams{
		Team: ed.teamID, Title: strings.TrimSpace(ed.title.Text()), Brief: strings.TrimSpace(ed.brief.Text()),
		Member: ed.memberID, Schedule: expression, Missed: ed.missedValue, Overlap: ed.overlapValue,
		Enabled: ed.on.Value, RunNow: ed.runNow.Value,
	}
	var created protocol.Routine
	e.Call(protocol.MTeamsRoutineCreate, params, &created, func(err error) {
		ed.busy = false
		if err != nil {
			ed.problem = err.Error()
			return
		}
		teamsui.OpenRoutine(e, created.ID)
	})
}

func (ed *editor) layout(gtx layout.Context, e *env.Env, page *Page, routine protocol.Routine) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return ed.layoutHeader(gtx, e, page, snapshot, routine) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			inset := look.ContentInset(e.Class)
			return ed.scroll.Layout(gtx, th, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S5, Left: inset, Right: inset}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return kit.DraftColumns(gtx, th.Theme,
						func(gtx layout.Context) layout.Dimensions { return ed.layoutForm(gtx, e, snapshot, routine) },
						func(gtx layout.Context) layout.Dimensions { return ed.aside.layout(gtx, e, ed, snapshot, routine) },
					)
				})
			})
		}),
	)
}

func (ed *editor) layoutHeader(gtx layout.Context, e *env.Env, page *Page, snapshot state.Snapshot, routine protocol.Routine) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title:    i18n.T("New routine"),
		Scrolled: ed.scroll.Position.First > 0 || ed.scroll.Position.Offset > 0,
	}
	team, known := teamsui.TeamByID(snapshot.Teams, ed.teamID)
	if known {
		content.Facts = []look.HeaderFact{{Text: team.Name, Kind: look.FactLink}}
	}
	if !ed.creating {
		content.Title = routine.Title
		content.Claim, content.ClaimTone = teamsui.RoutineStanding(routine)
		content.Actions = []look.HeaderAction{{Icon: look.Icon.Play, Label: i18n.T("Run now"), Description: i18n.T("Run now")}}
		content.Menu = []kit.MenuItem{{Icon: look.Icon.Delete, Label: i18n.T("Delete routine"), Danger: true}}
	}
	dimensions := ed.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := ed.header.LinkClicked(); ok && known {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Team: team.ID, View: env.TeamRoutines})
	}
	if _, ok := ed.header.ActionClicked(); ok && !ed.creating {
		teamsui.RunRoutine(e, routine)
	}
	if _, ok := ed.header.MenuClicked(); ok && !ed.creating {
		page.remove.open(routine)
	}
	return dimensions
}

func missedOf(routine protocol.Routine) string {
	if routine.Missed == protocol.MissedOnce {
		return protocol.MissedOnce
	}
	return protocol.MissedSkip
}

func overlapOf(routine protocol.Routine) string {
	if routine.Overlap == protocol.OverlapQueue {
		return protocol.OverlapQueue
	}
	return protocol.OverlapSkip
}
