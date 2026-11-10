package projects

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

var boardStatuses = []string{
	protocol.TaskBacklog,
	protocol.TaskReady,
	protocol.TaskWorking,
	protocol.TaskNeedsYou,
	protocol.TaskReview,
	protocol.TaskDone,
	protocol.TaskBlocked,
}

func (t *tasksState) layout(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	tasks := e.Snapshot().Tasks[page.project]
	if t.newTask.Clicked(gtx) {
		t.newOpen, t.newDialog.Open = true, true
	}
	if t.planTasks.Clicked(gtx) {
		t.planOpen, t.planDialog.Open = true, true
	}
	t.newTask.Kind, t.newTask.Label = kit.PlaqueGold, i18n.T("New task")
	if t.selected != "" {
		t.newTask.Kind = kit.PlaqueLine
	}
	t.planTasks.Kind, t.planTasks.Label = kit.PlaqueQuiet, i18n.T("Plan from a goal")
	dimensions := layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutToolbar(gtx, e)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return t.layoutTaskContent(gtx, e, page, tasks)
		}),
	)
	t.layoutNewDialog(gtx, e, page, tasks)
	t.layoutPlanDialog(gtx, e, page)
	t.layoutGateDialog(gtx, e, page)
	return dimensions
}

func (t *tasksState) layoutToolbar(
	gtx layout.Context,
	e *env.Env,
) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.newTask.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.planTasks.Layout(gtx, e.Theme.Theme)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.E.Layout(
				gtx,
				kit.Text(
					e.Theme.Theme,
					e.Theme.T.Caption,
					e.Theme.R.Fg3,
					i18n.T("Scroll sideways to see every column"),
				),
			)
		}),
	)
}

func (t *tasksState) layoutTaskContent(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	tasks []protocol.TaskInfo,
) layout.Dimensions {
	selected, hasSelection := taskByID(tasks, t.selected)
	if !hasSelection {
		return t.layoutBoard(gtx, e, page, tasks)
	}
	if gtx.Constraints.Max.X >= gtx.Dp(900) {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(.62, func(gtx layout.Context) layout.Dimensions {
				return t.layoutBoard(gtx, e, page, tasks)
			}),
			layout.Rigid(layout.Spacer{Width: 13}.Layout),
			layout.Flexed(.38, func(gtx layout.Context) layout.Dimensions {
				return t.scroll.Layout(gtx, e.Theme, 1, func(
					gtx layout.Context,
					_ int,
				) layout.Dimensions {
					return t.layoutDetail(gtx, e, page, selected, tasks)
				})
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return t.layoutBoard(gtx, e, page, tasks)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(240))
			return t.layoutDetail(gtx, e, page, selected, tasks)
		}),
	)
}

func (t *tasksState) layoutBoard(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	tasks []protocol.TaskInfo,
) layout.Dimensions {
	groups := groupTasks(tasks)
	return t.board.Layout(gtx, len(boardStatuses), func(
		gtx layout.Context,
		column int,
	) layout.Dimensions {
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(236), gtx.Dp(236)
		status := boardStatuses[column]
		items := groups[status]
		return layout.Inset{Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return kit.Slab(gtx, e.Theme.R.Stone1, e.Theme.Cut.Small, func(
				gtx layout.Context,
			) layout.Dimensions {
				return layout.Inset{Top: 8, Right: 7, Bottom: 8, Left: 7}.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(kit.GroupHeader(
								e.Theme.Theme,
								taskStatusWord(status),
								len(items),
							)),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return t.columns[column].Layout(
									gtx,
									len(items),
									func(gtx layout.Context, index int) layout.Dimensions {
										return t.layoutCard(gtx, e, tasks, items[index])
									},
								)
							}),
						)
					},
				)
			})
		})
	})
}

func (t *tasksState) layoutCard(
	gtx layout.Context,
	e *env.Env,
	all []protocol.TaskInfo,
	task protocol.TaskInfo,
) layout.Dimensions {
	row := t.rows[task.ID]
	if row == nil {
		row = new(taskRow)
		t.rows[task.ID] = row
	}
	if row.click.Clicked(gtx) {
		t.selected = task.ID
	}
	branch := task.Products.Branch
	if branch == "" {
		branch = i18n.T("No worktree yet")
	}
	harness := task.Harness
	if harness == "" {
		harness = i18n.T("Default harness")
	}
	meta := harness + " · " + branch
	dependencies := dependencyWord(task, all)
	gate := openGate(task)
	return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			background := e.Theme.R.Stone2
			if task.ID == t.selected {
				background = e.Theme.R.Stone3
			} else if row.click.Hovered() {
				background = palette.Mix(e.Theme.R.Stone2, e.Theme.R.Fg, .06)
			}
			return kit.Slab(gtx, background, e.Theme.Cut.Small, func(
				gtx layout.Context,
			) layout.Dimensions {
				return layout.Inset{Top: 7, Right: 8, Bottom: 7, Left: 8}.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := kit.Label{
									Style:    e.Theme.T.Row,
									Color:    e.Theme.R.Fg,
									MaxLines: 1,
								}
								return label.Layout(gtx, e.Theme.Theme, task.Title)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								label := kit.Label{
									Style:    e.Theme.T.MonoSm,
									Color:    e.Theme.R.Fg2,
									MaxLines: 1,
								}
								return label.Layout(gtx, e.Theme.Theme, meta)
							}),
							layout.Rigid(kit.Text(
								e.Theme.Theme,
								e.Theme.T.Caption,
								e.Theme.R.Fg3,
								dependencies,
							)),
						}
						if gate != nil {
							children = append(children, layout.Rigid(func(
								gtx layout.Context,
							) layout.Dimensions {
								label := kit.Label{
									Style:    e.Theme.T.Caption,
									Color:    e.Theme.R.Amber,
									MaxLines: 1,
								}
								return label.Layout(gtx, e.Theme.Theme, gate.Question)
							}))
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					},
				)
			})
		})
	})
}

func (t *tasksState) layoutDetail(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
	tasks []protocol.TaskInfo,
) layout.Dimensions {
	t.handleDetailActions(gtx, e, page, task)
	d := &t.detail
	d.start.Kind, d.start.Label = kit.PlaqueGold, i18n.T("Start")
	d.done.Kind, d.done.Label = kit.PlaqueLine, i18n.T("Mark done")
	d.remove.Kind, d.remove.Label = kit.PlaqueDangerLine, i18n.T("Delete")
	d.openWorktree.Kind, d.openWorktree.Label = kit.PlaqueLine, i18n.T("Open worktree")
	d.openGate.Kind, d.openGate.Label = kit.PlaqueLine, i18n.T("Open gate")
	return kit.Slab(
		gtx,
		e.Theme.R.Stone2,
		e.Theme.Cut.Mid,
		func(gtx layout.Context) layout.Dimensions {
			inset := layout.UniformInset(e.Theme.Sp.S4)
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return t.layoutDetailContent(gtx, e, page, task, tasks)
			})
		},
	)
}

func (t *tasksState) handleDetailActions(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
) {
	d := &t.detail
	if d.start.Clicked(gtx) {
		var result protocol.TaskStartResult
		params := protocol.TaskStartParams{ID: task.ID, Harness: task.Harness}
		e.Call(protocol.MTasksStart, params, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
				return
			}
			page.loadTasks(e)
			e.LoadWorktrees(page.project, nil)
			if e.Navigate != nil {
				e.Navigate(protocol.TabAgent, result.Session.ID)
			}
		})
	}
	if d.done.Clicked(gtx) {
		t.updateStatus(e, page, task.ID, protocol.TaskDone)
	}
	if d.remove.Clicked(gtx) {
		d.deleteConfirm.Danger, d.deleteConfirm.Dialog.Open = true, true
	}
	if d.openWorktree.Clicked(gtx) && task.Worktree != "" {
		page.open(e, page.project, TabOverview, task.Worktree)
	}
	if d.openGate.Clicked(gtx) {
		d.gateOpen, d.gateDialog.Open = true, true
	}
}

func (t *tasksState) layoutDetailContent(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
	tasks []protocol.TaskInfo,
) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(
			e.Theme.Theme,
			e.Theme.T.Section,
			e.Theme.R.Fg,
			task.Title,
		)),
		layout.Rigid(layout.Spacer{Height: 5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := kit.Label{Style: e.Theme.T.Body, Color: e.Theme.R.Fg2}
			return label.Layout(gtx, e.Theme.Theme, task.Goal)
		}),
		layout.Rigid(layout.Spacer{Height: 7}.Layout),
		layout.Rigid(kit.Facts(e.Theme.Theme, t.taskFacts(task, tasks))),
		layout.Rigid(layout.Spacer{Height: 7}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutTaskStatus(gtx, e, page, task)
		}),
		layout.Rigid(layout.Spacer{Height: 7}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutDetailActions(gtx, e, task)
		}),
	}
	children = t.appendSessionControls(children, e, task.Sessions)
	children = t.appendGateControls(children, e, page, task)
	t.layoutDeleteConfirm(gtx, e, page, task)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (t *tasksState) taskFacts(
	task protocol.TaskInfo,
	tasks []protocol.TaskInfo,
) []kit.Fact {
	products := i18n.Tf(
		"%s · %d commits · %d files · +%d -%d",
		task.Products.Branch,
		task.Products.Commits,
		task.Products.Files,
		task.Products.Added,
		task.Products.Removed,
	)
	return []kit.Fact{
		{Label: i18n.T("Harness"), Value: task.Harness},
		{Label: i18n.T("Dependencies"), Value: dependencyWord(task, tasks)},
		{Label: i18n.T("Worktree"), Value: task.Worktree, Mono: true},
		{Label: i18n.T("Products"), Value: products},
	}
}

func (t *tasksState) layoutTaskStatus(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
) layout.Dimensions {
	statuses := []string{
		i18n.T("Backlog"),
		i18n.T("Ready"),
		i18n.T("Working"),
		i18n.T("Needs you"),
		i18n.T("Review"),
		i18n.T("Done"),
		i18n.T("Blocked"),
	}
	selected := 0
	for index, status := range boardStatuses {
		if status == task.Status {
			selected = index
			break
		}
	}
	dimensions := t.detail.status.Layout(
		gtx,
		e.Theme,
		e.Overlays,
		statuses,
		selected,
	)
	if index, changed := t.detail.status.Changed(); changed && index < len(boardStatuses) {
		t.updateStatus(e, page, task.ID, boardStatuses[index])
	}
	return dimensions
}

func (t *tasksState) layoutDetailActions(
	gtx layout.Context,
	e *env.Env,
	task protocol.TaskInfo,
) layout.Dimensions {
	d := &t.detail
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if task.Worktree != "" {
				gtx = gtx.Disabled()
			}
			return d.start.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.done.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.openGate.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if task.Worktree == "" {
				gtx = gtx.Disabled()
			}
			return d.openWorktree.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return d.remove.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func (t *tasksState) appendSessionControls(
	children []layout.FlexChild,
	e *env.Env,
	sessions []string,
) []layout.FlexChild {
	for _, sessionID := range sessions {
		id := sessionID
		open := t.detail.sessionOpen[id]
		if open == nil {
			open = new(kit.Plaque)
			t.detail.sessionOpen[id] = open
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			open.Kind, open.Label, open.Small = kit.PlaqueLine, i18n.T("Open session"), true
			if open.Clicked(gtx) && e.Navigate != nil {
				e.Navigate(protocol.TabAgent, id)
			}
			return open.Layout(gtx, e.Theme.Theme)
		}))
	}
	return children
}

func (t *tasksState) appendGateControls(
	children []layout.FlexChild,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
) []layout.FlexChild {
	for _, taskGate := range task.Gates {
		gate := taskGate
		choice := t.detail.gateChoices[gate.ID]
		if choice == nil {
			choice = new(look.Choice)
			t.detail.gateChoices[gate.ID] = choice
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if gate.Resolution != "" {
				facts := []kit.Fact{{Label: gate.Question, Value: gate.Resolution}}
				return kit.Facts(e.Theme.Theme, facts)(gtx)
			}
			dimensions := choice.Layout(gtx, e.Theme, e.Overlays, gate.Options, 0)
			if index, changed := choice.Changed(); changed && index < len(gate.Options) {
				var result protocol.TaskInfo
				params := protocol.GateResolveParams{
					Task:       task.ID,
					Gate:       gate.ID,
					Resolution: gate.Options[index],
				}
				e.Call(protocol.MTasksResolveGate, params, &result, func(error) {
					page.loadTasks(e)
				})
			}
			return dimensions
		}))
	}
	return children
}

func (t *tasksState) layoutDeleteConfirm(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	task protocol.TaskInfo,
) {
	if !t.detail.deleteConfirm.Dialog.Open {
		return
	}
	confirmed := t.detail.deleteConfirm.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Delete task"),
		i18n.T("Delete this task? Its worktree and sessions stay."),
		i18n.T("Delete"),
		task.Title,
	)
	if confirmed {
		e.Call(
			protocol.MTasksDelete,
			protocol.TaskRef{ID: task.ID},
			new(protocol.Empty),
			func(error) {
				t.selected = ""
				page.loadTasks(e)
			},
		)
	}
}

func (t *tasksState) layoutNewDialog(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	tasks []protocol.TaskInfo,
) {
	if !t.newOpen {
		return
	}
	if t.newCancel.Clicked(gtx) {
		t.newOpen = false
		t.newDialog.Dismiss(gtx)
	}
	if t.newCreate.Clicked(gtx) {
		deps := []string{}
		for _, task := range tasks {
			toggle := t.depToggles[task.ID]
			if toggle != nil && toggle.Value {
				deps = append(deps, task.ID)
			}
		}
		var result protocol.TaskInfo
		params := protocol.TaskCreateParams{
			Project: page.project,
			Title:   strings.TrimSpace(t.title.Text()),
			Goal:    strings.TrimSpace(t.goal.Text()),
			Deps:    deps,
			Harness: strings.TrimSpace(t.harness.Text()),
		}
		e.Call(protocol.MTasksCreate, params, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
				return
			}
			t.newOpen = false
			t.newDialog.Dismiss(gtx)
			t.selected = result.ID
			page.loadTasks(e)
		})
	}
	t.newCancel.Kind, t.newCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	t.newCreate.Kind, t.newCreate.Label = kit.PlaqueGold, i18n.T("Create task")
	body := func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.title.Label = i18n.T("Title")
				return t.title.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.goal.Label = i18n.T("Goal")
				return t.goal.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.harness.Label = i18n.T("Harness")
				return t.harness.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(kit.Text(
				e.Theme.Theme,
				e.Theme.T.Group,
				e.Theme.R.Fg2,
				i18n.T("Dependencies"),
			)),
		}
		for _, task := range tasks {
			toggle := t.depToggles[task.ID]
			if toggle == nil {
				toggle = new(kit.Toggle)
				t.depToggles[task.ID] = toggle
			}
			toggle.Label = task.Title
			children = append(children, layout.Rigid(
				func(gtx layout.Context) layout.Dimensions {
					return toggle.Layout(gtx, e.Theme.Theme)
				},
			))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return t.newCancel.Layout(gtx, e.Theme.Theme)
	}
	create := func(gtx layout.Context) layout.Dimensions {
		return t.newCreate.Layout(gtx, e.Theme.Theme)
	}
	t.newDialog.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("New task"),
		body,
		cancel,
		create,
	)
}

func (t *tasksState) layoutPlanDialog(gtx layout.Context, e *env.Env, page *pageState) {
	if !t.planOpen {
		return
	}
	if t.planCancel.Clicked(gtx) {
		t.planOpen = false
		t.planDialog.Dismiss(gtx)
	}
	if t.planRun.Clicked(gtx) {
		var result protocol.TaskPlanResult
		e.Call(protocol.MTasksPlan, protocol.TaskPlanParams{Project: page.project, Goal: strings.TrimSpace(t.planGoal.Text())}, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
				return
			}
			t.drafts = result.Drafts
			t.draftSelected = make([]bool, len(result.Drafts))
			t.draftToggles = make([]kit.Toggle, len(result.Drafts))
			t.draftTitles = make([]kit.TextField, len(result.Drafts))
			t.draftGoals = make([]kit.TextField, len(result.Drafts))
			t.draftDeps = make([]kit.TextField, len(result.Drafts))
			for index, draft := range result.Drafts {
				t.draftSelected[index] = true
				t.draftToggles[index].Value = true
				t.draftTitles[index].SetText(draft.Title)
				t.draftGoals[index].SetText(draft.Goal)
				t.draftDeps[index].SetText(draftDepNumbers(draft))
			}
		})
	}
	if t.planCreate.Clicked(gtx) {
		t.createDrafts(e, page)
	}
	t.planCancel.Kind, t.planCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	t.planRun.Kind, t.planRun.Label = kit.PlaqueLine, i18n.T("Plan")
	t.planCreate.Kind, t.planCreate.Label = kit.PlaqueGold, i18n.T("Create selected")
	t.planDialog.Layout(gtx, e.Theme.Theme, i18n.T("Plan tasks from a goal"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t.planGoal.Label = i18n.T("Goal")
			return t.planGoal.Layout(gtx, e.Theme.Theme)
		}), layout.Rigid(layout.Spacer{Height: 7}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.planRun.Layout(gtx, e.Theme.Theme) })}
		for index := range t.drafts {
			index := index
			children = append(children, layout.Rigid(layout.Spacer{Height: 7}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.draftToggles[index].Label = i18n.T("Create this task")
				dimensions := t.draftToggles[index].Layout(gtx, e.Theme.Theme)
				t.draftSelected[index] = t.draftToggles[index].Value
				return dimensions
			}), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.draftTitles[index].Label = i18n.T("Title")
				return t.draftTitles[index].Layout(gtx, e.Theme.Theme)
			}), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.draftGoals[index].Label = i18n.T("Goal")
				return t.draftGoals[index].Layout(gtx, e.Theme.Theme)
			}), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				t.draftDeps[index].Label = i18n.T("Dependencies by task number")
				return t.draftDeps[index].Layout(gtx, e.Theme.Theme)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, func(gtx layout.Context) layout.Dimensions { return t.planCancel.Layout(gtx, e.Theme.Theme) }, func(gtx layout.Context) layout.Dimensions {
		if len(t.drafts) == 0 {
			gtx = gtx.Disabled()
		}
		return t.planCreate.Layout(gtx, e.Theme.Theme)
	})
}

func (t *tasksState) createDrafts(e *env.Env, page *pageState) {
	selected := make([]int, 0, len(t.drafts))
	for index := range t.drafts {
		if t.draftSelected[index] {
			selected = append(selected, index)
		}
	}
	if len(selected) == 0 {
		t.planOpen, t.planDialog.Open = false, false
		return
	}
	created := make(map[int]string, len(selected))
	var createNext func(int)
	createNext = func(position int) {
		if position >= len(selected) {
			t.planOpen, t.planDialog.Open = false, false
			page.loadTasks(e)
			return
		}
		index := selected[position]
		dependencyIndexes := parseDraftDeps(t.draftDeps[index].Text())
		deps := make([]string, 0, len(dependencyIndexes))
		for _, dependency := range dependencyIndexes {
			if id := created[dependency]; id != "" {
				deps = append(deps, id)
			}
		}
		var result protocol.TaskInfo
		params := protocol.TaskCreateParams{
			Project: page.project,
			Title:   t.draftTitles[index].Text(),
			Goal:    t.draftGoals[index].Text(),
			Deps:    deps,
		}
		e.Call(protocol.MTasksCreate, params, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
			} else {
				created[index] = result.ID
			}
			createNext(position + 1)
		})
	}
	createNext(0)
}

func (t *tasksState) layoutGateDialog(gtx layout.Context, e *env.Env, page *pageState) {
	if !t.detail.gateOpen {
		return
	}
	d := &t.detail
	if d.gateCancel.Clicked(gtx) {
		d.gateOpen = false
		d.gateDialog.Dismiss(gtx)
	}
	if d.gateCreate.Clicked(gtx) {
		var result protocol.TaskInfo
		params := protocol.GateParams{
			Task:     t.selected,
			Question: strings.TrimSpace(d.gateQuestion.Text()),
			Options:  splitList(d.gateOptions.Text()),
		}
		e.Call(protocol.MTasksGate, params, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
				return
			}
			d.gateOpen = false
			d.gateDialog.Dismiss(gtx)
			page.loadTasks(e)
		})
	}
	d.gateCancel.Kind, d.gateCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	d.gateCreate.Kind, d.gateCreate.Label = kit.PlaqueGold, i18n.T("Open gate")
	body := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d.gateQuestion.Label = i18n.T("Question")
				return d.gateQuestion.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d.gateOptions.Label = i18n.T("Options, one per line")
				return d.gateOptions.Layout(gtx, e.Theme.Theme)
			}),
		)
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return d.gateCancel.Layout(gtx, e.Theme.Theme)
	}
	create := func(gtx layout.Context) layout.Dimensions {
		return d.gateCreate.Layout(gtx, e.Theme.Theme)
	}
	d.gateDialog.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Open decision gate"),
		body,
		cancel,
		create,
	)
}

func (t *tasksState) updateStatus(e *env.Env, page *pageState, id, status string) {
	var result protocol.TaskInfo
	e.Call(protocol.MTasksUpdate, protocol.TaskUpdateParams{ID: id, Patch: rawPatch(map[string]string{"status": status})}, &result, func(err error) {
		if err != nil {
			page.error = err.Error()
		} else {
			page.loadTasks(e)
		}
	})
}

func groupTasks(tasks []protocol.TaskInfo) map[string][]protocol.TaskInfo {
	groups := make(map[string][]protocol.TaskInfo, len(boardStatuses))
	for _, status := range boardStatuses {
		groups[status] = nil
	}
	for _, task := range tasks {
		groups[task.Status] = append(groups[task.Status], task)
	}
	return groups
}
func taskByID(tasks []protocol.TaskInfo, id string) (protocol.TaskInfo, bool) {
	for _, task := range tasks {
		if task.ID == id {
			return task, true
		}
	}
	return protocol.TaskInfo{}, false
}
func dependencyWord(task protocol.TaskInfo, tasks []protocol.TaskInfo) string {
	if len(task.Deps) == 0 {
		return i18n.T("No dependencies")
	}
	done := map[string]bool{}
	for _, candidate := range tasks {
		done[candidate.ID] = candidate.Status == protocol.TaskDone
	}
	for _, dep := range task.Deps {
		if !done[dep] {
			return i18n.T("Blocked by dependencies")
		}
	}
	return i18n.T("Dependencies done")
}
func openGate(task protocol.TaskInfo) *protocol.Gate {
	for index := range task.Gates {
		if task.Gates[index].Resolution == "" {
			return &task.Gates[index]
		}
	}
	return nil
}
func taskStatusWord(status string) string {
	switch status {
	case protocol.TaskBacklog:
		return i18n.T("Backlog")
	case protocol.TaskReady:
		return i18n.T("Ready")
	case protocol.TaskWorking:
		return i18n.T("Working")
	case protocol.TaskNeedsYou:
		return i18n.T("Needs you")
	case protocol.TaskReview:
		return i18n.T("Review")
	case protocol.TaskDone:
		return i18n.T("Done")
	default:
		return i18n.T("Blocked")
	}
}
func draftDepNumbers(draft protocol.TaskDraft) string {
	values := make([]string, len(draft.Deps))
	for index, dep := range draft.Deps {
		values[index] = strconv.Itoa(dep + 1)
	}
	return strings.Join(values, ", ")
}
func parseDraftDeps(value string) []int {
	fields := splitList(value)
	result := make([]int, 0, len(fields))
	for _, field := range fields {
		number, err := strconv.Atoi(field)
		if err == nil && number > 0 {
			result = append(result, number-1)
		}
	}
	return result
}
