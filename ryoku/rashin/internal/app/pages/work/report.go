package work

import (
	"image"
	"path/filepath"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the report reads first, then what it made, then how it went for whoever wants to know
func (t *taskView) reportItems(e *env.Env, snapshot state.Snapshot, task protocol.TeamTask, session string, height int) []layout.Widget {
	th := e.Theme
	found := task.Report
	items := []layout.Widget{
		look.Paragraph(th, th.T.Title, th.R.Fg, found.Title),
		func(gtx layout.Context) layout.Dimensions {
			dimensions := t.prose.Layout(gtx, th, e.Overlays, found.Body)
			env.HandleChatActions(gtx, e, session, t.prose.Actions())
			return dimensions
		},
	}
	if len(found.Files) > 0 {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return t.layoutFiles(gtx, e, found.Files) })
	}
	if session == "" {
		return items
	}
	return append(items,
		func(gtx layout.Context) layout.Dimensions { return t.layoutHow(gtx, e, snapshot, task) },
		func(gtx layout.Context) layout.Dimensions {
			return t.how.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				return t.layoutTranscript(gtx, e, snapshot, task, session, height)
			})
		},
	)
}

func (t *taskView) layoutFiles(gtx layout.Context, e *env.Env, files []string) layout.Dimensions {
	if t.files == nil {
		t.files = make(map[string]*kit.Plaque)
	}
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(files), func(gtx layout.Context, index int) layout.Dimensions {
		path := files[index]
		plaque := t.files[path]
		if plaque == nil {
			plaque = new(kit.Plaque)
			t.files[path] = plaque
		}
		plaque.Kind, plaque.Small, plaque.Icon, plaque.Label = kit.PlaqueQuiet, true, look.Icon.File, filepath.Base(path)
		if plaque.Clicked(gtx) {
			e.OpenPath(path)
		}
		return plaque.Layout(gtx, e.Theme.Theme)
	})
}

// how long it worked and in how many steps opens its transcript; a notch says a step failed or waited on the user
func (t *taskView) layoutHow(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	title, tone := teamsui.Worked(snapshot, task)
	toggle := func(gtx layout.Context) layout.Dimensions { return foldToggle(gtx, th, &t.toggle, &t.how.Open, title) }
	if tone == palette.Idle {
		return toggle(gtx)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Dp(th.Cut.Notch)
			kit.Notch(gtx.Ops, image.Point{}, size, th.R.Stone(tone))
			return layout.Dimensions{Size: image.Pt(size, size)}
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(toggle),
	)
}

// a quiet line that opens and closes what follows it
func foldToggle(gtx layout.Context, th *look.Theme, click *kit.Click, open *bool, label string) layout.Dimensions {
	if click.Clicked(gtx) {
		*open = !*open
	}
	icon := look.Icon.ExpandMore
	if *open {
		icon = look.Icon.ExpandLess
	}
	ink := th.R.Fg2
	if click.Hovered() {
		ink = th.R.Fg
	}
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Label, ink, label)),
			layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
			layout.Rigid(look.IconWidget(icon, 18, ink)),
		)
	})
}

// the line above the work that says why it waits or why it ended, and the
// cards that answer what it waits on from the user, so it is answered where it is read
func (t *taskView) layoutState(gtx layout.Context, e *env.Env, snapshot state.Snapshot, task protocol.TeamTask) layout.Dimensions {
	th := e.Theme
	t.needs.Task = task.ID
	needs := t.needs.Shown(heldBy(snapshot, task.ID))
	line, tone, asking := stateLine(snapshot, task)
	if line == "" && len(needs) == 0 {
		return layout.Spacer{Height: th.Sp.S4}.Layout(gtx)
	}
	// the Inbox keeps every card together, a quiet second way to the same answer
	t.inbox.Kind, t.inbox.Small, t.inbox.Label = kit.PlaqueQuiet, true, i18n.T("Answer in the Inbox")
	if asking && t.inbox.Clicked(gtx) {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabInbox, Task: task.ID})
	}
	children := make([]layout.FlexChild, 0, 1+2*len(needs))
	if line != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Flake(gtx, th.Theme, tone, 1, 9) }),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Flexed(1, look.Paragraph(th, th.T.Body, th.R.Fg2, line)),
			}
			if asking {
				row = append(row,
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.inbox.Layout(gtx, th.Theme) }),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, row...)
		}))
	}
	for _, need := range needs {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.needs.Layout(gtx, e, need) }),
		)
	}
	return layout.Inset{Top: th.Sp.S4, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// the approvals the task waits on from the user
func heldBy(snapshot state.Snapshot, task string) []teamsui.Need {
	var held []teamsui.Need
	for _, need := range teamsui.Needs(snapshot) {
		if need.Approval != nil && need.Approval.Task == task {
			held = append(held, need)
		}
	}
	return held
}

// a held task says what holds it; asking is set when the answer is the user's
func stateLine(snapshot state.Snapshot, task protocol.TeamTask) (line string, tone palette.Tone, asking bool) {
	switch task.Status {
	case protocol.TaskBlocked:
		if task.Reason != "" {
			return i18n.Tf("Blocked: %s", task.Reason), palette.Failed, false
		}
		return i18n.T("Blocked before it finished."), palette.Failed, false
	case protocol.TaskStopped:
		if task.Reason != "" {
			return i18n.Tf("Stopped: %s", task.Reason), palette.Paused, false
		}
		return i18n.T("Stopped before it finished."), palette.Paused, false
	case protocol.TaskReady:
		return i18n.T("Queued: it starts when its team has room."), palette.Idle, false
	}
	if task.Status == protocol.TaskNeedsYou || task.Waiting == protocol.WaitingOnYou {
		for _, approval := range snapshot.Approvals {
			if approval.Task == task.ID && approval.Status == protocol.ApprovalPending {
				return i18n.Tf("Waiting for you: %s", approval.Title), palette.Attention, true
			}
		}
		return i18n.T("Waiting for you."), palette.Attention, true
	}
	if task.Waiting == protocol.WaitingOnMembers {
		return i18n.T("Waiting on its members."), palette.Becoming, false
	}
	for _, team := range snapshot.Teams {
		if team.ID == task.Team && team.Paused && task.Status == protocol.TaskWorking {
			return i18n.T("Waiting while its team is paused."), palette.Paused, false
		}
	}
	return "", palette.Idle, false
}

// the session's transcript, live while it works; it scrolls inside the height it is given
func (t *taskView) layoutTranscript(
	gtx layout.Context,
	e *env.Env,
	snapshot state.Snapshot,
	task protocol.TeamTask,
	session string,
	height int,
) layout.Dimensions {
	th := e.Theme
	if session == "" {
		note := i18n.T("No conversation was kept for this task.")
		if task.Status == protocol.TaskReady {
			note = i18n.T("Its conversation shows here once it starts.")
		}
		return look.Paragraph(th, th.T.Caption, th.R.Fg3, note)(gtx)
	}
	size := image.Pt(gtx.Constraints.Max.X, max(0, height))
	attached, ok := snapshot.Attached[session]
	if !ok {
		return look.SkeletonRows(gtx, th, 4, true)
	}
	view := gtx
	view.Constraints = layout.Exact(size)
	t.view.Layout(view, th, e.Overlays, attached.Model)
	// permission cards answer through the session, as they do in Agents
	env.HandleChatActions(gtx, e, session, t.view.Actions())
	return layout.Dimensions{Size: size}
}
