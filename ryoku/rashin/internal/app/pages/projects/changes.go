package projects

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/anim"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

const (
	changeConflicted = iota
	changeStaged
	changeUnstaged
	changeUntracked
	changeGroups
)

const (
	scopeUncommitted = iota
	scopeBranch
)

type changesState struct {
	worktree look.Select
	scope    kit.Filters
	files    look.Scroller
	rows     map[string]*changeRow
	groups   [changeGroups]changeGroup
	view     diffview.View

	status                 protocol.GitStatusResult
	diff                   protocol.DiffResult
	loadedPath, loadedMode string
	loading                bool
	selectedPath           string

	message                  kit.TextField
	amend                    kit.Toggle
	commit, push, pull       kit.Plaque
	fetch, review            kit.Plaque
	abort, proceed, resolver kit.Plaque
	discard                  kit.Confirm
	discardFiles             []string
	force                    kit.Confirm
}

type changeRow struct {
	click          kit.Click
	hover          look.Hover
	press          anim.Tween
	stage, discard look.IconAction
	tips           [2]look.Tip
}

type changeGroup struct {
	hover          look.Hover
	stage, discard look.IconAction
	tips           [2]look.Tip
}

type changeItem struct {
	group  int
	header bool
	file   protocol.FileChange
}

func (c *changesState) init() { c.rows = make(map[string]*changeRow) }

func (c *changesState) layout(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	worktree, ok := page.selectedWorktree(e)
	if !ok {
		return kit.Empty{Icon: look.Icon.Diff, Title: i18n.T("No worktrees"), Body: i18n.T("Changes are read from a worktree.")}.Layout(gtx, th.Theme)
	}
	page.worktree = worktree.Path
	c.handleActions(gtx, e, page)
	if mode := c.diffMode(); c.loadedPath != page.worktree || c.loadedMode != mode {
		c.load(e, page, mode)
	}
	files := func(gtx layout.Context) layout.Dimensions { return c.layoutFiles(gtx, e, page) }
	diff := func(gtx layout.Context) layout.Dimensions { return c.layoutDiff(gtx, e, page) }
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutBar(gtx, e, page, worktree) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if gtx.Constraints.Max.X < gtx.Dp(overviewSplit) {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(300))
						return files(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
					layout.Flexed(1, diff),
				)
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					width := gtx.Dp(340)
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
					return files(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := image.Pt(max(1, gtx.Dp(1)), gtx.Constraints.Max.Y)
					look.FillRect(gtx, image.Rectangle{Max: size}, th.R.Vein)
					return layout.Dimensions{Size: size}
				}),
				layout.Flexed(1, diff),
			)
		}),
	)
}

func (c *changesState) layoutBar(gtx layout.Context, e *env.Env, page *pageState, worktree protocol.WorktreeInfo) layout.Dimensions {
	th := e.Theme
	worktrees := e.Snapshot().Worktrees[page.project]
	labels, selected := make([]string, len(worktrees)), 0
	for index, candidate := range worktrees {
		labels[index] = candidate.Name
		if candidate.Path == worktree.Path {
			selected = index
		}
	}
	c.worktree.Form, c.worktree.Icon, c.worktree.Label = look.SelectQuiet, look.Icon.Branch, i18n.T("Worktree")
	c.scope.Labels = []string{i18n.T("Uncommitted"), i18n.T("Whole branch")}
	status := c.status.Status
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := c.worktree.Layout(gtx, th, e.Overlays, labels, selected)
			if index, changed := c.worktree.Changed(); changed && index < len(worktrees) {
				c.selectedPath = ""
				e.NavigateWork(page.project, TabChanges, worktrees[index].Path)
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(look.TokenChip(th, worktreeBranch(worktree))),
	}
	if status.Upstream != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(kit.Text(th.Theme, th.T.MonoSm, th.R.Fg3, fmt.Sprintf("↑%d ↓%d", status.Ahead, status.Behind))),
		)
	}
	if status.Operation != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
			layout.Rigid(kit.Claim(th.Theme, palette.Attention, i18n.Tf("%s in progress", status.Operation))),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutOperation(gtx, th) }),
		)
	}
	children = append(children,
		layout.Flexed(1, layout.Spacer{}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := c.scope.Layout(gtx, th.Theme)
			if _, changed := c.scope.Changed(); changed {
				c.selectedPath, c.loadedMode = "", ""
			}
			return dimensions
		}),
	)
	return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S2, Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (c *changesState) layoutOperation(gtx layout.Context, th *look.Theme) layout.Dimensions {
	c.proceed.Kind, c.proceed.Small, c.proceed.Label = kit.PlaqueLine, true, i18n.T("Continue")
	c.resolver.Kind, c.resolver.Small, c.resolver.Label = kit.PlaqueQuiet, true, i18n.T("Resolve with an agent")
	c.abort.Kind, c.abort.Small, c.abort.Label = kit.PlaqueQuiet, true, i18n.T("Abort")
	plaques := [...]*kit.Plaque{&c.proceed, &c.resolver, &c.abort}
	return look.Flow{Gap: 4}.Layout(gtx, len(plaques), func(gtx layout.Context, index int) layout.Dimensions {
		return plaques[index].Layout(gtx, th.Theme)
	})
}

func (c *changesState) diffMode() string {
	if c.scope.Selected == scopeBranch {
		return protocol.DiffBase
	}
	for _, file := range c.status.Files {
		if file.Path == c.selectedPath && file.Staged {
			return protocol.DiffStaged
		}
	}
	return protocol.DiffWorking
}

func (c *changesState) load(e *env.Env, page *pageState, mode string) {
	if c.loading || page.worktree == "" {
		return
	}
	c.loading, c.loadedPath, c.loadedMode = true, page.worktree, mode
	worktree := page.worktree
	var status protocol.GitStatusResult
	e.Call(protocol.MGitStatus, protocol.WorktreeRef{Path: worktree}, &status, func(err error) {
		if err != nil {
			c.loading = false
			page.fail(err)
			return
		}
		c.status = status
		if c.scope.Selected == scopeUncommitted && !c.hasFile(c.selectedPath) && len(status.Files) > 0 {
			c.selectedPath = c.items()[1].file.Path
		}
		params := protocol.DiffParams{Project: page.project, Worktree: worktree, Mode: c.diffMode(), Context: 4}
		c.loadedMode = params.Mode
		var result protocol.DiffResult
		e.Call(protocol.MGitDiff, params, &result, func(err error) {
			c.loading = false
			if err != nil {
				page.fail(err)
				return
			}
			c.diff = result
			if c.scope.Selected == scopeBranch && !diffHas(result, c.selectedPath) && len(result.Files) > 0 {
				c.selectedPath = result.Files[0].Path
			}
		})
	})
	e.LoadReviews(worktree, nil)
}

func (c *changesState) hasFile(path string) bool {
	for _, file := range c.status.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func diffHas(result protocol.DiffResult, path string) bool {
	for _, file := range result.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func (c *changesState) items() []changeItem {
	if c.scope.Selected == scopeBranch {
		items := make([]changeItem, 0, len(c.diff.Files))
		for _, file := range c.diff.Files {
			change := protocol.FileChange{Path: file.Path, OldPath: file.OldPath, Status: file.Status, Added: file.Added, Removed: file.Removed}
			items = append(items, changeItem{group: -1, file: change})
		}
		return items
	}
	var groups [changeGroups][]protocol.FileChange
	for _, file := range c.status.Files {
		groups[changeGroupOf(file)] = append(groups[changeGroupOf(file)], file)
	}
	items := make([]changeItem, 0, len(c.status.Files)+changeGroups)
	for group, files := range groups {
		if len(files) == 0 {
			continue
		}
		items = append(items, changeItem{group: group, header: true})
		for _, file := range files {
			items = append(items, changeItem{group: group, file: file})
		}
	}
	return items
}

func changeGroupOf(file protocol.FileChange) int {
	switch {
	case file.Status == protocol.ChangeConflict:
		return changeConflicted
	case file.Staged:
		return changeStaged
	case file.Status == protocol.ChangeUntracked:
		return changeUntracked
	default:
		return changeUnstaged
	}
}

func (c *changesState) groupFiles(group int) []string {
	paths := make([]string, 0, 8)
	for _, file := range c.status.Files {
		if changeGroupOf(file) == group {
			paths = append(paths, file.Path)
		}
	}
	return paths
}

func (c *changesState) layoutFiles(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	items := c.items()
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			if len(items) == 0 {
				return c.layoutNoFiles(gtx, th)
			}
			return layout.Inset{Top: th.Sp.S1, Left: th.Sp.S2, Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				c.files.Gap = gtx.Dp(2)
				return c.files.Layout(gtx, th, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					item := items[index]
					if item.header {
						return c.layoutGroup(gtx, e, page, item.group)
					}
					return c.layoutFile(gtx, e, page, item)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if c.scope.Selected == scopeBranch {
				return layout.Dimensions{}
			}
			return c.layoutCommit(gtx, e)
		}),
	)
}

func (c *changesState) layoutNoFiles(gtx layout.Context, th *look.Theme) layout.Dimensions {
	body := i18n.T("Everything in this worktree is committed.")
	if c.scope.Selected == scopeBranch {
		body = i18n.T("This branch has nothing its base does not.")
	}
	if c.loading {
		return look.SkeletonRows(gtx, th, 5, false)
	}
	return kit.Empty{Icon: look.Icon.Check, Title: i18n.T("Nothing changed"), Body: body}.Layout(gtx, th.Theme)
}

func changeGroupTitle(group int) string {
	switch group {
	case changeConflicted:
		return i18n.T("In conflict")
	case changeStaged:
		return i18n.T("Staged")
	case changeUntracked:
		return i18n.T("Untracked")
	default:
		return i18n.T("Changed")
	}
}

func (c *changesState) layoutGroup(gtx layout.Context, e *env.Env, page *pageState, group int) layout.Dimensions {
	th := e.Theme
	state := &c.groups[group]
	paths := c.groupFiles(group)
	stageTip, stageIcon, method := i18n.T("Stage all"), look.Icon.Add, protocol.MGitStage
	if group == changeStaged {
		stageTip, stageIcon, method = i18n.T("Unstage all"), look.Icon.Remove, protocol.MGitUnstage
	}
	state.stage.Icon, state.stage.Description = stageIcon, stageTip
	state.discard.Icon, state.discard.Description = look.Icon.Undo, i18n.T("Discard all")
	if state.stage.Clicked(gtx) {
		c.gitPaths(e, page, method, paths)
	}
	if state.discard.Clicked(gtx) {
		c.askDiscard(paths)
	}
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(32))
	gtx.Constraints = layout.Exact(size)
	_, hover := state.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		look.MiddleLeft(gtx, gtx.Dp(8), size.Y, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
				layout.Rigid(kit.RawText(th.Theme, th.T.Group, th.R.Fg3, changeGroupTitle(group))),
				layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
				layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg3, fmt.Sprint(len(paths)))),
			)
		})
		return layout.Dimensions{Size: size}
	})
	if hover > 0 {
		box := gtx.Dp(28)
		actions := []*look.IconAction{&state.stage}
		if group != changeStaged {
			actions = append(actions, &state.discard)
		}
		for index, action := range actions {
			at := image.Pt(size.X-gtx.Dp(2)-(len(actions)-index)*box, (size.Y-box)/2)
			look.Place(gtx, image.Rectangle{Min: at, Max: at.Add(image.Pt(box, box))}, func(gtx layout.Context) layout.Dimensions {
				return state.tips[index].Layout(gtx, th, e.Overlays, action.Description, func(gtx layout.Context) layout.Dimensions {
					return action.Layout(gtx, th, th.R.Fg2)
				})
			})
		}
	}
	return layout.Dimensions{Size: size}
}

func (c *changesState) layoutFile(gtx layout.Context, e *env.Env, page *pageState, item changeItem) layout.Dimensions {
	th := e.Theme
	file := item.file
	key := fmt.Sprint(item.group, "\x00", file.Path)
	row := c.rows[key]
	if row == nil {
		row = new(changeRow)
		c.rows[key] = row
	}
	if row.click.Clicked(gtx) {
		c.selectedPath, c.loadedMode = file.Path, ""
	}
	stageTip, stageIcon, method := i18n.T("Stage"), look.Icon.Add, protocol.MGitStage
	if file.Staged {
		stageTip, stageIcon, method = i18n.T("Unstage"), look.Icon.Remove, protocol.MGitUnstage
	}
	row.stage.Icon, row.stage.Description = stageIcon, stageTip
	row.discard.Icon, row.discard.Description = look.Icon.Undo, i18n.T("Discard changes")
	if row.stage.Clicked(gtx) {
		c.gitPaths(e, page, method, []string{file.Path})
	}
	if row.discard.Clicked(gtx) {
		c.askDiscard([]string{file.Path})
	}
	selected := file.Path == c.selectedPath && (item.group < 0 || changeGroupOf(file) == item.group)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(32))
	gtx.Constraints = layout.Exact(size)
	record := op.Record(gtx.Ops)
	_, hover := row.hover.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return row.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			c.drawFile(gtx, e, row, item, selected)
			return layout.Dimensions{Size: size}
		})
	})
	call := record.Stop()
	bounds := image.Rectangle{Max: size}
	cut := gtx.Dp(th.Cut.Small)
	switch {
	case selected:
		kit.FillCut(gtx.Ops, bounds, cut, th.R.Sel)
	case hover > 0:
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Stone3, hover))
	}
	if press := look.Transition(gtx, &row.press, row.click.Pressed(), th.Motion.Press, th.Motion.Press); press > 0 {
		kit.FillCut(gtx.Ops, bounds, cut, palette.WithAlpha(th.R.Fg, .08*press))
	}
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

func (c *changesState) drawFile(gtx layout.Context, e *env.Env, row *changeRow, item changeItem, selected bool) {
	th := e.Theme
	file := item.file
	size := gtx.Constraints.Max
	letter, letterInk := diffview.StatusMark(th, file.Status)
	ink := th.R.Fg
	if selected {
		ink, letterInk = th.R.OnSel, th.R.OnSel
	}
	look.MiddleLeft(gtx, gtx.Dp(8), size.Y, kit.RawText(th.Theme, th.T.Mono, letterInk, letter))
	right := size.X - gtx.Dp(8)
	if row.hover.Hovered() && item.group >= 0 {
		box := gtx.Dp(28)
		actions := [...]*look.IconAction{&row.stage, &row.discard}
		right -= len(actions)*box - gtx.Dp(6)
		for index, action := range actions {
			at := image.Pt(right+index*box, (size.Y-box)/2)
			look.Place(gtx, image.Rectangle{Min: at, Max: at.Add(image.Pt(box, box))}, func(gtx layout.Context) layout.Dimensions {
				return row.tips[index].Layout(gtx, th, e.Overlays, action.Description, func(gtx layout.Context) layout.Dimensions {
					return action.Layout(gtx, th, ink)
				})
			})
		}
	} else if file.Added+file.Removed > 0 {
		record := op.Record(gtx.Ops)
		counts := gtx
		counts.Constraints = layout.Constraints{Max: image.Pt(size.X/2, size.Y)}
		dimensions := look.DiffCount(th, file.Added, file.Removed, selected)(counts)
		call := record.Stop()
		right -= dimensions.Size.X
		offset := op.Offset(image.Pt(right, (size.Y-dimensions.Size.Y)/2)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
	}
	left := gtx.Dp(28)
	room := max(0, right-gtx.Dp(8)-left)
	character := max(1, gtx.Sp(th.T.Mono.Size)*3/5)
	label := gtx
	label.Constraints = layout.Constraints{Max: image.Pt(room, size.Y)}
	look.MiddleLeft(label, left, size.Y, kit.RawText(th.Theme, th.T.Mono, ink, look.ElidePath(file.Path, room/character)))
}

func (c *changesState) layoutDiff(gtx layout.Context, e *env.Env, page *pageState) layout.Dimensions {
	th := e.Theme
	gtx.Constraints.Min = gtx.Constraints.Max
	result := protocol.DiffResult{Truncated: c.diff.Truncated}
	for _, file := range c.diff.Files {
		if file.Path == c.selectedPath {
			result.Files = append(result.Files, file)
		}
	}
	if len(result.Files) == 0 {
		if c.selectedPath == "" || c.loading {
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}
		return kit.Empty{
			Icon: look.Icon.File, Title: i18n.T("Nothing to show as text"),
			Body: i18n.T("This file is binary, or it changed in a way that has no lines to compare."),
		}.Layout(gtx, th.Theme)
	}
	options := diffview.Options{
		Overlays: e.Overlays, Comments: e.Snapshot().Reviews[page.worktree], Commentable: true,
		FoldOver: 400, Headless: true,
	}
	layout.Inset{Top: th.Sp.S2, Left: th.Sp.S2, Right: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return c.view.Layout(gtx, th, result, options)
	})
	c.handleDiffActions(e, page, c.view.Actions())
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (c *changesState) layoutCommit(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	c.message.Placeholder = i18n.T("Describe what is staged")
	c.amend.Label = i18n.T("Amend")
	c.commit.Kind, c.commit.Small, c.commit.Label = kit.PlaqueGold, true, i18n.T("Commit")
	c.fetch.Kind, c.fetch.Small, c.fetch.Label = kit.PlaqueQuiet, true, i18n.T("Fetch")
	c.pull.Kind, c.pull.Small, c.pull.Label = kit.PlaqueQuiet, true, i18n.T("Pull")
	c.push.Kind, c.push.Small, c.push.Label = kit.PlaqueQuiet, true, i18n.T("Push")
	c.review.Kind, c.review.Small, c.review.Label = kit.PlaqueLine, true, i18n.T("Send comments to the agent")
	staged := len(c.groupFiles(changeStaged)) > 0 || c.amend.Value
	ready := staged && strings.TrimSpace(c.message.Text()) != ""
	moves := [...]*kit.Plaque{&c.fetch, &c.pull, &c.push}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Rule(gtx, th) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(th.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.message.Layout(gtx, th.Theme) }),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.amend.Layout(gtx, th.Theme) }),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !ready {
									gtx = gtx.Disabled()
								}
								return c.commit.Layout(gtx, th.Theme)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.Flow{Gap: 4, LineGap: 4}.Layout(gtx, len(moves)+1, func(gtx layout.Context, index int) layout.Dimensions {
							if index < len(moves) {
								return moves[index].Layout(gtx, th.Theme)
							}
							return layout.Inset{Left: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return c.review.Layout(gtx, th.Theme)
							})
						})
					}),
				)
			})
		}),
	)
}

func (c *changesState) handleActions(gtx layout.Context, e *env.Env, page *pageState) {
	reload := func(err error) {
		page.fail(err)
		c.loadedPath = ""
	}
	worktree := protocol.WorktreeRef{Path: page.worktree}
	if c.commit.Clicked(gtx) {
		if message := strings.TrimSpace(c.message.Text()); message != "" {
			params := protocol.GitCommitWriteParams{Path: page.worktree, Message: message, Amend: c.amend.Value}
			e.Call(protocol.MGitCommit, params, new(protocol.GitOpResult), func(err error) {
				if err == nil {
					c.message.SetText("")
					c.amend.Value = false
					e.Notify(i18n.T("Committed."))
				}
				reload(err)
			})
		}
	}
	if c.push.Clicked(gtx) {
		// behind its upstream means replacing what's there, so it takes asking by name
		if c.status.Status.Behind > 0 {
			c.force.Danger, c.force.Dialog.Open = true, true
		} else {
			c.pushNow(e, page, false)
		}
	}
	if c.fetch.Clicked(gtx) {
		e.Call(protocol.MGitFetch, protocol.ProjectRef{Project: page.project}, new(protocol.GitOpResult), reload)
	}
	if c.pull.Clicked(gtx) {
		e.Call(protocol.MGitPull, worktree, new(protocol.GitOpResult), reload)
	}
	if c.review.Clicked(gtx) {
		var info protocol.SessionInfo
		e.Call(protocol.MReviewSend, protocol.ReviewSendParams{Worktree: page.worktree}, &info, func(err error) {
			if err != nil {
				page.fail(err)
				return
			}
			e.Attach(info.ID, nil)
			e.Navigate(protocol.TabAgent, info.ID)
		})
	}
	if c.abort.Clicked(gtx) {
		e.Call(protocol.MGitAbort, worktree, new(protocol.GitOpResult), reload)
	}
	if c.proceed.Clicked(gtx) {
		e.Call(protocol.MGitContinue, worktree, new(protocol.GitOpResult), reload)
	}
	if c.resolver.Clicked(gtx) {
		c.resolveWithAgent(e, page)
	}
}

func (c *changesState) askDiscard(paths []string) {
	c.discardFiles = paths
	c.discard.Danger, c.discard.Dialog.Open = true, true
}

func (c *changesState) layoutConfirms(gtx layout.Context, e *env.Env, page *pageState) {
	if len(c.discardFiles) > 0 {
		title := i18n.Tf("Discard changes to %d files?", len(c.discardFiles))
		if len(c.discardFiles) == 1 {
			title = i18n.Tf("Discard changes to %s?", filepath.Base(c.discardFiles[0]))
		}
		if c.discard.Layout(gtx, e.Theme.Theme, title, i18n.T("What you changed is lost. This cannot be undone."), i18n.T("Discard"), "") {
			c.gitPaths(e, page, protocol.MGitDiscard, c.discardFiles)
		}
		if c.discard.Closed() {
			c.discardFiles = nil
		}
	}
	if c.force.Dialog.Open {
		body := i18n.T("The branch is behind its upstream. Pushing replaces the commits that are there, unless someone pushed since you last fetched.")
		if c.force.Layout(gtx, e.Theme.Theme, i18n.T("Replace the upstream branch?"), body, i18n.T("Force push"), c.status.Branch) {
			c.pushNow(e, page, true)
		}
	}
}

func (c *changesState) handleDiffActions(e *env.Env, page *pageState, actions []diffview.Action) {
	refresh := func(err error) {
		page.fail(err)
		e.LoadReviews(page.worktree, nil)
	}
	for _, action := range actions {
		switch action.Kind {
		case diffview.ActComment:
			params := protocol.ReviewAddParams{Worktree: page.worktree, Path: action.Path, Line: action.Line, Side: action.Side, Text: action.Text}
			e.Call(protocol.MReviewAdd, params, new(protocol.ReviewComment), refresh)
		case diffview.ActResolve:
			resolved := false
			for _, comment := range e.Snapshot().Reviews[page.worktree] {
				if comment.ID == action.ID {
					resolved = comment.Resolved
				}
			}
			params := protocol.ReviewRef{Worktree: page.worktree, ID: action.ID, Resolved: !resolved}
			e.Call(protocol.MReviewResolve, params, new(protocol.ReviewComment), refresh)
		case diffview.ActDelete:
			e.Call(protocol.MReviewDelete, protocol.ReviewRef{Worktree: page.worktree, ID: action.ID}, new(protocol.Empty), refresh)
		case diffview.ActStage:
			c.gitPaths(e, page, protocol.MGitStage, []string{action.Path})
		case diffview.ActUnstage:
			c.gitPaths(e, page, protocol.MGitUnstage, []string{action.Path})
		case diffview.ActDiscard:
			c.askDiscard([]string{action.Path})
		case diffview.ActOpen:
			env.Spawn("xdg-open", filepath.Join(page.worktree, action.Path))
		}
	}
}

func (c *changesState) gitPaths(e *env.Env, page *pageState, method string, files []string) {
	var result protocol.GitStatusResult
	e.Call(method, protocol.GitPathsParams{Path: page.worktree, Files: files}, &result, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		c.status, c.loadedPath = result, ""
	})
}

func (c *changesState) pushNow(e *env.Env, page *pageState, force bool) {
	params := protocol.GitPushParams{Path: page.worktree, SetUpstream: c.status.Status.Upstream == "", ForceWithLease: force}
	e.Call(protocol.MGitPush, params, new(protocol.GitOpResult), func(err error) {
		if err == nil {
			e.Notify(i18n.T("Pushed."))
		}
		page.fail(err)
		c.loadedPath = ""
	})
}

func (c *changesState) resolveWithAgent(e *env.Env, page *pageState) {
	prompt := i18n.Tf("Resolve the %s conflicts in this worktree. Conflicted files:\n%s",
		c.status.Status.Operation, strings.Join(c.groupFiles(changeConflicted), "\n"))
	send := func(session string) {
		e.Submit(session, false, chat.Submission{Text: prompt})
		e.Attach(session, nil)
		e.Navigate(protocol.TabAgent, session)
	}
	for _, info := range e.Snapshot().Sessions {
		if info.Worktree == page.worktree && info.Kind == protocol.KindAgent {
			send(info.ID)
			return
		}
	}
	var info protocol.SessionInfo
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Origin: protocol.OriginApp, Project: page.project, Worktree: page.worktree}
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		send(info.ID)
	})
}
