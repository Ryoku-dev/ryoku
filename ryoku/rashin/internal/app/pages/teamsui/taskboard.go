package teamsui

import (
	"sort"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// from this width the board stands in columns, narrower it is the grouped list
const columnsFrom unit.Dp = 1100

// TaskBoard is the Work board of every team's tasks or one team's: four
// columns from 1100 wide (Queued, Working, Needs you, Done), narrower the
// grouped list, and Load older under what ended. Its rows go into the
// scroller of the page that holds it.
type TaskBoard struct {
	// Team is the team the board shows, "" for every team.
	Team string
	// Empty is what the grouped list says while the board has no task.
	Empty string

	tablets map[string]*kit.Tablet
	older   kit.Plaque
	loading bool
	// the presence of each task in hand, so what its member does cross-fades
	presences map[string]*Presence
	// teams whose older tasks are all loaded, so the board stops offering more
	exhausted map[string]bool

	// the frame's tasks and rows, read by Rows and drawn by LayoutRow
	tasks [GroupCount][]protocol.TeamTask
	rows  []boardRow
}

type boardRow struct {
	kind  int
	group Group
	task  protocol.TeamTask
}

const (
	rowColumns = iota
	rowSection
	rowTask
	rowOlder
	rowEmpty
)

// Rows reads the frame's tasks and how wide the pane is, and returns how many
// rows the board has and how many tasks it shows.
func (b *TaskBoard) Rows(gtx layout.Context, snapshot state.Snapshot) (rows, tasks int) {
	b.tasks = GroupTasks(snapshot, b.Team)
	for _, list := range b.tasks {
		tasks += len(list)
	}
	b.rows = b.rows[:0]
	if gtx.Constraints.Max.X >= gtx.Dp(columnsFrom) {
		b.rows = append(b.rows, boardRow{kind: rowColumns})
		return len(b.rows), tasks
	}
	for at := GroupNeeds; at < GroupCount; at++ {
		if list := b.tasks[at]; len(list) > 0 {
			b.rows = append(b.rows, boardRow{kind: rowSection, group: at})
			for _, task := range list {
				b.rows = append(b.rows, boardRow{kind: rowTask, task: task})
			}
		}
		if at == GroupDone && b.offersOlder(snapshot) {
			b.rows = append(b.rows, boardRow{kind: rowOlder})
		}
	}
	if tasks == 0 && b.Empty != "" {
		b.rows = append(b.rows, boardRow{kind: rowEmpty})
	}
	return len(b.rows), tasks
}

// LayoutRow draws one of the rows Rows counted.
func (b *TaskBoard) LayoutRow(gtx layout.Context, e *env.Env, index int) layout.Dimensions {
	if index < 0 || index >= len(b.rows) {
		return layout.Dimensions{}
	}
	th := e.Theme
	snapshot := e.Snapshot()
	row := b.rows[index]
	bottom := unit.Dp(0)
	if index == len(b.rows)-1 {
		bottom = th.Sp.S5
	}
	return layout.Inset{Bottom: bottom}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch row.kind {
		case rowColumns:
			return b.layoutColumns(gtx, e, snapshot)
		case rowSection:
			list := b.tasks[row.group]
			return layout.Inset{Top: th.Sp.S3, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, row.group.Label(), len(list)))
		case rowOlder:
			return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return b.layoutOlder(gtx, e, snapshot)
			})
		case rowEmpty:
			return look.Paragraph(th, th.T.Caption, th.R.Fg3, b.Empty)(gtx)
		}
		return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return b.layoutTablet(gtx, e, snapshot, row.task, row.group.Label())
		})
	})
}

// four columns: what waits for a slot, what works, what wants the user, what ended
func (b *TaskBoard) layoutColumns(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	var needs, ended []protocol.TeamTask
	needs = append(needs, b.tasks[GroupNeeds]...)
	ended = append(ended, b.tasks[GroupDone]...)
	// a blocked task waits on the user like one that needs them, a stopped one has ended
	for _, task := range b.tasks[GroupHalted] {
		if task.Status == protocol.TaskBlocked {
			needs = append(needs, task)
		} else {
			ended = append(ended, task)
		}
	}
	sort.SliceStable(ended, func(i, j int) bool { return Ended(ended[i]).After(Ended(ended[j])) })
	columns := [...]struct {
		title string
		tasks []protocol.TeamTask
	}{
		{i18n.T("Queued"), b.tasks[GroupQueued]},
		{i18n.T("Working"), b.tasks[GroupWorking]},
		{i18n.T("Needs you"), needs},
		{i18n.T("Done"), ended},
	}
	children := make([]layout.FlexChild, 0, len(columns)*2)
	for index, column := range columns {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout))
		}
		older := index == len(columns)-1 && b.offersOlder(snapshot)
		children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2}.Layout(gtx, look.SectionTitle(th, column.title, len(column.tasks)))
				}),
			}
			if len(column.tasks) == 0 {
				rows = append(rows, layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Nothing here."))))
			}
			for _, task := range column.tasks {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return b.layoutTablet(gtx, e, snapshot, task, column.title)
					})
				}))
			}
			if older {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return b.layoutOlder(gtx, e, snapshot)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx, children...)
}

// a task's tablet: its team's mark, the title, who works on it on what model and when;
// one in hand shows its presence, a finished one is read by the first line of what it
// found. A claim its column's heading already says is left out, so the title keeps the room
func (b *TaskBoard) layoutTablet(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, heading string) layout.Dimensions {
	th := e.Theme
	if b.tablets == nil {
		b.tablets = make(map[string]*kit.Tablet)
	}
	tablet := b.tablets[task.ID]
	if tablet == nil {
		tablet = new(kit.Tablet)
		b.tablets[task.ID] = tablet
	}
	tablet.Pane = th.R.Stone2
	if tablet.Clicked(gtx) || tablet.DoubleClicked(gtx) {
		OpenTask(e, task.ID)
	}
	word, tone, _ := TaskStanding(task)
	if word == heading {
		word = ""
	}
	facts := TaskEngine(snapshot, task).Short()
	if member := MemberOf(snapshot, task.Team, task.Member); member != "" {
		facts = i18n.Tf("%s on %s", member, facts)
	}
	content := kit.TabletContent{
		Title: task.Title, Facts: facts, Claim: word, Tone: tone,
		Mark: Mark(th, TeamName(snapshot.Teams, task.Team), 32, false),
	}
	live, inHand := TaskLive(snapshot, task)
	switch {
	case task.Status == protocol.TaskDone && task.Report != nil:
		content.Facts = Excerpt(task.Report.Body, 1)
		content.Proof = look.Ago(gtx.Now, Ended(task))
	case Finished(task):
		content.Proof = look.Ago(gtx.Now, Ended(task))
	case task.Status == protocol.TaskReady:
		content.Proof = look.Ago(gtx.Now, task.Created)
	case inHand:
		presence := b.presence(task.ID)
		content.FactsWidget = func(gtx layout.Context) layout.Dimensions { return presence.Line(gtx, th, live, facts) }
		content.Proof = live.Elapsed(gtx.Now)
		if content.Proof == "" {
			content.Proof = look.Ago(gtx.Now, Started(task))
		}
	default:
		content.Proof = look.Ago(gtx.Now, Started(task))
	}
	return tablet.Layout(gtx, th.Theme, content, false, false)
}

func (b *TaskBoard) presence(task string) *Presence {
	if b.presences == nil {
		b.presences = make(map[string]*Presence)
	}
	presence := b.presences[task]
	if presence == nil {
		presence = new(Presence)
		b.presences[task] = presence
	}
	return presence
}

// the daemon's snapshot holds the newest finished tasks, older ones load on request
func (b *TaskBoard) offersOlder(snapshot state.Snapshot) bool {
	if b.exhausted[b.Team] {
		return false
	}
	for _, task := range snapshot.TeamTasks {
		if Finished(task) && (b.Team == "" || task.Team == b.Team) {
			return true
		}
	}
	return false
}

func (b *TaskBoard) layoutOlder(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	b.older.Kind, b.older.Small, b.older.Label, b.older.Busy = kit.PlaqueTonal, true, i18n.T("Load older"), b.loading
	if b.older.Clicked(gtx) && !b.loading {
		b.loadOlder(e, snapshot)
	}
	return b.older.Layout(gtx, e.Theme.Theme)
}

func (b *TaskBoard) loadOlder(e *env.Env, snapshot state.Snapshot) {
	var oldest time.Time
	for _, task := range snapshot.TeamTasks {
		if (b.Team == "" || task.Team == b.Team) && (oldest.IsZero() || task.Created.Before(oldest)) {
			oldest = task.Created
		}
	}
	if oldest.IsZero() {
		return
	}
	team, before := b.Team, len(snapshot.TeamTasks)
	b.loading = true
	e.LoadTeamTasks(team, oldest.Format(time.RFC3339Nano), func(err error) {
		b.loading = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		if len(e.Snapshot().TeamTasks) == before {
			if b.exhausted == nil {
				b.exhausted = make(map[string]bool)
			}
			b.exhausted[team] = true
		}
	})
}
