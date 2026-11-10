package projects

import (
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

func (o *overviewState) layoutDialogs(gtx layout.Context, e *env.Env, page *pageState) {
	o.create.layout(gtx, e, page)
	o.move.layout(gtx, e, page)
	o.remove.layout(gtx, e, page)
	o.prune.layout(gtx, e, page)
	o.cleanup.layout(gtx, e, page)
}

func (o *overviewState) openNew(e *env.Env, page *pageState, info protocol.ProjectInfo) {
	o.create.open(e, page, info)
}

type createWorktree struct {
	project protocol.ProjectInfo
	dialog  kit.Dialog
	scroll  look.Scroller

	name, branch         kit.TextField
	mode, from, parent   look.Select
	modeIndex, fromIndex int
	parentIndex          int
	lock, setup          kit.Toggle
	refs                 []protocol.GitRef
	create, cancel       kit.Plaque
}

var worktreeModes = [...]string{protocol.WorktreeNewBranch, protocol.WorktreeExistingBranch, protocol.WorktreeDetached}

func (d *createWorktree) open(e *env.Env, page *pageState, info protocol.ProjectInfo) {
	d.project, d.dialog.Open = info, true
	d.name.SetText("")
	d.branch.SetText("")
	d.modeIndex, d.fromIndex, d.parentIndex = 0, 0, 0
	d.setup.Value = info.Settings.Setup != ""
	var log protocol.GitLogResult
	e.Call(protocol.MGitLog, protocol.GitLogParams{Project: info.ID, Limit: 1}, &log, func(err error) {
		if err == nil {
			d.refs = log.Refs
		}
	})
}

func (d *createWorktree) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if d.project.ID == "" {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.create.Kind, d.create.Label = kit.PlaqueGold, i18n.T("Create worktree")
	name := strings.TrimSpace(d.name.Text())
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.create.Clicked(gtx) && name != "" {
		d.submit(e, page, name)
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("New worktree"),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(480))
			gtx.Constraints.Min.Y = 0
			rows := d.rows(e)
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if name == "" {
				gtx = gtx.Disabled()
			}
			return d.create.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.project = protocol.ProjectInfo{}
	}
}

func (d *createWorktree) rows(e *env.Env) []layout.Widget {
	th := e.Theme
	snapshot := e.Snapshot()
	refLabels := []string{i18n.Tf("The project's base, %s", d.project.Settings.BaseRef)}
	for _, ref := range d.refs {
		refLabels = append(refLabels, ref.Name)
	}
	parents := []string{i18n.T("None")}
	for _, worktree := range snapshot.Worktrees[d.project.ID] {
		parents = append(parents, worktree.Name)
	}
	field := func(input *kit.TextField, label, placeholder string, mono bool) layout.Widget {
		input.Label, input.Placeholder, input.Mono = label, placeholder, mono
		return func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) }
	}
	choice := func(selection *look.Select, label string, options []string, selected *int) layout.Widget {
		selection.Label = label
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := selection.Layout(gtx, th, e.Overlays, options, *selected)
			if index, changed := selection.Changed(); changed {
				*selected = index
			}
			return dimensions
		}
	}
	modes := []string{i18n.T("A new branch"), i18n.T("A branch that exists"), i18n.T("No branch, detached")}
	rows := []layout.Widget{
		field(&d.name, i18n.T("Name"), "settings-panel", false),
		choice(&d.mode, i18n.T("Checks out"), modes, &d.modeIndex),
	}
	defaultBranch := d.project.Settings.BranchPrefix + strings.TrimSpace(d.name.Text())
	switch d.modeIndex {
	case 0:
		rows = append(rows,
			field(&d.branch, i18n.T("Branch"), defaultBranch, true),
			choice(&d.from, i18n.T("Starts from"), refLabels, &d.fromIndex),
		)
	case 1:
		rows = append(rows, field(&d.branch, i18n.T("Branch"), "", true))
	default:
		rows = append(rows, choice(&d.from, i18n.T("At"), refLabels, &d.fromIndex))
	}
	d.lock.Label = i18n.T("Lock it, so cleanup never removes it")
	d.setup.Label = i18n.T("Run the project's setup in it")
	rows = append(rows,
		choice(&d.parent, i18n.T("Stacked on"), parents, &d.parentIndex),
		func(gtx layout.Context) layout.Dimensions { return d.setup.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.lock.Layout(gtx, th.Theme) },
	)
	return rows
}

func (d *createWorktree) submit(e *env.Env, page *pageState, name string) {
	branch := strings.TrimSpace(d.branch.Text())
	if d.modeIndex == 0 && branch == "" {
		branch = d.project.Settings.BranchPrefix + name
	}
	base := d.project.Settings.BaseRef
	if d.fromIndex > 0 && d.fromIndex-1 < len(d.refs) {
		base = d.refs[d.fromIndex-1].Name
	}
	parent := ""
	if worktrees := e.Snapshot().Worktrees[d.project.ID]; d.parentIndex > 0 && d.parentIndex-1 < len(worktrees) {
		parent = worktrees[d.parentIndex-1].Path
	}
	params := protocol.WorktreeCreateParams{
		Project: d.project.ID, Name: name, Mode: worktreeModes[d.modeIndex], Base: base, Branch: branch,
		Parent: parent, Lock: d.lock.Value, Setup: d.setup.Value,
	}
	project := d.project.ID
	var created protocol.WorktreeInfo
	e.Call(protocol.MWorktreesCreate, params, &created, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		e.LoadWorktrees(project, nil)
		e.NavigateWork(project, TabOverview, created.Path)
	})
}

type moveWorktree struct {
	path       string
	dialog     kit.Dialog
	field      env.PathField
	do, cancel kit.Plaque
}

func (d *moveWorktree) open(worktree protocol.WorktreeInfo) {
	d.path, d.dialog.Open = worktree.Path, true
	d.field.SetText(worktree.Path)
}

func (d *moveWorktree) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if d.path == "" {
		return
	}
	th := e.Theme
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.do.Kind, d.do.Label = kit.PlaqueGold, i18n.T("Move")
	d.field.Title, d.field.Label = i18n.T("Move this worktree"), i18n.T("New folder")
	to := d.field.Path()
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.do.Clicked(gtx) && to != "" && to != d.path {
		params := protocol.WorktreeMoveParams{Path: d.path, To: to}
		e.Call(protocol.MWorktreesMove, params, new(protocol.WorktreeInfo), func(err error) {
			if err != nil {
				page.fail(err)
				return
			}
			e.LoadWorktrees(page.project, nil)
			e.NavigateWork(page.project, TabOverview, to)
		})
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Move this worktree"),
		func(gtx layout.Context) layout.Dimensions { return d.field.Layout(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return d.do.Layout(gtx, th.Theme) },
	)
	if d.dialog.Closed() {
		d.path = ""
	}
}

type removeWorktree struct {
	preview      protocol.RemovePreview
	shown        bool
	dialog       kit.Dialog
	branch       kit.Toggle
	force        kit.Toggle
	proof        kit.TextField
	do, cancel   kit.Plaque
	worktreeName string
}

func (d *removeWorktree) open(e *env.Env, page *pageState, worktree protocol.WorktreeInfo) {
	d.worktreeName = worktree.Name
	e.Call(protocol.MWorktreesPreviewRemove, protocol.WorktreeRef{Path: worktree.Path}, &d.preview, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		d.shown, d.dialog.Open = true, true
		d.branch.Value, d.force.Value = false, false
		d.proof.SetText("")
	})
}

func (d *removeWorktree) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if !d.shown {
		return
	}
	th := e.Theme
	preview := d.preview
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.do.Kind, d.do.Label = kit.PlaqueDanger, i18n.T("Remove worktree")
	ready := removeReady(preview, d.worktreeName, d.proof.Text())
	if d.cancel.Clicked(gtx) {
		d.dialog.Dismiss(gtx)
	}
	if d.do.Clicked(gtx) && ready {
		params := removeParams(preview, d.branch.Value, d.force.Value)
		e.Call(protocol.MWorktreesRemove, params, new(protocol.Empty), func(err error) {
			if err != nil {
				page.fail(err)
				return
			}
			e.LoadWorktrees(page.project, nil)
			e.NavigateWork(page.project, TabOverview, "")
		})
		d.dialog.Dismiss(gtx)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.Tf("Remove %s?", d.worktreeName),
		func(gtx layout.Context) layout.Dimensions { return d.body(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.do.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.shown = false
	}
}

func (d *removeWorktree) body(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	preview := d.preview
	gap := layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout)
	children := []layout.FlexChild{layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, removeWords(preview)))}
	if !preview.CanRemove {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
	if preview.Branch != "" {
		d.branch.Label = i18n.Tf("Also delete the branch %s", preview.Branch)
		d.force.Label = i18n.T("Even though it is not merged")
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.branch.Layout(gtx, th.Theme) }))
		if preview.Unmerged && d.branch.Value {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.force.Layout(gtx, th.Theme) }),
			)
		}
	}
	if preview.NeedsForce {
		d.proof.Label = i18n.Tf("Type %s to remove it with what it holds", d.worktreeName)
		d.proof.Placeholder = d.worktreeName
		children = append(children, gap, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.proof.Layout(gtx, th.Theme) }))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func removeWords(preview protocol.RemovePreview) string {
	lines := make([]string, 0, 6)
	list := func(format string, values []string) {
		if len(values) > 0 {
			lines = append(lines, i18n.Tf(format, strings.Join(values, ", ")))
		}
	}
	list("Changed and not committed: %s", preview.Dirty)
	list("Not tracked by Git: %s", preview.Untracked)
	list("Committed and not pushed: %s", preview.Unpushed)
	list("Agent sessions running in it: %s", preview.Running)
	if preview.Locked {
		lines = append(lines, i18n.T("It is locked."))
	}
	if preview.Unmerged {
		lines = append(lines, i18n.T("Its branch is not merged."))
	}
	if preview.BlockReason != "" {
		lines = append(lines, preview.BlockReason)
	}
	if len(lines) == 0 {
		return i18n.T("Its folder is removed. Nothing in it is uncommitted or unpushed.")
	}
	return strings.Join(lines, "\n")
}

type pruneWorktrees struct {
	shown   bool
	result  protocol.PruneResult
	confirm kit.Confirm
}

func (d *pruneWorktrees) open(e *env.Env, page *pageState) {
	e.Call(protocol.MWorktreesPrune, protocol.PruneParams{Project: page.project, DryRun: true}, &d.result, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		if len(d.result.Pruned) == 0 {
			e.Notify(i18n.T("Nothing to prune: every worktree's folder is there."))
			return
		}
		d.shown, d.confirm.Dialog.Open = true, true
	})
}

func (d *pruneWorktrees) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if !d.shown {
		return
	}
	body := i18n.T("These worktrees have no folder any more. Their records are dropped:") + "\n" + strings.Join(d.result.Pruned, "\n")
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Prune missing worktrees?"), body, i18n.T("Prune"), "") {
		e.Call(protocol.MWorktreesPrune, protocol.PruneParams{Project: page.project}, new(protocol.PruneResult), func(err error) {
			page.fail(err)
			e.LoadWorktrees(page.project, nil)
		})
	}
	if d.confirm.Closed() {
		d.shown = false
	}
}

type cleanupWorktrees struct {
	shown   bool
	result  protocol.CleanupResult
	confirm kit.Confirm
}

func (d *cleanupWorktrees) open(e *env.Env, page *pageState) {
	e.Call(protocol.MWorktreesCleanup, protocol.CleanupParams{Project: page.project, DryRun: true}, &d.result, func(err error) {
		if err != nil {
			page.fail(err)
			return
		}
		removes := 0
		for _, item := range d.result.Items {
			if item.Removed {
				removes++
			}
		}
		if removes == 0 {
			e.Notify(i18n.T("Nothing to clean up: no finished worktree matches the project's policy."))
			return
		}
		d.shown, d.confirm.Dialog.Open, d.confirm.Danger = true, true, true
	})
}

func (d *cleanupWorktrees) layout(gtx layout.Context, e *env.Env, page *pageState) {
	if !d.shown {
		return
	}
	lines := make([]string, 0, len(d.result.Items))
	for _, item := range d.result.Items {
		if item.Removed {
			lines = append(lines, i18n.Tf("Remove %s: %s", item.Path, item.Reason))
		}
	}
	body := strings.Join(lines, "\n")
	if d.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Clean up finished worktrees?"), body, i18n.T("Clean up"), "") {
		e.Call(protocol.MWorktreesCleanup, protocol.CleanupParams{Project: page.project}, new(protocol.CleanupResult), func(err error) {
			page.fail(err)
			e.LoadWorktrees(page.project, nil)
		})
	}
	if d.confirm.Closed() {
		d.shown = false
	}
}

// losing work takes typing the worktree's name, not one more press
func removeReady(preview protocol.RemovePreview, name, typed string) bool {
	return preview.CanRemove && (!preview.NeedsForce || strings.TrimSpace(typed) == name)
}

// forces the removal only when the preview said it must, and the branch only when the branch goes too
func removeParams(preview protocol.RemovePreview, deleteBranch, forceBranch bool) protocol.WorktreeRemoveParams {
	return protocol.WorktreeRemoveParams{
		Path: preview.Path, Force: preview.NeedsForce,
		DeleteBranch: deleteBranch, ForceBranch: deleteBranch && forceBranch,
	}
}
