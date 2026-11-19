package projects

import (
	"image"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (t *tasksState) layoutDetail(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
	tasks []protocol.TaskInfo,
) layout.Dimensions {
	th := e.Theme
	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return t.layoutIdentity(gtx, e, page, task) },
		func(gtx layout.Context) layout.Dimensions {
			return look.SpecStrip(gtx, th, []look.Spec{
				{Label: i18n.T("Sessions"), Value: kit.Pad2(len(task.Sessions))},
				{Label: i18n.T("Depends on"), Value: kit.Pad2(len(task.Deps))},
				{Label: i18n.T("Questions"), Value: kit.Pad2(len(task.Gates))},
			})
		},
	}
	for index := range task.Gates {
		if gate := task.Gates[index]; gate.Resolution == "" {
			sections = append(sections, func(gtx layout.Context) layout.Dimensions { return t.layoutGate(gtx, e, page, task, gate) })
		}
	}
	sections = append(sections, func(gtx layout.Context) layout.Dimensions {
		return look.Facts(gtx, th, taskFactRows(e, task, tasks))
	})
	if len(task.Sessions) > 0 {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions { return t.layoutSessions(gtx, e, task) })
	}
	sections = append(sections, func(gtx layout.Context) layout.Dimensions {
		return page.sheet.Colophon(gtx, th, page.sheetPage(e))
	})
	return t.detail.Layout(gtx, th, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
		top, bottom := th.Sp.S4, th.Sp.S1
		if index == len(sections)-1 {
			top, bottom = th.Sp.S5, th.Sp.S3
		}
		return detailColumn(gtx, th, top, bottom, sections[index])
	})
}

func (t *tasksState) layoutIdentity(gtx layout.Context, e *env.Env, page *pageState, task protocol.TaskInfo) layout.Dimensions {
	th := e.Theme
	word, tone := taskClaim(task)
	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Section, th.R.Fg, task.Title)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(kit.Claim(th.Theme, tone, word)),
	}
	if goal := strings.TrimSpace(task.Goal); goal != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Proof, th.R.Fg2, goal)),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.layoutActs(gtx, e, page, task) }),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (t *tasksState) layoutActs(gtx layout.Context, e *env.Env, page *pageState, task protocol.TaskInfo) layout.Dimensions {
	th := e.Theme
	started := task.Worktree != "" || len(task.Sessions) > 0
	t.start.Kind, t.start.Icon, t.start.Label = kit.PlaqueGold, look.Icon.Play, i18n.T("Start")
	if started {
		t.start.Icon, t.start.Label = look.Icon.Agents, i18n.T("Open session")
	}
	t.done.Kind, t.done.Icon, t.done.Label = kit.PlaqueTonal, look.Icon.Check, i18n.T("Mark done")
	t.worktree.Kind, t.worktree.Icon, t.worktree.Label = kit.PlaqueTonal, look.Icon.Branch, i18n.T("Open worktree")
	t.more.Kind, t.more.Icon, t.more.Description = kit.PlaqueQuiet, look.Icon.More, i18n.T("Move, ask or delete")
	if t.start.Clicked(gtx) {
		t.act(e, page, task)
	}
	if t.done.Clicked(gtx) {
		t.updateStatus(e, page, task.ID, protocol.TaskDone)
	}
	if t.worktree.Clicked(gtx) {
		e.NavigateWork(page.project, TabOverview, task.Worktree)
	}
	if t.more.Clicked(gtx) {
		t.openMenu(gtx, e, page, task)
	}
	widgets := make([]layout.Widget, 0, 4)
	if task.Status != protocol.TaskDone && (started || task.Status != protocol.TaskBlocked) {
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions { return t.start.Layout(gtx, th.Theme) })
	}
	if task.Status != protocol.TaskDone {
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions { return t.done.Layout(gtx, th.Theme) })
	}
	if task.Worktree != "" {
		widgets = append(widgets, func(gtx layout.Context) layout.Dimensions { return t.worktree.Layout(gtx, th.Theme) })
	}
	widgets = append(widgets, func(gtx layout.Context) layout.Dimensions {
		return t.more.Layout(gtx, th.Theme)
	})
	return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(widgets), func(gtx layout.Context, index int) layout.Dimensions {
		return widgets[index](gtx)
	})
}

func (t *tasksState) openMenu(gtx layout.Context, e *env.Env, page *pageState, task protocol.TaskInfo) {
	moves := [...]string{protocol.TaskBacklog, protocol.TaskReady, protocol.TaskBlocked}
	items := make([]kit.MenuItem, 0, 7)
	for _, status := range moves {
		items = append(items, kit.MenuItem{Label: i18n.Tf("Move to %s", taskStatusWord(status)), Disabled: task.Status == status})
	}
	items = append(items,
		kit.MenuItem{Divider: true},
		kit.MenuItem{Icon: look.Icon.Key, Label: i18n.T("Ask for a decision")},
		kit.MenuItem{Icon: look.Icon.Delete, Label: i18n.T("Delete task"), Danger: true},
	)
	tag := &t.menu
	size := gtx.Dp(40)
	anchor := e.Overlays.AnchorOf(&t.more.Clickable, image.Pt(size, size))
	e.Overlays.Open(tag, anchor, look.Placement{}, func(gtx layout.Context) layout.Dimensions {
		dimensions := t.menu.Layout(gtx, e.Theme.Theme, items)
		item, ok := t.menu.Clicked(gtx)
		if !ok {
			return dimensions
		}
		e.Overlays.Close(tag)
		switch {
		case item < len(moves):
			t.updateStatus(e, page, task.ID, moves[item])
		case item == len(moves)+1:
			t.question.open(task)
		case item == len(moves)+2:
			t.remove.open(task)
		}
		return dimensions
	})
}

// none of the options is gold, none is the consequential one before the reader has chosen
func (t *tasksState) layoutGate(gtx layout.Context, e *env.Env, page *pageState, task protocol.TaskInfo, gate protocol.Gate) layout.Dimensions {
	th := e.Theme
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.T("Needs you"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Title, th.R.Fg, gate.Question)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(gate.Options), func(gtx layout.Context, index int) layout.Dimensions {
					option := gate.Options[index]
					key := gate.ID + "\x00" + option
					plaque := t.answers[key]
					if plaque == nil {
						plaque = new(kit.Plaque)
						t.answers[key] = plaque
					}
					plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, option
					if plaque.Clicked(gtx) {
						params := protocol.GateResolveParams{Task: task.ID, Gate: gate.ID, Resolution: option}
						e.Call(protocol.MTasksResolveGate, params, new(protocol.TaskInfo), func(err error) {
							page.fail(err)
							page.loadTasks(e)
						})
					}
					return plaque.Layout(gtx, th.Theme)
				})
			}),
		)
	})
}

func taskFactRows(e *env.Env, task protocol.TaskInfo, tasks []protocol.TaskInfo) []look.FactRow {
	th := e.Theme
	rows := make([]look.FactRow, 0, 8)
	if len(task.Deps) > 0 {
		names := make([]string, 0, len(task.Deps))
		for _, dependency := range task.Deps {
			if other, ok := taskByID(tasks, dependency); ok {
				names = append(names, other.Title)
			}
		}
		rows = append(rows, look.FactRow{Label: i18n.T("Waits for"), Value: strings.Join(names, ", ")})
	}
	if task.Model != "" {
		rows = append(rows, look.FactRow{Label: i18n.T("Model"), Value: env.ModelWords(e, task.Model)})
	} else {
		rows = append(rows, look.FactRow{Label: i18n.T("Model"), Value: i18n.T("Project default")})
	}
	products := task.Products
	if products.Branch != "" {
		rows = append(rows, look.FactRow{Label: i18n.T("Branch"), Value: products.Branch, Mono: true})
	}
	if products.Commits > 0 {
		rows = append(rows, look.FactRow{Label: i18n.T("Commits"), Value: i18n.Tf("%d ahead of its base", products.Commits)})
	}
	if products.Added+products.Removed > 0 {
		rows = append(rows,
			look.FactRow{Label: i18n.T("Files"), Value: i18n.Tf("%d changed", products.Files)},
			look.FactRow{Label: i18n.T("Lines"), Widget: look.DiffCount(th, products.Added, products.Removed, false)},
		)
	}
	if task.Worktree != "" {
		folder := look.ElidePath(paths.Short(task.Worktree), 48)
		rows = append(rows, look.FactRow{Label: i18n.T("Worktree"), Value: folder, Mono: true})
	}
	for _, gate := range task.Gates {
		if gate.Resolution != "" {
			rows = append(rows, look.FactRow{Label: i18n.T("Decided"), Value: gate.Question + ": " + gate.Resolution})
		}
	}
	return rows
}

func (t *tasksState) layoutSessions(gtx layout.Context, e *env.Env, task protocol.TaskInfo) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	children := []layout.FlexChild{layout.Rigid(sectionTitle(th, i18n.T("Sessions"), 1, 1))}
	for _, id := range task.Sessions {
		info, ok := snapshot.Sessions[id]
		if !ok {
			continue
		}
		tablet := t.sessions[id]
		if tablet == nil {
			tablet = new(resourceRow)
			t.sessions[id] = tablet
		}
		if tablet.Clicked(gtx) {
			e.Navigate(protocol.TabAgent, id)
		}
		facts := info.Activity
		if facts == "" {
			facts = info.Preview
		}
		claim, tone := sessionClaim(info)
		content := resourceContent{Title: info.Title, Facts: facts, Claim: claim, Tone: tone, Mark: look.Icon.Agents}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return tablet.Layout(gtx, th, content, false)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
