package agent

import (
	"fmt"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/diffview"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

const (
	scopeBranch = iota
	scopeUncommitted
	scopeTurn
)

// it only reads, staging and committing belong to the project's Changes view
type changesPanel struct {
	header look.PaneHeader
	tabs   kit.Tabs
	diff   diffview.View
	review kit.Plaque

	turn    int
	loaded  string
	loading bool
	result  protocol.DiffResult
	failure string
}

func (panel *changesPanel) showTurn(turn int) {
	panel.turn = turn
	panel.tabs.Selected = scopeTurn
}

const (
	sideChanges = iota
	sideScreen
)

func (p *Page) HasPanel(e *env.Env) bool {
	snapshot := e.Snapshot()
	info, ok := snapshot.Sessions[p.selected]
	if !ok {
		return false
	}
	if _, screen := snapshot.Screens[info.ID]; screen && !p.screen.large {
		return true
	}
	_, tracked := worktreeOf(snapshot, info)
	return tracked
}

func (p *Page) Panel(gtx layout.Context, e *env.Env) layout.Dimensions {
	snapshot := e.Snapshot()
	info := snapshot.Sessions[p.selected]
	worktree, tracked := worktreeOf(snapshot, info)
	if screen, ok := snapshot.Screens[info.ID]; ok && !p.screen.large && (p.side == sideScreen || !tracked) {
		return p.screen.layout(gtx, e, screen)
	}
	return p.panel.layout(gtx, e, info, worktree)
}

// showSide opens the side panel on one view, or closes it when that view is
// already open
func (p *Page) showSide(e *env.Env, side int) {
	if e.PanelOpen && p.side == side {
		e.TogglePanel()
		return
	}
	p.side = side
	if !e.PanelOpen {
		e.TogglePanel()
	}
}

func (panel *changesPanel) layout(gtx layout.Context, e *env.Env, info protocol.SessionInfo, worktree protocol.WorktreeInfo) layout.Dimensions {
	th := e.Theme
	panel.tabs.Labels = []string{i18n.T("Branch"), i18n.T("Uncommitted")}
	if panel.turn > 0 {
		panel.tabs.Labels = append(panel.tabs.Labels, i18n.Tf("Turn %d", panel.turn))
	}
	panel.tabs.Selected = min(panel.tabs.Selected, len(panel.tabs.Labels)-1)
	panel.load(e, info, worktree)
	dirty := worktree.Status != nil && worktree.Status.Staged+worktree.Status.Modified+worktree.Status.Untracked > 0
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return panel.layoutHeader(gtx, e, info, worktree) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return panel.tabs.Layout(gtx, th.Theme)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S2, Left: th.Sp.S2, Right: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return panel.layoutBody(gtx, e, worktree)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !dirty {
				return layout.Dimensions{}
			}
			panel.review.Kind, panel.review.Small, panel.review.Label = kit.PlaqueTonal, true, i18n.T("Review and commit")
			if panel.review.Clicked(gtx) {
				e.NavigateWork(info.Project, env.ViewChanges, worktree.Path)
			}
			return layout.Inset{Top: th.Sp.S2, Bottom: th.Sp.S3, Left: th.Sp.S4, Right: th.Sp.S3}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions { return panel.review.Layout(gtx, th.Theme) })
		}),
	)
}

func (panel *changesPanel) layoutHeader(gtx layout.Context, e *env.Env, info protocol.SessionInfo, worktree protocol.WorktreeInfo) layout.Dimensions {
	content := look.PaneHeaderContent{
		Title: i18n.T("Changes"), Scrolled: panel.diff.Scrolled(),
		Actions: []look.HeaderAction{
			{Icon: look.Icon.OpenInNew, Description: i18n.T("Review in Projects")},
			{Icon: look.Icon.Close, Description: i18n.T("Close") + "  Ctrl \\"},
		},
	}
	if status := worktree.Status; status != nil && worktree.Base != "" {
		content.Facts = []look.HeaderFact{{Text: i18n.Tf("%d ahead of %s", status.AheadBase, worktree.Base)}}
	}
	dimensions := panel.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := panel.header.ActionClicked(); ok {
		if index == 0 {
			e.NavigateWork(info.Project, env.ViewChanges, worktree.Path)
		} else {
			e.TogglePanel()
		}
	}
	return dimensions
}

func (panel *changesPanel) layoutBody(gtx layout.Context, e *env.Env, worktree protocol.WorktreeInfo) layout.Dimensions {
	th := e.Theme
	switch {
	case panel.failure != "":
		return kit.Empty{Icon: look.Icon.Warning, Title: i18n.T("The changes could not be read"), Body: panel.failure}.Layout(gtx, th.Theme)
	case panel.loading && len(panel.result.Files) == 0:
		return look.SkeletonRows(gtx, th, 6, false)
	case len(panel.result.Files) == 0:
		return kit.Empty{Icon: look.Icon.Diff, Title: i18n.T("Nothing changed"), Body: panel.emptyBody(worktree)}.Layout(gtx, th.Theme)
	}
	// fills the panel so its foot stays at the foot
	gtx.Constraints.Min = gtx.Constraints.Max
	panel.diff.Layout(gtx, th, panel.result, diffview.Options{Overlays: e.Overlays, Collapsed: true, FoldOver: 200})
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (panel *changesPanel) emptyBody(worktree protocol.WorktreeInfo) string {
	switch panel.tabs.Selected {
	case scopeUncommitted:
		return i18n.T("Every change in this worktree is committed.")
	case scopeTurn:
		return i18n.T("This turn changed no files.")
	default:
		return i18n.Tf("This branch is the same as %s.", worktree.Base)
	}
}

// runs again when the session, the scope or the worktree's status changes, so the panel follows the agent
func (panel *changesPanel) load(e *env.Env, info protocol.SessionInfo, worktree protocol.WorktreeInfo) {
	scope := panel.tabs.Selected
	key := fmt.Sprint(info.ID, scope, panel.turn, info.Turns)
	if status := worktree.Status; status != nil {
		key += fmt.Sprint(status.Added, status.Removed, status.Staged, status.Modified, status.Untracked, status.Checked.UnixNano())
	}
	if panel.loading || key == panel.loaded {
		return
	}
	panel.loading, panel.loaded, panel.failure = true, key, ""
	base := protocol.DiffParams{Project: info.Project, Worktree: worktree.Path, Context: 3}
	finish := func(result protocol.DiffResult, err error) {
		panel.loading = false
		if err != nil {
			panel.failure, panel.result = err.Error(), protocol.DiffResult{}
			return
		}
		panel.result = result
	}
	switch scope {
	case scopeTurn:
		base.Mode, base.Session, base.Turn = protocol.DiffCheckpoint, info.ID, panel.turn
		panel.fetch(e, base, finish)
	case scopeUncommitted:
		// staged and unstaged are two diffs, the panel reads them as one
		staged := base
		staged.Mode, base.Mode = protocol.DiffStaged, protocol.DiffWorking
		panel.fetch(e, staged, func(first protocol.DiffResult, err error) {
			if err != nil {
				finish(first, err)
				return
			}
			panel.fetch(e, base, func(second protocol.DiffResult, err error) {
				first.Files = append(first.Files, second.Files...)
				first.Truncated = first.Truncated || second.Truncated
				finish(first, err)
			})
		})
	default:
		base.Mode = protocol.DiffBase
		panel.fetch(e, base, finish)
	}
}

func (panel *changesPanel) fetch(e *env.Env, params protocol.DiffParams, done func(protocol.DiffResult, error)) {
	var result protocol.DiffResult
	e.Call(protocol.MGitDiff, params, &result, func(err error) { done(result, err) })
}
