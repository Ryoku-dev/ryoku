package projects

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

var taskOrder = [...]string{
	protocol.TaskNeedsYou, protocol.TaskWorking, protocol.TaskReview, protocol.TaskReady,
	protocol.TaskBlocked, protocol.TaskBacklog, protocol.TaskDone,
}

type tasksState struct {
	list     look.Scroller
	detail   look.Scroller
	rows     map[string]*taskRow
	selected string
	fresh    kit.Plaque
	plan     kit.Plaque

	start, done, gate kit.Plaque
	worktree          kit.Plaque
	more              kit.IconPlaque
	moreTip           look.Tip
	menu              kit.Menu
	answers           map[string]*kit.Plaque
	sessions          map[string]*kit.Tablet

	create   createTask
	planner  planTasks
	question openGate
	remove   removeTask
}

type taskRow struct {
	tablet kit.Tablet
	act    kit.Plaque
}

type taskItem struct {
	header string
	count  int
	task   protocol.TaskInfo
}

func (t *tasksState) init() {
	t.rows = make(map[string]*taskRow)
	t.answers = make(map[string]*kit.Plaque)
	t.sessions = make(map[string]*kit.Tablet)
}

func (t *tasksState) layout(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	tasks := e.Snapshot().Tasks[page.project]
	selected, ok := taskByID(tasks, t.selected)
	if !ok && len(tasks) > 0 {
		selected, ok = firstTask(tasks), true
		t.selected = selected.ID
	}
	list := func(gtx layout.Context) layout.Dimensions { return t.layoutList(gtx, e, page, tasks) }
	if !ok {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3, Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return t.layoutTools(gtx, e)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return kit.Empty{
					Icon: look.Icon.Tasks, Title: i18n.T("No tasks yet"),
					Body: i18n.T("A task is a piece of work an agent does in its own worktree. Write one, or plan several from a goal."),
				}.Layout(gtx, th.Theme)
			}),
		)
	}
	detail := func(gtx layout.Context) layout.Dimensions { return t.layoutDetail(gtx, e, page, selected, tasks) }
	if gtx.Constraints.Max.X < gtx.Dp(overviewSplit) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(280))
				return list(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
			layout.Flexed(1, detail),
		)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			width := gtx.Dp(380)
			gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
			return list(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(max(1, gtx.Dp(1)), gtx.Constraints.Max.Y)
			look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
			return layout.Dimensions{Size: size}
		}),
		layout.Flexed(1, detail),
	)
}

func (t *tasksState) layoutDialogs(gtx layout.Context, e *env.Env, page *pageState) {
	t.create.layout(gtx, e, page, t, e.Snapshot().Tasks[page.project])
	t.planner.layout(gtx, e, page)
	t.question.layout(gtx, e, page)
	t.remove.layout(gtx, e, page, t)
}

func (t *tasksState) layoutTools(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	t.fresh.Kind, t.fresh.Small, t.fresh.Icon, t.fresh.Label = kit.PlaqueGold, true, look.Icon.Add, i18n.T("New task")
	t.plan.Kind, t.plan.Small, t.plan.Label = kit.PlaqueLine, true, i18n.T("Plan from a goal")
	if t.fresh.Clicked(gtx) {
		t.create.open()
	}
	if t.plan.Clicked(gtx) {
		t.planner.open()
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.fresh.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.plan.Layout(gtx, th.Theme) }),
	)
}

func (t *tasksState) layoutList(gtx layout.Context, e *env.Env, page *pageState, tasks []protocol.TaskInfo) layout.Dimensions {
	th := e.Theme
	items := taskItems(tasks)
	return layout.Inset{Top: th.Sp.S3, Left: th.Sp.S3, Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.layoutTools(gtx, e) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return t.list.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					item := items[index]
					if item.header != "" {
						return kit.GroupHeader(th.Theme, item.header, item.count)(gtx)
					}
					return layout.Inset{Bottom: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return t.layoutRow(gtx, e, page, item.task, tasks)
					})
				})
			}),
		)
	})
}

func (t *tasksState) layoutRow(gtx layout.Context, e *env.Env, page *pageState, task protocol.TaskInfo, tasks []protocol.TaskInfo) layout.Dimensions {
	th := e.Theme
	row := t.rows[task.ID]
	if row == nil {
		row = new(taskRow)
		t.rows[task.ID] = row
	}
	row.tablet.Pane = th.R.Stone2
	if row.tablet.Clicked(gtx) {
		t.selected = task.ID
	}
	// the group header already names the state so the row doesn't say it again
	content := kit.TabletContent{Title: task.Title, Facts: taskFacts(task, tasks), Progress: -1}
	_, content.Tone = taskClaim(task)
	if products := task.Products; products.Added+products.Removed > 0 {
		content.Proof = fmt.Sprintf("+%d −%d", products.Added, products.Removed)
	}
	content.Mark = func(gtx layout.Context) layout.Dimensions {
		return layout.Center.Layout(gtx, look.IconWidget(look.Icon.Tasks, 18, th.R.Fg3))
	}
	if label := taskActLabel(task); label != "" {
		row.act.Kind, row.act.Small, row.act.Label = kit.PlaqueLine, true, label
		content.Action = &row.act
		if row.act.Clicked(gtx) {
			t.selected = task.ID
			t.act(e, page, task)
		}
	}
	return row.tablet.Layout(gtx, th.Theme, content, task.ID == t.selected, false)
}

func taskItems(tasks []protocol.TaskInfo) []taskItem {
	groups := make(map[string][]protocol.TaskInfo, len(taskOrder))
	for _, task := range tasks {
		groups[task.Status] = append(groups[task.Status], task)
	}
	items := make([]taskItem, 0, len(tasks)+len(taskOrder))
	for _, status := range taskOrder {
		if len(groups[status]) == 0 {
			continue
		}
		items = append(items, taskItem{header: taskStatusWord(status), count: len(groups[status])})
		for _, task := range groups[status] {
			items = append(items, taskItem{task: task})
		}
	}
	return items
}

func firstTask(tasks []protocol.TaskInfo) protocol.TaskInfo {
	for _, item := range taskItems(tasks) {
		if item.header == "" {
			return item.task
		}
	}
	return protocol.TaskInfo{}
}

func taskFacts(task protocol.TaskInfo, tasks []protocol.TaskInfo) string {
	if gate := openGateOf(task); gate != nil {
		return gate.Question
	}
	switch waiting := unfinishedDeps(task, tasks); {
	case waiting == 1:
		return i18n.T("Waits for one other task")
	case waiting > 1:
		return i18n.Tf("Waits for %d other tasks", waiting)
	}
	if task.Products.Branch != "" {
		return task.Products.Branch
	}
	return firstWords(task.Goal)
}

func firstWords(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func taskClaim(task protocol.TaskInfo) (string, palette.Tone) {
	switch task.Status {
	case protocol.TaskNeedsYou:
		return i18n.T("Needs you"), palette.Attention
	case protocol.TaskWorking:
		return i18n.T("Working"), palette.Becoming
	case protocol.TaskReview:
		return i18n.T("Review"), palette.Alive
	case protocol.TaskBlocked:
		return i18n.T("Blocked"), palette.Idle
	case protocol.TaskDone:
		return i18n.T("Done"), palette.Idle
	default:
		return taskStatusWord(task.Status), palette.Idle
	}
}

func taskActLabel(task protocol.TaskInfo) string {
	switch {
	case task.Status == protocol.TaskNeedsYou || task.Status == protocol.TaskReview || task.Status == protocol.TaskWorking:
		if len(task.Sessions) > 0 {
			return i18n.T("Open")
		}
	case task.Worktree == "" && task.Status != protocol.TaskDone && task.Status != protocol.TaskBlocked:
		return i18n.T("Start")
	}
	return ""
}

func (t *tasksState) act(e *env.Env, page *pageState, task protocol.TaskInfo) {
	if len(task.Sessions) > 0 && task.Worktree != "" {
		e.Navigate(protocol.TabAgent, task.Sessions[len(task.Sessions)-1])
		return
	}
	var result protocol.TaskStartResult
	e.Call(protocol.MTasksStart, protocol.TaskStartParams{ID: task.ID, Harness: task.Harness}, &result, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		page.loadTasks(e)
		e.LoadWorktrees(page.project, nil)
		e.Navigate(protocol.TabAgent, result.Session.ID)
	})
}

func (t *tasksState) updateStatus(e *env.Env, page *pageState, id, status string) {
	params := protocol.TaskUpdateParams{ID: id, Patch: rawPatch(map[string]string{"status": status})}
	e.Call(protocol.MTasksUpdate, params, new(protocol.TaskInfo), func(err error) {
		page.fail(err)
		page.loadTasks(e)
	})
}

func taskByID(tasks []protocol.TaskInfo, id string) (protocol.TaskInfo, bool) {
	for _, task := range tasks {
		if task.ID == id {
			return task, true
		}
	}
	return protocol.TaskInfo{}, false
}

func unfinishedDeps(task protocol.TaskInfo, tasks []protocol.TaskInfo) int {
	waiting := 0
	for _, dependency := range task.Deps {
		if other, ok := taskByID(tasks, dependency); ok && other.Status != protocol.TaskDone {
			waiting++
		}
	}
	return waiting
}

func openGateOf(task protocol.TaskInfo) *protocol.Gate {
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
