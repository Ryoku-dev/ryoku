package projects

import (
	"path/filepath"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

const (
	changeStaged = iota
	changeUnstaged
	changeUntracked
	changeConflicted
)

type changeListItem struct {
	group int
	file  *protocol.FileChange
}

func (c *changesState) layout(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	worktrees := e.Snapshot().Worktrees[page.project]
	if len(worktrees) == 0 {
		empty := kit.Empty{
			Title: i18n.T("No worktrees"),
			Body:  i18n.T("Create a worktree to review changes."),
		}
		return empty.Layout(gtx, e.Theme.Theme)
	}
	selected := selectedWorktree(worktrees, page.worktree)
	if page.worktree == "" {
		page.worktree = worktrees[selected].Path
	}
	c.handleActions(gtx, e, page)
	mode := c.selectedDiffMode()
	if c.loadedPath != page.worktree || c.loadedMode != mode {
		c.load(e, page, mode)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutHeader(gtx, e, page, worktrees, selected)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return c.layoutReview(gtx, e, page)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutGitActions(gtx, e, page)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return c.layoutConfirms(gtx, e, page)
		}),
	)
}

func selectedWorktree(worktrees []protocol.WorktreeInfo, path string) int {
	for index, worktree := range worktrees {
		if worktree.Path == path {
			return index
		}
	}
	return 0
}

func (c *changesState) layoutHeader(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktrees []protocol.WorktreeInfo,
	selected int,
) layout.Dimensions {
	labels := make([]string, len(worktrees))
	for index, worktree := range worktrees {
		labels[index] = worktree.Name + " · " + worktree.Branch
	}
	c.modeTabs.Labels = []string{i18n.T("Working tree"), i18n.T("Branch vs base")}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(.32, func(gtx layout.Context) layout.Dimensions {
			dimensions := c.worktreeChoice.Layout(gtx, e.Theme, e.Overlays, labels, selected)
			if index, changed := c.worktreeChoice.Changed(); changed &&
				index >= 0 && index < len(worktrees) {
				page.worktree = worktrees[index].Path
				c.selectedPath = ""
				c.loadedPath = ""
				e.LoadReviews(page.worktree, nil)
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Flexed(.68, func(gtx layout.Context) layout.Dimensions {
			return c.layoutStatus(gtx, e)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := c.modeTabs.Layout(gtx, e.Theme.Theme)
			if _, changed := c.modeTabs.Changed(); changed {
				c.selectedPath = ""
				c.loadedMode = ""
			}
			return dimensions
		}),
	)
}

func (c *changesState) load(
	e *env.Env,
	page *pageState,
	mode string,
) {
	if c.loading || page.worktree == "" {
		return
	}
	c.loading, c.loadedPath, c.loadedMode = true, page.worktree, mode
	var status protocol.GitStatusResult
	e.Call(
		protocol.MGitStatus,
		protocol.WorktreeRef{Path: page.worktree},
		&status,
		func(err error) {
			if err != nil {
				c.error = err.Error()
				c.loading = false
				return
			}
			c.status = status
			if c.modeTabs.Selected == 0 && !c.statusHasPath(c.selectedPath) {
				c.selectedPath = firstChangedPath(status.Files)
			}
			params := protocol.DiffParams{
				Project:  page.project,
				Worktree: page.worktree,
				Mode:     mode,
				Context:  4,
			}
			var result protocol.DiffResult
			e.Call(protocol.MGitDiff, params, &result, func(err error) {
				c.loading = false
				if err != nil {
					c.error = err.Error()
					return
				}
				c.diff = result
				if c.modeTabs.Selected == 1 && !diffHasPath(result, c.selectedPath) {
					c.selectedPath = firstDiffPath(result)
				}
			})
		},
	)
	e.LoadReviews(page.worktree, nil)
}

func firstChangedPath(files []protocol.FileChange) string {
	for _, file := range files {
		return file.Path
	}
	return ""
}

func firstDiffPath(result protocol.DiffResult) string {
	for _, file := range result.Files {
		return file.Path
	}
	return ""
}

func diffHasPath(result protocol.DiffResult, path string) bool {
	for _, file := range result.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func (c *changesState) statusHasPath(path string) bool {
	for _, file := range c.status.Files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func (c *changesState) selectedDiffMode() string {
	if c.modeTabs.Selected == 1 {
		return protocol.DiffBase
	}
	for _, file := range c.status.Files {
		if file.Path == c.selectedPath && file.Staged {
			return protocol.DiffStaged
		}
	}
	return protocol.DiffWorking
}

func (c *changesState) handleActions(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) {
	if c.commit.Clicked(gtx) {
		message := strings.TrimSpace(c.commitMessage.Text())
		if message != "" {
			var result protocol.GitOpResult
			params := protocol.GitCommitWriteParams{
				Path:    page.worktree,
				Message: message,
				Amend:   c.amend.Value,
			}
			e.Call(protocol.MGitCommit, params, &result, func(err error) {
				if err != nil {
					c.error = err.Error()
					return
				}
				c.commitMessage.SetText("")
				c.loadedPath = ""
			})
		}
	}
	if c.push.Clicked(gtx) {
		if c.status.Status.Behind > 0 {
			c.forcePush, c.pushConfirm.Danger, c.pushConfirm.Dialog.Open = true, true, true
		} else {
			c.pushNow(e, page, false)
		}
	}
	if c.fetch.Clicked(gtx) {
		var result protocol.GitOpResult
		e.Call(
			protocol.MGitFetch,
			protocol.ProjectRef{Project: page.project},
			&result,
			func(err error) {
				if err != nil {
					c.error = err.Error()
				}
				c.loadedPath = ""
			},
		)
	}
	if c.pull.Clicked(gtx) {
		var result protocol.GitOpResult
		e.Call(
			protocol.MGitPull,
			protocol.WorktreeRef{Path: page.worktree},
			&result,
			func(err error) {
				if err != nil {
					c.error = err.Error()
				}
				c.loadedPath = ""
			},
		)
	}
	if c.sendReview.Clicked(gtx) {
		var info protocol.SessionInfo
		e.Call(
			protocol.MReviewSend,
			protocol.ReviewSendParams{Worktree: page.worktree},
			&info,
			func(err error) {
				if err != nil {
					c.error = err.Error()
					return
				}
				e.Attach(info.ID, nil)
				if e.Navigate != nil {
					e.Navigate(protocol.TabAgent, info.ID)
				}
			},
		)
	}
	if c.abort.Clicked(gtx) {
		var result protocol.GitOpResult
		e.Call(
			protocol.MGitAbort,
			protocol.WorktreeRef{Path: page.worktree},
			&result,
			func(error) { c.loadedPath = "" },
		)
	}
	if c.continueOp.Clicked(gtx) {
		var result protocol.GitOpResult
		e.Call(
			protocol.MGitContinue,
			protocol.WorktreeRef{Path: page.worktree},
			&result,
			func(error) { c.loadedPath = "" },
		)
	}
	if c.resolveAgent.Clicked(gtx) {
		c.resolveWithAgent(e, page)
	}
}

func (c *changesState) layoutStatus(
	gtx layout.Context,
	e *env.Env,
) layout.Dimensions {
	status := c.status.Status
	upstream := status.Upstream
	if upstream == "" {
		upstream = i18n.T("no upstream")
	}
	parts := []string{
		c.status.Branch,
		i18n.Tf("→ %s", upstream),
		i18n.Tf("+%d -%d", status.Ahead, status.Behind),
	}
	if status.Operation != "" {
		parts = append(parts, i18n.Tf("%s in progress", status.Operation))
	}
	children := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := kit.Label{
				Style:    e.Theme.T.MonoSm,
				Color:    e.Theme.R.Fg2,
				MaxLines: 1,
			}
			return label.Layout(gtx, e.Theme.Theme, strings.Join(parts, " · "))
		}),
	}
	if status.Operation != "" {
		c.abort.Kind, c.abort.Label, c.abort.Small = kit.PlaqueDangerLine, i18n.T("Abort"), true
		c.continueOp.Kind, c.continueOp.Label, c.continueOp.Small = kit.PlaqueQuiet, i18n.T("Continue"), true
		c.resolveAgent.Kind, c.resolveAgent.Label, c.resolveAgent.Small =
			kit.PlaqueQuiet, i18n.T("Resolve with agent"), true
		children = append(
			children,
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.abort.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.continueOp.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.resolveAgent.Layout(gtx, e.Theme.Theme)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (c *changesState) layoutReview(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	files := func(gtx layout.Context) layout.Dimensions {
		return c.layoutFileList(gtx, e, page)
	}
	diff := func(gtx layout.Context) layout.Dimensions {
		options := diffview.Options{
			Comments:    e.Snapshot().Reviews[page.worktree],
			Commentable: true,
			FileActions: false,
			Staged:      c.selectedDiffMode() == protocol.DiffStaged,
			FoldOver:    240,
			Wrap:        true,
		}
		dimensions := c.view.Layout(gtx, e.Theme.Theme, c.selectedDiff(), options)
		c.handleDiffActions(e, page, c.view.Actions())
		return dimensions
	}
	if gtx.Constraints.Max.X >= gtx.Dp(1100) {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				width := min(gtx.Dp(320), gtx.Constraints.Max.X)
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
				return files(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: 13}.Layout),
			layout.Flexed(1, diff),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(210))
			return files(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Flexed(1, diff),
	)
}

func (c *changesState) layoutFileList(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	items := c.changeListItems()
	if len(items) == 0 {
		empty := kit.Empty{
			Title: i18n.T("No changes"),
			Body:  i18n.T("This worktree has no files to review."),
		}
		return empty.Layout(gtx, e.Theme.Theme)
	}
	return c.fileList.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		item := items[index]
		if item.file == nil {
			return c.layoutGroupHeader(gtx, e, page, item.group)
		}
		return c.layoutFileRow(gtx, e, page, *item.file)
	})
}

func (c *changesState) changeListItems() []changeListItem {
	if c.modeTabs.Selected == 1 {
		items := []changeListItem{{group: -1}}
		for index := range c.diff.Files {
			file := c.diff.Files[index]
			change := protocol.FileChange{
				Path:    file.Path,
				OldPath: file.OldPath,
				Status:  file.Status,
				Added:   file.Added,
				Removed: file.Removed,
				Binary:  file.Binary,
			}
			items = append(items, changeListItem{group: -1, file: &change})
		}
		return items
	}
	groups := c.groupedChanges()
	items := make([]changeListItem, 0, len(c.status.Files)+4)
	for group := changeStaged; group <= changeConflicted; group++ {
		if len(groups[group]) == 0 {
			continue
		}
		items = append(items, changeListItem{group: group})
		for index := range groups[group] {
			file := groups[group][index]
			items = append(items, changeListItem{group: group, file: &file})
		}
	}
	return items
}

func (c *changesState) groupedChanges() [4][]protocol.FileChange {
	var groups [4][]protocol.FileChange
	for _, file := range c.status.Files {
		group := changeUnstaged
		switch {
		case file.Status == protocol.ChangeConflict:
			group = changeConflicted
		case file.Staged:
			group = changeStaged
		case file.Status == protocol.ChangeUntracked:
			group = changeUntracked
		}
		groups[group] = append(groups[group], file)
	}
	return groups
}

func (c *changesState) layoutGroupHeader(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	group int,
) layout.Dimensions {
	if group < 0 {
		return kit.GroupHeader(e.Theme.Theme, i18n.T("Branch vs base"), len(c.diff.Files))(gtx)
	}
	groups := c.groupedChanges()
	files := groups[group]
	row := &c.groupRows[group]
	paths := fileChangePaths(files)
	if row.stage.Clicked(gtx) {
		c.gitPaths(e, page, protocol.MGitStage, paths, false)
	}
	if row.unstage.Clicked(gtx) {
		c.gitPaths(e, page, protocol.MGitUnstage, paths, false)
	}
	if row.discard.Clicked(gtx) {
		c.discardFiles = paths
		c.discardConfirm.Danger, c.discardConfirm.Dialog.Open = true, true
	}
	row.stage.Kind, row.stage.Label, row.stage.Small = kit.PlaqueQuiet, i18n.T("Stage all"), true
	row.unstage.Kind, row.unstage.Label, row.unstage.Small = kit.PlaqueQuiet, i18n.T("Unstage all"), true
	row.discard.Kind, row.discard.Label, row.discard.Small =
		kit.PlaqueDangerLine, i18n.T("Discard all"), true
	title := []string{
		i18n.T("Staged"),
		i18n.T("Unstaged"),
		i18n.T("Untracked"),
		i18n.T("Conflicted"),
	}[group]
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, kit.GroupHeader(e.Theme.Theme, title, len(files))),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if group == changeStaged {
				return row.unstage.Layout(gtx, e.Theme.Theme)
			}
			return row.stage.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if group == changeStaged {
				return layout.Dimensions{}
			}
			return row.discard.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func fileChangePaths(files []protocol.FileChange) []string {
	paths := make([]string, len(files))
	for index, file := range files {
		paths[index] = file.Path
	}
	return paths
}

func (c *changesState) layoutFileRow(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	file protocol.FileChange,
) layout.Dimensions {
	row := c.fileRows[file.Path]
	if row == nil {
		row = new(changeFileRow)
		c.fileRows[file.Path] = row
	}
	if row.selectFile.Clicked(gtx) {
		c.selectedPath = file.Path
		c.loadedMode = ""
		c.view.ScrollToFile(file.Path)
	}
	if row.stage.Clicked(gtx) {
		c.gitPaths(e, page, protocol.MGitStage, []string{file.Path}, false)
	}
	if row.unstage.Clicked(gtx) {
		c.gitPaths(e, page, protocol.MGitUnstage, []string{file.Path}, false)
	}
	if row.discard.Clicked(gtx) {
		c.discardFiles = []string{file.Path}
		c.discardConfirm.Danger, c.discardConfirm.Dialog.Open = true, true
	}
	row.stage.Kind, row.stage.Label, row.stage.Small = kit.PlaqueQuiet, i18n.T("Stage"), true
	row.unstage.Kind, row.unstage.Label, row.unstage.Small = kit.PlaqueQuiet, i18n.T("Unstage"), true
	row.discard.Kind, row.discard.Label, row.discard.Small =
		kit.PlaqueDangerLine, i18n.T("Discard"), true
	return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return row.selectFile.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			fill := e.Theme.R.Stone1
			if c.selectedPath == file.Path {
				fill = e.Theme.R.Stone3
			} else if row.selectFile.Hovered() {
				fill = palette.Mix(fill, e.Theme.R.Fg, .06)
			}
			return kit.Slab(gtx, fill, e.Theme.Cut.Small, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 5, Right: 7, Bottom: 5, Left: 7}.Layout(
					gtx,
					func(gtx layout.Context) layout.Dimensions {
						return c.layoutFileRowContent(gtx, e, row, file)
					},
				)
			})
		})
	})
}

func (c *changesState) layoutFileRowContent(
	gtx layout.Context,
	e *env.Env,
	row *changeFileRow,
	file protocol.FileChange,
) layout.Dimensions {
	counts := i18n.Tf("+%d -%d", file.Added, file.Removed)
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg3, changeLetter(file.Status))),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := kit.Label{Style: e.Theme.T.MonoSm, Color: e.Theme.R.Fg, MaxLines: 1}
			return label.Layout(gtx, e.Theme.Theme, file.Path)
		}),
		layout.Rigid(layout.Spacer{Width: 6}.Layout),
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg3, counts)),
	}
	if row.selectFile.Hovered() && c.modeTabs.Selected == 0 {
		children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		if file.Staged {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.unstage.Layout(gtx, e.Theme.Theme)
			}))
		} else {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.stage.Layout(gtx, e.Theme.Theme)
			}))
		}
		children = append(
			children,
			layout.Rigid(layout.Spacer{Width: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.discard.Layout(gtx, e.Theme.Theme)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func changeLetter(status string) string {
	switch status {
	case protocol.ChangeAdded:
		return "A"
	case protocol.ChangeDeleted:
		return "D"
	case protocol.ChangeRenamed:
		return "R"
	case protocol.ChangeCopied:
		return "C"
	case protocol.ChangeType:
		return "T"
	case protocol.ChangeConflict:
		return "!"
	case protocol.ChangeUntracked:
		return "?"
	default:
		return "M"
	}
}

func (c *changesState) selectedDiff() protocol.DiffResult {
	result := protocol.DiffResult{Truncated: c.diff.Truncated}
	for _, file := range c.diff.Files {
		if c.selectedPath == "" || file.Path == c.selectedPath {
			result.Files = append(result.Files, file)
		}
	}
	return result
}

func (c *changesState) layoutGitActions(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	c.commitMessage.Label = i18n.T("Commit message")
	c.commitMessage.Placeholder = i18n.T("Describe the staged change")
	c.amend.Label = i18n.T("Amend")
	c.commit.Kind, c.commit.Label = kit.PlaqueGold, i18n.T("Commit")
	c.push.Kind, c.push.Label, c.push.Small = kit.PlaqueQuiet, i18n.T("Push"), true
	c.fetch.Kind, c.fetch.Label, c.fetch.Small = kit.PlaqueQuiet, i18n.T("Fetch"), true
	c.pull.Kind, c.pull.Label, c.pull.Small = kit.PlaqueQuiet, i18n.T("Pull"), true
	c.sendReview.Kind, c.sendReview.Label, c.sendReview.Small =
		kit.PlaqueQuiet, i18n.T("Send review to agent"), true
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return c.commitMessage.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.amend.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.commit.Layout(gtx, e.Theme.Theme)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.fetch.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.pull.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.push.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return c.sendReview.Layout(gtx, e.Theme.Theme)
				}),
			)
		}),
	)
}

func (c *changesState) layoutConfirms(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	if c.discardConfirm.Dialog.Open {
		confirmed := c.discardConfirm.Layout(
			gtx,
			e.Theme.Theme,
			i18n.T("Discard changes"),
			i18n.Tf("Discard changes in %d files? This cannot be undone.", len(c.discardFiles)),
			i18n.T("Discard"),
			"",
		)
		if confirmed {
			c.gitPaths(e, page, protocol.MGitDiscard, c.discardFiles, false)
			c.discardFiles = nil
		}
	}
	if c.pushConfirm.Dialog.Open {
		confirmed := c.pushConfirm.Layout(
			gtx,
			e.Theme.Theme,
			i18n.T("Force push with lease"),
			i18n.T(
				"The branch is behind its upstream. Force push only if you intend to replace those remote commits.",
			),
			i18n.T("Force push with lease"),
			c.status.Branch,
		)
		if confirmed {
			c.pushNow(e, page, true)
			c.forcePush = false
		}
	}
	return layout.Dimensions{}
}

func (c *changesState) handleDiffActions(
	e *env.Env,
	page *pageState,
	actions []diffview.Action,
) {
	refreshReviews := func(error) { e.LoadReviews(page.worktree, nil) }
	for _, action := range actions {
		switch action.Kind {
		case diffview.ActComment:
			var result protocol.ReviewComment
			params := protocol.ReviewAddParams{
				Worktree: page.worktree,
				Path:     action.Path,
				Line:     action.Line,
				Side:     action.Side,
				Text:     action.Text,
			}
			e.Call(protocol.MReviewAdd, params, &result, refreshReviews)
		case diffview.ActResolve:
			comment := findComment(e.Snapshot().Reviews[page.worktree], action.ID)
			var result protocol.ReviewComment
			params := protocol.ReviewRef{
				Worktree: page.worktree,
				ID:       action.ID,
				Resolved: !comment.Resolved,
			}
			e.Call(protocol.MReviewResolve, params, &result, refreshReviews)
		case diffview.ActDelete:
			params := protocol.ReviewRef{Worktree: page.worktree, ID: action.ID}
			e.Call(protocol.MReviewDelete, params, new(protocol.Empty), refreshReviews)
		case diffview.ActStage:
			c.gitPaths(e, page, protocol.MGitStage, []string{action.Path}, false)
		case diffview.ActUnstage:
			c.gitPaths(e, page, protocol.MGitUnstage, []string{action.Path}, false)
		case diffview.ActDiscard:
			c.discardFiles = []string{action.Path}
			c.discardConfirm.Danger, c.discardConfirm.Dialog.Open = true, true
		case diffview.ActOpen:
			env.Spawn("xdg-open", filepath.Join(page.worktree, action.Path))
		}
	}
}

func (c *changesState) gitPaths(
	e *env.Env,
	page *pageState,
	method string,
	files []string,
	all bool,
) {
	var result protocol.GitStatusResult
	params := protocol.GitPathsParams{Path: page.worktree, Files: files, All: all}
	e.Call(method, params, &result, func(err error) {
		if err != nil {
			c.error = err.Error()
			return
		}
		c.status = result
		c.loadedPath = ""
	})
}

func (c *changesState) pushNow(
	e *env.Env,
	page *pageState,
	force bool,
) {
	var result protocol.GitOpResult
	params := protocol.GitPushParams{
		Path:           page.worktree,
		SetUpstream:    c.status.Status.Upstream == "",
		ForceWithLease: force,
	}
	e.Call(protocol.MGitPush, params, &result, func(err error) {
		if err != nil {
			c.error = err.Error()
		}
		c.loadedPath = ""
	})
}

func (c *changesState) resolveWithAgent(e *env.Env, page *pageState) {
	conflicts := make([]string, 0)
	for _, file := range c.status.Files {
		if file.Status == protocol.ChangeConflict {
			conflicts = append(conflicts, file.Path)
		}
	}
	prompt := i18n.Tf(
		"Resolve the %s conflicts in this worktree. Conflicted files:\n%s",
		c.status.Status.Operation,
		strings.Join(conflicts, "\n"),
	)
	var session string
	for _, info := range e.Snapshot().Sessions {
		if info.Worktree == page.worktree && info.Kind == protocol.KindAgent {
			session = info.ID
			break
		}
	}
	if session == "" {
		var info protocol.SessionInfo
		params := protocol.SessionNewParams{
			Kind:     protocol.KindAgent,
			Origin:   protocol.OriginApp,
			Project:  page.project,
			Worktree: page.worktree,
		}
		e.Call(protocol.MSessionNew, params, &info, func(err error) {
			if err == nil {
				c.promptSession(e, info.ID, prompt)
			}
		})
		return
	}
	c.promptSession(e, session, prompt)
}

func (c *changesState) promptSession(e *env.Env, session, text string) {
	var result protocol.PromptResult
	params := protocol.PromptParams{ID: session, Text: text}
	e.Call(protocol.MSessionPrompt, params, &result, func(err error) {
		if err != nil {
			return
		}
		e.Attach(session, nil)
		if e.Navigate != nil {
			e.Navigate(protocol.TabAgent, session)
		}
	})
}

func findComment(comments []protocol.ReviewComment, id string) protocol.ReviewComment {
	for _, comment := range comments {
		if comment.ID == id {
			return comment
		}
	}
	return protocol.ReviewComment{}
}
