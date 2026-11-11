package work

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	actionStop = iota
	actionRetry
	actionConversation
)

// the details stand beside the reading column from this width, below it when narrower
const (
	asideFrom  unit.Dp = 1100
	asideWidth unit.Dp = 300
	asideGap   unit.Dp = 34
)

// one task's page: its report or its live transcript, and the details beside them
type taskView struct {
	id     string
	header look.PaneHeader
	column look.Scroller
	side   look.Scroller
	prose  chat.Prose
	view   chat.View
	files  map[string]*kit.Plaque
	how    kit.Disclosure
	toggle kit.Click
	// the details fold above a live transcript where they cannot stand beside it
	details       kit.Disclosure
	detailsToggle kit.Click
	inbox         kit.Plaque
	links         map[string]*kit.Click
	// what the task waits on from the user, answered here on the Inbox's cards
	needs teamsui.NeedCards
	// the session whose transcript is attached, and the task whose report was marked read
	attached, read string
}

func (t *taskView) open(id string) {
	t.id = id
	t.prose, t.view = chat.Prose{}, chat.View{}
	t.how.Open, t.details.Open, t.attached = false, false, ""
	t.column.Position, t.side.Position = layout.Position{}, layout.Position{}
}

func (t *taskView) layout(gtx layout.Context, e *env.Env, page *Page, task protocol.TeamTask) layout.Dimensions {
	snapshot := e.Snapshot()
	// opening a task with a report the user has not seen counts as reading it
	if task.Report != nil && !task.Read && t.read != task.ID {
		t.read = task.ID
		e.Call(protocol.MTeamsTaskRead, protocol.TeamTaskRef{ID: task.ID}, new(protocol.TeamTask), e.Report)
	}
	session := taskSession(task)
	// a report stands for the work, so its transcript is only fetched once asked for
	if session != "" && (task.Report == nil || t.how.Open) {
		t.attach(e, session)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutHeader(gtx, e, page, snapshot, task, session)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return t.layoutBody(gtx, e, snapshot, task, session) }),
	)
}

// a task run again works in a new session, which gets a fresh transcript
func (t *taskView) attach(e *env.Env, session string) {
	if t.attached == session {
		return
	}
	if t.attached != "" {
		t.view = chat.View{}
	}
	t.attached = session
	e.Attach(session, e.Report)
}

func (t *taskView) layoutHeader(
	gtx layout.Context,
	e *env.Env,
	page *Page,
	snapshot state.Snapshot,
	task protocol.TeamTask,
	session string,
) layout.Dimensions {
	word, tone, _ := teamsui.TaskStanding(task)
	facts := []look.HeaderFact{{Text: teamsui.TeamName(snapshot.Teams, task.Team), Kind: look.FactLink}}
	if member := teamsui.MemberOf(snapshot, task.Team, task.Member); member != "" {
		facts = append(facts, look.HeaderFact{Text: member})
	}
	facts = append(facts, look.HeaderFact{Text: teamsui.Origin(snapshot.Teams, snapshot.Routines, task)})
	if task.Started != nil {
		facts = append(facts, look.HeaderFact{Text: i18n.Tf("Started %s", look.Ago(gtx.Now, *task.Started))})
	}
	content := look.PaneHeaderContent{
		Title: task.Title, Facts: facts, Claim: word, ClaimTone: tone,
		Scrolled: t.column.Position.First > 0 || t.column.Position.Offset > 0,
		Menu:     []kit.MenuItem{{Icon: look.Icon.Delete, Label: i18n.T("Delete task"), Danger: true}},
	}
	actions := make([]int, 0, 3)
	if !teamsui.Finished(task) {
		content.Actions = append(content.Actions, look.HeaderAction{Icon: look.Icon.Stop, Label: i18n.T("Stop"), Description: i18n.T("Stop the task and the pieces it handed out")})
		actions = append(actions, actionStop)
	} else {
		content.Actions = append(content.Actions, look.HeaderAction{Icon: look.Icon.Retry, Label: i18n.T("Run again"), Description: i18n.T("Run the task again in a new session")})
		actions = append(actions, actionRetry)
	}
	if session != "" {
		content.Actions = append(content.Actions, look.HeaderAction{Icon: look.Icon.OpenInNew, Description: i18n.T("Open the conversation")})
		actions = append(actions, actionConversation)
	}
	dimensions := t.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := t.header.ActionClicked(); ok && index < len(actions) {
		switch actions[index] {
		case actionStop:
			e.Call(protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: task.ID}, new(protocol.TeamTask), e.Report)
		case actionRetry:
			page.retry.open(task.ID)
		case actionConversation:
			e.Navigate(protocol.TabAgent, session)
		}
	}
	if index, ok := t.header.LinkClicked(); ok && index == 0 {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams, Team: task.Team})
	}
	if _, ok := t.header.MenuClicked(); ok {
		page.remove.open(task.ID)
	}
	return dimensions
}

func (t *taskView) layoutBody(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, session string) layout.Dimensions {
	size := gtx.Constraints.Max
	inset := gtx.Dp(look.ContentInset(e.Class))
	aside := func(gtx layout.Context) layout.Dimensions { return t.layoutAside(gtx, e, snapshot, task) }
	if size.X < gtx.Dp(asideFrom) {
		if task.Report == nil {
			// a live transcript scrolls by itself, so it takes the height under the state line
			// rather than sit in a second scroll it would keep the wheel from
			return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
				return t.layoutLive(gtx, e, snapshot, task, session, aside)
			})
		}
		// narrower, the details follow the report and everything scrolls together
		items := append(t.items(e, snapshot, task, session, size.Y), aside)
		return t.column.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
				return padItem(gtx, e.Theme, index, len(items), items[index])
			})
		})
	}
	gap, side := gtx.Dp(asideGap), gtx.Dp(asideWidth)
	width := max(0, min(gtx.Dp(look.ReadingColumnWidth), size.X-2*inset-gap-side))
	left := max(inset, (size.X-width-gap-side)/2)

	column := gtx
	column.Constraints = layout.Exact(image.Pt(width, size.Y))
	offset := op.Offset(image.Pt(left, 0)).Push(gtx.Ops)
	if task.Report == nil {
		t.layoutLive(column, e, snapshot, task, session, nil)
	} else {
		items := t.items(e, snapshot, task, session, size.Y)
		t.column.Layout(column, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			return padItem(gtx, e.Theme, index, len(items), items[index])
		})
	}
	offset.Pop()

	details := gtx
	details.Constraints = layout.Exact(image.Pt(side, size.Y))
	offset = op.Offset(image.Pt(left+width+gap, 0)).Push(gtx.Ops)
	t.side.Layout(details, e.Theme, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layout.Inset{Top: e.Theme.Sp.S4, Bottom: e.Theme.Sp.S5}.Layout(gtx, aside)
	})
	offset.Pop()
	return layout.Dimensions{Size: size}
}

// a live task is its transcript, filling the column under the line that says why it waits;
// where no details stand beside it, they fold open above the transcript
func (t *taskView) layoutLive(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, session string, details layout.Widget) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.layoutState(gtx, e, snapshot, task) }),
	}
	if details != nil {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: e.Theme.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return foldToggle(gtx, e.Theme, &t.detailsToggle, &t.details.Open, i18n.T("Details"))
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return t.details.Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: e.Theme.Sp.S3}.Layout(gtx, details)
				})
			}),
		)
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return t.layoutTranscript(gtx, e, snapshot, task, session, gtx.Constraints.Max.Y)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a finished task's column, top to bottom: why it stopped, then the report and how it went
func (t *taskView) items(e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, session string, height int) []layout.Widget {
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return t.layoutState(gtx, e, snapshot, task) }}
	return append(items, t.reportItems(e, snapshot, task, session, height)...)
}

// the state line comes first and brings its own space above
func padItem(gtx layout.Context, th *look.Theme, index, count int, item layout.Widget) layout.Dimensions {
	inset := layout.Inset{Bottom: th.Sp.S3}
	if index == 0 {
		inset.Bottom = 0
	}
	if index == count-1 {
		inset.Bottom = th.Sp.S5
	}
	return inset.Layout(gtx, item)
}
