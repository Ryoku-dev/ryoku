package routines

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the board with no routine open: what runs next, then every routine
type board struct {
	header look.PaneHeader
	scroll look.Scroller
	cards  map[string]*teamsui.RoutineTablet
	// coming rows are clicked by their place in Coming up
	coming [comingLimit]kit.Click
	start  kit.Plaque
}

type boardItem struct {
	kind    int
	title   string
	count   int
	index   int
	run     run
	routine protocol.Routine
}

const (
	itemLead = iota
	itemNoTeam
	itemSection
	itemComing
	itemRoutine
)

func (b *board) layout(gtx layout.Context, e *env.Env, page *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	runs := comingRuns(gtx.Now, snapshot, &page.schedules, comingLimit)
	items := b.items(snapshot, runs)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{
				Title:    i18n.T("Routines"),
				Actions:  []look.HeaderAction{{Icon: look.Icon.Add, Label: i18n.T("New routine"), Description: i18n.T("New routine")}},
				Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
			}
			dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
			if _, ok := b.header.ActionClicked(); ok {
				page.New(e)
			}
			return dimensions
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
				return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
					return b.layoutItem(gtx, e, page, snapshot, runs, items[index], index == len(items)-1)
				})
			})
		}),
	)
}

func (b *board) items(snapshot state.Snapshot, runs []run) []boardItem {
	items := []boardItem{{kind: itemLead}}
	if len(snapshot.Teams) == 0 {
		return append(items, boardItem{kind: itemNoTeam})
	}
	if len(runs) > 0 {
		items = append(items, boardItem{kind: itemSection, title: i18n.T("Coming up"), count: len(runs)})
		for index, next := range runs {
			items = append(items, boardItem{kind: itemComing, index: index, run: next})
		}
	}
	var listed []protocol.Routine
	for _, team := range snapshot.Teams {
		for _, routine := range snapshot.Routines {
			if routine.Team == team.ID {
				listed = append(listed, routine)
			}
		}
	}
	if len(listed) > 0 {
		items = append(items, boardItem{kind: itemSection, title: i18n.T("Every routine"), count: len(listed)})
		for _, routine := range listed {
			items = append(items, boardItem{kind: itemRoutine, routine: routine})
		}
	}
	b.prune(listed)
	return items
}

// cards of deleted routines go with them
func (b *board) prune(listed []protocol.Routine) {
	if len(b.cards) <= len(listed) {
		return
	}
	kept := make(map[string]*teamsui.RoutineTablet, len(listed))
	for _, routine := range listed {
		if item := b.cards[routine.ID]; item != nil {
			kept[routine.ID] = item
		}
	}
	b.cards = kept
}

func (b *board) layoutItem(
	gtx layout.Context,
	e *env.Env,
	page *Page,
	snapshot state.Snapshot,
	runs []run,
	item boardItem,
	last bool,
) layout.Dimensions {
	th := e.Theme
	bottom := th.Sp.S1
	if last {
		bottom = th.Sp.S5
	}
	switch item.kind {
	case itemLead:
		return layout.Inset{Top: th.Sp.S5, Bottom: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutLead(gtx, e, snapshot, runs)
		})
	case itemNoTeam:
		return layout.Inset{Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutNoTeam(gtx, e)
		})
	case itemSection:
		return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, item.title, item.count))
	case itemComing:
		return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutComing(gtx, e, snapshot, item)
		})
	default:
		return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.card(item.routine.ID).Layout(gtx, e, &page.schedules, item.routine, page.selected == item.routine.ID)
		})
	}
}

func (b *board) card(id string) *teamsui.RoutineTablet {
	if b.cards == nil {
		b.cards = make(map[string]*teamsui.RoutineTablet)
	}
	item := b.cards[id]
	if item == nil {
		item = new(teamsui.RoutineTablet)
		b.cards[id] = item
	}
	return item
}

func (b *board) layoutLead(gtx layout.Context, e *env.Env, snapshot state.Snapshot, runs []run) layout.Dimensions {
	return look.Monument(gtx, e.Theme, boardSentence(gtx.Now, snapshot, runs), boardProof(gtx.Now, snapshot))
}

func (b *board) layoutNoTeam(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	b.start.Kind, b.start.Label = kit.PlaqueTonal, i18n.T("Start a team")
	if b.start.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Draft: env.DraftTeam})
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("A routine belongs to a team, so start one first."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return b.start.Layout(gtx, th.Theme) }),
	)
}

// one coming run: when, the routine, its team; it opens the routine
func (b *board) layoutComing(gtx layout.Context, e *env.Env, snapshot state.Snapshot, item boardItem) layout.Dimensions {
	click := &b.coming[item.index]
	if click.Clicked(gtx) {
		teamsui.OpenRoutine(e, item.run.routine.ID)
	}
	routine := item.run.routine
	return teamsui.RunRow(gtx, e.Theme, click, item.run.at, routine.Title, teamsui.TeamName(snapshot.Teams, routine.Team))
}

// what is on and when it next runs, or that nothing is
func boardSentence(now time.Time, snapshot state.Snapshot, runs []run) string {
	if len(snapshot.Routines) == 0 {
		return i18n.T("No routine yet.")
	}
	on := teamsui.EnabledRoutines(snapshot)
	switch {
	case on == 0:
		return i18n.T("No routine is on.")
	case len(runs) == 0 && on == 1:
		return i18n.T("One routine on. Its team is paused.")
	case len(runs) == 0:
		return i18n.Tf("%s routines on. Their teams are paused.", look.CountStart(on))
	case on == 1:
		return i18n.Tf("One routine on. It runs %s.", look.Until(now, runs[0].at))
	}
	return i18n.Tf("%s routines on. The next runs %s.", look.CountStart(on), look.Until(now, runs[0].at))
}

// the latest run, or what a routine is for when there has been none
func boardProof(now time.Time, snapshot state.Snapshot) string {
	var latest protocol.Routine
	for _, routine := range snapshot.Routines {
		if routine.LastRun != nil && (latest.LastRun == nil || routine.LastRun.After(*latest.LastRun)) {
			latest = routine
		}
	}
	if latest.LastRun == nil {
		return i18n.T("A routine gives a team a task on a clock, such as every weekday at 09:00.")
	}
	word, _ := teamsui.RoutineOutcome(snapshot, latest)
	return i18n.Tf("%s last ran %s. %s.", latest.Title, look.Ago(now, *latest.LastRun), word)
}
