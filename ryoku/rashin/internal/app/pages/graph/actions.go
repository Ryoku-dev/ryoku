package graph

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type operationKind uint8

const (
	opNone operationKind = iota
	opCreateWorktree
	opCreateBranch
	opCheckout
	opMerge
	opCherryPick
	opRevert
	opRebase
	opCompare
)

type pendingOperation struct {
	kind        operationKind
	hash        string
	dialog      kit.Dialog
	confirm     kit.Plaque
	cancel      kit.Plaque
	name        kit.TextField
	branch      kit.TextField
	mode        look.Select
	modeIndex   int
	target      look.Select
	targetIndex int
	running     bool
	error       string
}

func (operation *pendingOperation) reset() {
	*operation = pendingOperation{}
	operation.name.Editor.SingleLine = true
	operation.branch.Editor.SingleLine = true
}

func (v *viewState) openRowMenu(gtx layout.Context, e *env.Env, row *rowState, entry graphEntry) {
	if e.Overlays == nil {
		return
	}
	items := v.commitMenuItems(entry)
	if entry.kind == rowWIP {
		items = []kit.MenuItem{{Label: i18n.T("Open changes")}, {Label: i18n.T("Open worktree")}, {Label: i18n.T("New session")}}
	}
	content := func(gtx layout.Context) layout.Dimensions {
		dimensions := row.menu.Layout(gtx, e.Theme.Theme, items)
		if index, clicked := row.menu.Clicked(gtx); clicked {
			e.Overlays.Close(&row.menuTag)
			if entry.kind == rowWIP {
				kinds := []ActionKind{ActOpenChanges, ActOpenWorktree, ActNewSession}
				if index >= 0 && index < len(kinds) {
					v.actions = append(v.actions, Action{Kind: kinds[index], Project: v.project, Worktree: entry.wip.Worktree})
				}
			} else {
				v.handleCommitMenu(gtx, e, entry, index)
			}
			gtx.Execute(op.InvalidateCmd{})
		}
		return dimensions
	}
	e.Overlays.OpenAt(&row.menuTag, e.Overlays.Press(), content)
}

func (v *viewState) commitMenuItems(entry graphEntry) []kit.MenuItem {
	return []kit.MenuItem{
		{Label: i18n.T("Create worktree here")},
		{Label: i18n.T("Create branch here")},
		{Label: v.checkoutLabel(entry)},
		{Label: i18n.T("Merge into a worktree")},
		{Label: i18n.T("Cherry-pick into a worktree")},
		{Label: i18n.T("Revert in a worktree")},
		{Label: i18n.T("Rebase a worktree onto this")},
		{Label: i18n.T("Copy hash")},
		{Label: i18n.T("Compare with a worktree")},
	}
}

func (v *viewState) checkoutLabel(entry graphEntry) string {
	if _, ok := v.checkedOutAt(entry.commit.Hash); ok {
		return i18n.T("Open worktree")
	}
	return i18n.T("Check out in a worktree")
}

func (v *viewState) handleCommitMenu(gtx layout.Context, e *env.Env, entry graphEntry, index int) {
	if index == 7 {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(entry.commit.Hash))})
		return
	}
	if index == 2 {
		if path, ok := v.checkedOutAt(entry.commit.Hash); ok {
			v.actions = append(v.actions, Action{Kind: ActOpenWorktree, Project: v.project, Worktree: path})
			return
		}
	}
	kinds := []operationKind{opCreateWorktree, opCreateBranch, opCheckout, opMerge, opCherryPick, opRevert, opRebase, opNone, opCompare}
	if index < 0 || index >= len(kinds) || kinds[index] == opNone {
		return
	}
	v.operation.reset()
	v.operation.kind = kinds[index]
	v.operation.hash = entry.commit.Hash
	v.operation.dialog.Open = true
	v.operation.dialog.Width = 520
}

func (v *viewState) layoutOperationDialog(gtx layout.Context, e *env.Env) layout.Dimensions {
	operation := &v.operation
	if operation.kind == opNone {
		return layout.Dimensions{}
	}
	operation.cancel.Kind, operation.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	operation.confirm.Kind, operation.confirm.Small = kit.PlaqueGold, false
	operation.confirm.Busy = operation.running
	if operation.cancel.Clicked(gtx) && !operation.running {
		operation.dialog.Open = false
	}
	if operation.confirm.Clicked(gtx) && !operation.running {
		v.runOperation(e)
	}
	title, action := operationTitle(operation.kind)
	operation.confirm.Label = action
	dimensions := operation.dialog.Layout(gtx, e.Theme.Theme, title, func(gtx layout.Context) layout.Dimensions {
		return v.layoutOperationBody(gtx, e)
	}, func(gtx layout.Context) layout.Dimensions { return operation.cancel.Layout(gtx, e.Theme.Theme) }, func(gtx layout.Context) layout.Dimensions { return operation.confirm.Layout(gtx, e.Theme.Theme) })
	if operation.dialog.Closed() {
		operation.reset()
	}
	return dimensions
}

func operationTitle(kind operationKind) (string, string) {
	switch kind {
	case opCreateWorktree:
		return i18n.T("Create worktree here"), i18n.T("Create")
	case opCreateBranch:
		return i18n.T("Create branch here"), i18n.T("Create")
	case opCheckout:
		return i18n.T("Check out in a worktree"), i18n.T("Check out")
	case opMerge:
		return i18n.T("Merge into a worktree"), i18n.T("Merge")
	case opCherryPick:
		return i18n.T("Cherry-pick into a worktree"), i18n.T("Cherry-pick")
	case opRevert:
		return i18n.T("Revert in a worktree"), i18n.T("Revert")
	case opRebase:
		return i18n.T("Rebase a worktree onto this"), i18n.T("Rebase")
	case opCompare:
		return i18n.T("Compare with a worktree"), i18n.T("Compare")
	default:
		return i18n.T("Git action"), i18n.T("Continue")
	}
}

func (v *viewState) layoutOperationBody(gtx layout.Context, e *env.Env) layout.Dimensions {
	operation := &v.operation
	short := shortHash(operation.hash)
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Body, e.Theme.R.Fg, operationStatement(operation.kind, short))),
	}
	if operation.kind == opCreateWorktree {
		modes := []string{i18n.T("New branch"), i18n.T("Detached")}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dimensions := operation.mode.Layout(gtx, e.Theme, e.Overlays, modes, operation.modeIndex)
				if index, changed := operation.mode.Changed(); changed {
					operation.modeIndex = index
				}
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				operation.name.Label, operation.name.Placeholder = i18n.T("Worktree name"), i18n.T("review-change")
				return operation.name.Layout(gtx, e.Theme.Theme)
			}),
		)
		if operation.modeIndex == 0 {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					operation.branch.Label, operation.branch.Placeholder = i18n.T("Branch"), i18n.T("Derived from the worktree name")
					return operation.branch.Layout(gtx, e.Theme.Theme)
				}),
			)
		}
	} else if operation.kind == opCreateBranch {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				operation.branch.Label, operation.branch.Placeholder = i18n.T("Branch"), i18n.T("feature/name")
				return operation.branch.Layout(gtx, e.Theme.Theme)
			}),
		)
	} else {
		labels, _ := v.worktreeNames()
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(labels) == 0 {
					return kit.Text(e.Theme.Theme, e.Theme.T.Body, e.Theme.R.Jasper, i18n.T("No worktree is available"))(gtx)
				}
				if operation.targetIndex >= len(labels) {
					operation.targetIndex = 0
				}
				dimensions := operation.target.Layout(gtx, e.Theme, e.Overlays, labels, operation.targetIndex)
				if index, changed := operation.target.Changed(); changed {
					operation.targetIndex = index
				}
				return dimensions
			}),
		)
	}
	if operation.error != "" {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Body, e.Theme.R.Jasper, operation.error)),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func operationStatement(kind operationKind, hash string) string {
	switch kind {
	case opCreateWorktree:
		return i18n.Tf("Create a worktree at commit %s. Choose whether it owns a new branch or stays detached.", hash)
	case opCreateBranch:
		return i18n.Tf("Create a branch at commit %s. No worktree will move.", hash)
	case opCheckout:
		return i18n.Tf("Move the chosen clean worktree to commit %s. Dirty worktrees are refused.", hash)
	case opMerge:
		return i18n.Tf("Merge commit %s into the chosen worktree.", hash)
	case opCherryPick:
		return i18n.Tf("Cherry-pick commit %s into the chosen worktree.", hash)
	case opRevert:
		return i18n.Tf("Create a revert of commit %s in the chosen worktree.", hash)
	case opRebase:
		return i18n.Tf("Rebase the chosen worktree onto commit %s. Its branch history will be rewritten.", hash)
	case opCompare:
		return i18n.Tf("Compare commit %s with the chosen worktree head.", hash)
	default:
		return ""
	}
}

func (v *viewState) runOperation(e *env.Env) {
	operation := &v.operation
	operation.error = ""
	if operation.kind == opCreateWorktree {
		name := strings.TrimSpace(operation.name.Text())
		if name == "" {
			operation.error = i18n.T("Enter a worktree name")
			return
		}
		mode := protocol.WorktreeNewBranch
		if operation.modeIndex == 1 {
			mode = protocol.WorktreeDetached
		}
		params := protocol.WorktreeCreateParams{Project: v.project, Name: name, Mode: mode, Branch: strings.TrimSpace(operation.branch.Text()), Base: operation.hash}
		operation.running = true
		var result protocol.WorktreeInfo
		e.Call(protocol.MWorktreesCreate, params, &result, func(err error) {
			operation.running = false
			if err != nil {
				operation.error = err.Error()
				return
			}
			operation.dialog.Open = false
			v.refreshRequested = true
			v.actions = append(v.actions, Action{Kind: ActOpenWorktree, Project: v.project, Worktree: result.Path})
		})
		return
	}
	if operation.kind == opCreateBranch {
		branch := strings.TrimSpace(operation.branch.Text())
		if branch == "" {
			operation.error = i18n.T("Enter a branch name")
			return
		}
		operation.running = true
		var result protocol.GitOpResult
		e.Call(protocol.MGitBranchCreate, protocol.GitRefParams{Project: v.project, Ref: branch, At: operation.hash}, &result, v.operationDone(&result))
		return
	}
	_, worktrees := v.worktreeNames()
	if len(worktrees) == 0 || operation.targetIndex < 0 || operation.targetIndex >= len(worktrees) {
		operation.error = i18n.T("Choose a worktree")
		return
	}
	worktree := worktrees[operation.targetIndex]
	if operation.kind == opCheckout && worktreeDirty(worktree) {
		operation.error = i18n.T("This worktree has uncommitted changes. Open changes before checking out another commit.")
		return
	}
	if operation.kind == opCompare {
		operation.running = true
		var result protocol.DiffResult
		e.Call(protocol.MGitDiff, protocol.DiffParams{Project: v.project, Mode: protocol.DiffRange, From: operation.hash, To: worktree.Head, Context: 3}, &result, func(err error) {
			operation.running = false
			if err != nil {
				operation.error = err.Error()
				return
			}
			v.diff = result
			v.compareWith = worktree.Name
			if v.compareWith == "" {
				v.compareWith = worktree.Path
			}
			operation.dialog.Open = false
		})
		return
	}
	operation.running = true
	var result protocol.GitOpResult
	var method string
	var params any
	switch operation.kind {
	case opCheckout:
		method, params = protocol.MGitCheckout, protocol.GitRefParams{Path: worktree.Path, Ref: operation.hash}
	case opMerge:
		method, params = protocol.MGitMerge, protocol.GitApplyParams{Path: worktree.Path, Ref: operation.hash}
	case opCherryPick:
		method, params = protocol.MGitCherryPick, protocol.GitApplyParams{Path: worktree.Path, Commits: []string{operation.hash}}
	case opRevert:
		method, params = protocol.MGitRevert, protocol.GitApplyParams{Path: worktree.Path, Commits: []string{operation.hash}}
	case opRebase:
		method, params = protocol.MGitRebase, protocol.GitApplyParams{Path: worktree.Path, Ref: operation.hash}
	}
	path := worktree.Path
	e.Call(method, params, &result, func(err error) {
		operation.running = false
		if err != nil {
			operation.error = err.Error()
			return
		}
		v.opResult = result
		v.conflictWorktree = ""
		operation.dialog.Open = false
		v.refreshRequested = true
		if len(result.Conflicts) != 0 {
			v.conflictWorktree = path
			v.selected = selection{kind: rowWIP, worktree: path}
			v.loadWIP(e, path)
		}
	})
}

func (v *viewState) operationDone(result *protocol.GitOpResult) func(error) {
	return func(err error) {
		v.operation.running = false
		if err != nil {
			v.operation.error = err.Error()
			return
		}
		v.opResult = *result
		v.operation.dialog.Open = false
		v.conflictWorktree = ""
		v.refreshRequested = true
	}
}

func (v *viewState) runConflictAction(e *env.Env, method string) {
	path := v.selected.worktree
	var result protocol.GitOpResult
	e.Call(method, protocol.WorktreeRef{Path: path}, &result, func(err error) {
		if err != nil {
			v.detailError = err.Error()
			return
		}
		v.opResult = result
		v.conflictWorktree = path
		if len(result.Conflicts) == 0 {
			v.conflictWorktree = ""
			v.refreshRequested = true
		}
		v.loadWIP(e, path)
	})
}
