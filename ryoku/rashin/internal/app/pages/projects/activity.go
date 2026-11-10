package projects

import (
	"image"
	"strings"
	"time"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

type activityState struct {
	scroll  look.Scroller
	items   []protocol.Activity
	loaded  string
	loading bool
}

type activityColumns struct {
	time, actor, action, result int
}

func (a *activityState) layout(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	if a.loaded != page.project && !a.loading {
		a.loading = true
		project := page.project
		var result protocol.ActivityResult
		e.Call(protocol.MActivityList, protocol.ActivityParams{Project: project, Limit: 300}, &result, func(err error) {
			a.loading = false
			if err != nil {
				page.fail(err)
				return
			}
			a.items, a.loaded = result.Activity, project
		})
	}
	if len(a.items) == 0 {
		if a.loading {
			return layout.Inset{Top: th.Sp.S3, Left: th.Sp.S4, Right: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.SkeletonRows(gtx, th, 8, false)
			})
		}
		return kit.Empty{
			Icon: look.Icon.Activity, Title: i18n.T("Nothing has happened yet"),
			Body: i18n.T("Commits, worktrees made and removed, reviews and task changes are recorded here."),
		}.Layout(gtx, th.Theme)
	}
	columns := activityColumns{time: gtx.Dp(104), actor: gtx.Dp(176), action: gtx.Dp(220), result: gtx.Dp(88)}
	if gtx.Constraints.Max.X < gtx.Dp(820) {
		columns = activityColumns{time: gtx.Dp(92), actor: gtx.Dp(112), action: gtx.Dp(168), result: gtx.Dp(80)}
	}
	names := activityNames{snapshot: e.Snapshot(), project: page.project, home: paths.Home()}
	return layout.Inset{Left: th.Sp.S4, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				head := [...]string{i18n.T("When"), i18n.T("Who"), i18n.T("What"), i18n.T("On"), ""}
				dimensions := layoutActivityRow(gtx, columns, gtx.Dp(36), func(column int) layout.Widget {
					return kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, head[column])
				})
				look.FillRect(gtx, image.Rect(0, dimensions.Size.Y-max(1, gtx.Dp(1)), dimensions.Size.X, dimensions.Size.Y), th.R.Vein)
				return dimensions
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.scroll.Layout(gtx, th, len(a.items), func(gtx layout.Context, index int) layout.Dimensions {
					item := a.items[index]
					return layoutActivityRow(gtx, columns, gtx.Dp(40), func(column int) layout.Widget {
						return activityCell(th, gtx.Now, names, item, column)
					})
				})
			}),
		)
	})
}

func layoutActivityRow(gtx layout.Context, columns activityColumns, height int, cell func(int) layout.Widget) layout.Dimensions {
	width := gtx.Constraints.Max.X
	gap := gtx.Dp(13)
	widths := [...]int{columns.time, columns.actor, columns.action, 0, columns.result}
	widths[3] = max(0, width-columns.time-columns.actor-columns.action-columns.result-4*gap)
	x := 0
	for column, cellWidth := range widths {
		child := gtx
		child.Constraints = layout.Constraints{Max: image.Pt(cellWidth, height)}
		offset := op.Offset(image.Pt(x, 0)).Push(gtx.Ops)
		look.MiddleLeft(child, 0, height, cell(column))
		offset.Pop()
		x += cellWidth + gap
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func activityCell(th *look.Theme, now time.Time, names activityNames, item protocol.Activity, column int) layout.Widget {
	switch column {
	case 0:
		return kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, activityTime(now, item.Time))
	case 1:
		return kit.RawText(th.Theme, th.T.Body, th.R.Fg2, names.actor(item.Actor))
	case 2:
		return kit.RawText(th.Theme, th.T.Body, th.R.Fg, activityWords(item.Action))
	case 3:
		return func(gtx layout.Context) layout.Dimensions {
			target, literal := names.target(item)
			style := th.T.Body
			if literal {
				style = th.T.Mono
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(kit.RawText(th.Theme, style, th.R.Fg2, target)),
				layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
				layout.Flexed(1, kit.RawText(th.Theme, th.T.Caption, th.R.Fg3, item.Detail)),
			)
		}
	default:
		// a step that worked needs no word, only a failure is a state
		if item.OK {
			return func(layout.Context) layout.Dimensions { return layout.Dimensions{} }
		}
		return kit.Claim(th.Theme, palette.Failed, i18n.T("Failed"))
	}
}

type activityNames struct {
	snapshot appstate.Snapshot
	project  string
	home     string
}

func (names activityNames) actor(actor string) string {
	if actor == "you" || actor == "" {
		return i18n.T("You")
	}
	if session, ok := names.snapshot.Sessions[actor]; ok && session.Title != "" {
		return session.Title
	}
	return actor
}

func (names activityNames) target(item protocol.Activity) (name string, literal bool) {
	target := item.Target
	if strings.HasPrefix(item.Action, "task.") {
		for _, task := range names.snapshot.Tasks[names.project] {
			if task.ID == target {
				return task.Title, false
			}
		}
		return target, false
	}
	for _, worktree := range names.snapshot.Worktrees[names.project] {
		if worktree.Path == target && worktree.Name != "" {
			return worktree.Name, false
		}
	}
	for _, project := range names.snapshot.Projects {
		if (project.ID == target || project.Root == target) && project.Name != "" {
			return project.Name, false
		}
	}
	return shortPath(target, names.home), true
}

// an action the app doesn't know yet keeps its own name
func activityWords(action string) string {
	switch action {
	case "project.add":
		return i18n.T("Added the project")
	case "project.settings":
		return i18n.T("Changed the settings")
	case "worktree.create":
		return i18n.T("Made a worktree")
	case "worktree.remove":
		return i18n.T("Removed a worktree")
	case "worktree.move":
		return i18n.T("Moved a worktree")
	case "worktree.lock":
		return i18n.T("Locked a worktree")
	case "worktree.prune":
		return i18n.T("Pruned missing worktrees")
	case "worktree.cleanup":
		return i18n.T("Cleaned up finished worktrees")
	case "worktree.repair":
		return i18n.T("Repaired the worktrees")
	case "git.stage":
		return i18n.T("Staged changes")
	case "git.unstage":
		return i18n.T("Unstaged changes")
	case "git.discard":
		return i18n.T("Discarded changes")
	case "git.commit":
		return i18n.T("Committed")
	case "git.push":
		return i18n.T("Pushed")
	case "git.pull":
		return i18n.T("Pulled")
	case "git.fetch":
		return i18n.T("Fetched")
	case "git.merge":
		return i18n.T("Merged")
	case "git.rebase":
		return i18n.T("Rebased")
	case "git.revert":
		return i18n.T("Reverted a commit")
	case "git.checkout":
		return i18n.T("Checked out")
	case "git.continue":
		return i18n.T("Continued after conflicts")
	case "git.abort":
		return i18n.T("Gave up an operation")
	case "review.add":
		return i18n.T("Commented on a line")
	case "review.resolve":
		return i18n.T("Resolved a comment")
	case "review.delete":
		return i18n.T("Deleted a comment")
	case "review.send":
		return i18n.T("Sent comments to the agent")
	case "task.create":
		return i18n.T("Wrote a task")
	case "task.update":
		return i18n.T("Changed a task")
	case "task.delete":
		return i18n.T("Deleted a task")
	case "task.start":
		return i18n.T("Started a task")
	case "task.gate.open":
		return i18n.T("Asked for a decision")
	case "task.gate.resolve":
		return i18n.T("Decided")
	default:
		return action
	}
}

func activityTime(now, when time.Time) string {
	now, when = now.Local(), when.Local()
	if now.Year() == when.Year() && now.YearDay() == when.YearDay() {
		return when.Format("15:04")
	}
	return when.Format("2 Jan 15:04")
}
