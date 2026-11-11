package projects

import (
	"strconv"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type createTask struct {
	shown        bool
	dialog       kit.Dialog
	scroll       look.Scroller
	title, goal  kit.TextField
	waits        map[string]*kit.Toggle
	add, cancel  kit.Plaque
	harness      look.Select
	harnessIndex int
}

func (d *createTask) open() {
	d.shown, d.dialog.Open = true, true
	d.title.SetText("")
	d.goal.SetText("")
	d.waits = make(map[string]*kit.Toggle)
	d.harnessIndex = 0
}

func (d *createTask) layout(gtx layout.Context, e *env.Env, page *pageState, tasks *tasksState, existing []protocol.TaskInfo) {
	if !d.shown {
		return
	}
	th := e.Theme
	harnessLabels, harnessIDs := []string{i18n.T("The default harness")}, []string{""}
	for _, harness := range e.Snapshot().Harnesses {
		if harness.Installed {
			harnessLabels, harnessIDs = append(harnessLabels, harness.Name), append(harnessIDs, harness.ID)
		}
	}
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.add.Kind, d.add.Label = kit.PlaqueGold, i18n.T("Create task")
	title := strings.TrimSpace(d.title.Text())
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.add.Clicked(gtx) && title != "" {
		waits := make([]string, 0, len(d.waits))
		for _, task := range existing {
			if toggle := d.waits[task.ID]; toggle != nil && toggle.Value {
				waits = append(waits, task.ID)
			}
		}
		params := protocol.TaskCreateParams{
			Project: page.project, Title: title, Goal: strings.TrimSpace(d.goal.Text()),
			Deps: waits, Harness: harnessIDs[min(d.harnessIndex, len(harnessIDs)-1)],
		}
		var created protocol.TaskInfo
		e.Call(protocol.MTasksCreate, params, &created, func(err error) {
			if err != nil {
				page.fail(err)
				return
			}
			tasks.selected = created.ID
			page.loadTasks(e)
		})
		d.dialog.Dismiss(gtx)
	}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			d.title.Label = i18n.T("Title")
			return d.title.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions {
			d.goal.Label, d.goal.Help = i18n.T("Goal"), i18n.T("What done looks like. The agent starts with this.")
			return d.goal.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions {
			d.harness.Label = i18n.T("Run by")
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := d.harness.Layout(gtx, th, e.Overlays, harnessLabels, d.harnessIndex)
			if index, changed := d.harness.Changed(); changed {
				d.harnessIndex = index
			}
			return dimensions
		},
	}
	if len(existing) > 0 {
		rows = append(rows, kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.T("Waits for")))
		for _, task := range existing {
			if task.Status == protocol.TaskDone {
				continue
			}
			toggle := d.waits[task.ID]
			if toggle == nil {
				toggle = new(kit.Toggle)
				d.waits[task.ID] = toggle
			}
			toggle.Label = task.Title
			rows = append(rows, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) })
		}
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("New task"),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(440))
			gtx.Constraints.Min.Y = 0
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if title == "" {
				gtx = gtx.Disabled()
			}
			return d.add.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

type planDraft struct {
	keep        kit.Toggle
	title, goal kit.TextField
	waits       []int
}

type planTasks struct {
	shown   bool
	working bool
	dialog  kit.Dialog
	scroll  look.Scroller
	goal    kit.TextField
	drafts  []*planDraft
	via     string

	run, create, cancel kit.Plaque
}

func (d *planTasks) open() {
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 640
	d.goal.SetText("")
	d.drafts, d.via = nil, ""
}

func (d *planTasks) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if !d.shown {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.run.Kind, d.run.Label, d.run.Busy = kit.PlaqueLine, i18n.T("Plan"), d.working
	d.create.Kind, d.create.Label = kit.PlaqueGold, i18n.T("Create the kept tasks")
	goal := strings.TrimSpace(d.goal.Text())
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.run.Clicked(gtx) && goal != "" {
		d.plan(e, page, goal)
	}
	if d.create.Clicked(gtx) && len(d.drafts) > 0 {
		d.createKept(e, page)
		d.dialog.Dismiss(gtx)
	}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			d.goal.Label, d.goal.Help = i18n.T("Goal"), i18n.T("Nothing is created until you keep the tasks it proposes.")
			return d.goal.Layout(gtx, th.Theme)
		},
		func(gtx layout.Context) layout.Dimensions { return d.run.Layout(gtx, th.Theme) },
	}
	for index, draft := range d.drafts {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return d.layoutDraft(gtx, th, index, draft) })
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Plan tasks from a goal"),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(480))
			gtx.Constraints.Min.Y = 0
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if len(d.drafts) == 0 {
				gtx = gtx.Disabled()
			}
			return d.create.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

func (d *planTasks) layoutDraft(gtx layout.Context, th *look.Theme, index int, draft *planDraft) layout.Dimensions {
	draft.keep.Label = i18n.Tf("Keep task %d", index+1)
	draft.title.Label, draft.goal.Label = i18n.T("Title"), i18n.T("Goal")
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		fields := gtx
		if !draft.keep.Value {
			fields = gtx.Disabled()
		}
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return draft.keep.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(layout.Context) layout.Dimensions { return draft.title.Layout(fields, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(layout.Context) layout.Dimensions { return draft.goal.Layout(fields, th.Theme) }),
		}
		if len(draft.waits) > 0 {
			numbers := make([]string, len(draft.waits))
			for at, wait := range draft.waits {
				numbers[at] = strconv.Itoa(wait + 1)
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(kit.Text(th.Theme, th.T.Caption, th.R.Fg3, i18n.Tf("Waits for task %s", strings.Join(numbers, ", ")))),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (d *planTasks) plan(e *env.Env, page *pageState, goal string) {
	d.working = true
	var result protocol.TaskPlanResult
	e.Call(protocol.MTasksPlan, protocol.TaskPlanParams{Project: page.project, Goal: goal}, &result, func(err error) {
		d.working = false
		if err != nil {
			page.fail(err)
			return
		}
		d.via = result.Via
		d.drafts = make([]*planDraft, len(result.Drafts))
		for index, proposed := range result.Drafts {
			draft := &planDraft{waits: proposed.Deps}
			draft.keep.Value = true
			draft.title.SetText(proposed.Title)
			draft.goal.SetText(proposed.Goal)
			d.drafts[index] = draft
		}
	})
}

// one after another so a task can wait on the ones made before it
func (d *planTasks) createKept(e *env.Env, page *pageState) {
	drafts := d.drafts
	created := make(map[int]string, len(drafts))
	var next func(int)
	next = func(index int) {
		for index < len(drafts) && !drafts[index].keep.Value {
			index++
		}
		if index >= len(drafts) {
			page.loadTasks(e)
			return
		}
		draft := drafts[index]
		waits := make([]string, 0, len(draft.waits))
		for _, wait := range draft.waits {
			if id := created[wait]; id != "" {
				waits = append(waits, id)
			}
		}
		params := protocol.TaskCreateParams{Project: page.project, Title: draft.title.Text(), Goal: draft.goal.Text(), Deps: waits}
		var task protocol.TaskInfo
		e.Call(protocol.MTasksCreate, params, &task, func(err error) {
			if err != nil {
				page.fail(err)
			} else {
				created[index] = task.ID
			}
			next(index + 1)
		})
	}
	next(0)
}

type openGate struct {
	task              protocol.TaskInfo
	dialog            kit.Dialog
	question, options kit.TextField
	ask, cancel       kit.Plaque
}

func (d *openGate) open(task protocol.TaskInfo) {
	d.task, d.dialog.Open = task, true
	d.question.SetText("")
	d.options.SetText("")
}

func (d *openGate) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if d.task.ID == "" {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.ask.Kind, d.ask.Label = kit.PlaqueGold, i18n.T("Ask")
	question, options := strings.TrimSpace(d.question.Text()), splitList(d.options.Text())
	ready := question != "" && len(options) > 1
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.ask.Clicked(gtx) && ready {
		params := protocol.GateParams{Task: d.task.ID, Question: question, Options: options}
		e.Call(protocol.MTasksGate, params, new(protocol.TaskInfo), func(err error) {
			page.fail(err)
			page.loadTasks(e)
		})
		d.dialog.Dismiss(gtx)
	}
	d.question.Label = i18n.T("Question")
	d.options.Label, d.options.Help = i18n.T("Answers"), i18n.T("Two or more, separated by commas.")
	d.dialog.Layout(gtx, th.Theme, i18n.T("Ask for a decision"),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2,
					i18n.T("The task waits until you answer. The answer goes to the agent working on it."))),
				layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.question.Layout(gtx, th.Theme) }),
				layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.options.Layout(gtx, th.Theme) }),
			)
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.ask.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.task = protocol.TaskInfo{}
	}
}

type removeTask struct {
	task    protocol.TaskInfo
	confirm kit.Confirm
}

func (d *removeTask) open(task protocol.TaskInfo) {
	d.task, d.confirm.Dialog.Open, d.confirm.Danger = task, true, true
}

func (d *removeTask) layout(gtx layout.Context, e *env.Env, page *pageState, tasks *tasksState) {
	if d.task.ID == "" {
		return
	}
	body := i18n.Tf("“%s” is removed from the project. Its worktree, branch and sessions stay.", d.task.Title)
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this task?"), body, i18n.T("Delete"), "") {
		id := d.task.ID
		e.Call(protocol.MTasksDelete, protocol.TaskRef{ID: id}, new(protocol.Empty), func(err error) {
			page.fail(err)
			if tasks.selected == id {
				tasks.selected = ""
			}
			page.loadTasks(e)
		})
	}
	if d.confirm.Closed() {
		d.task = protocol.TaskInfo{}
	}
}
