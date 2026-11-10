package projects

import (
	"fmt"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/protocol"
)

func (o *overviewState) layout(gtx layout.Context, e *env.Env, page *pageState, class kit.FrameClass) layout.Dimensions {
	worktrees := e.Snapshot().Worktrees[page.project]
	if page.worktree == "" && len(worktrees) > 0 {
		page.worktree = worktrees[0].Path
	}
	o.handleToolbar(gtx, e, page)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutToolbar(gtx, e, page) }),
		layout.Rigid(layout.Spacer{Height: 10}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutRemovePreview(gtx, e, page) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return o.scroll.Layout(gtx, e.Theme, len(worktrees)+1, func(gtx layout.Context, index int) layout.Dimensions {
				if index == len(worktrees) {
					return o.settings.layout(gtx, e, page)
				}
				return o.layoutWorktree(gtx, e, page, worktrees[index])
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return o.layoutDialogs(gtx, e, page, worktrees) }),
	)
}

func (o *overviewState) handleToolbar(gtx layout.Context, e *env.Env, page *pageState) {
	if o.newButton.Clicked(gtx) {
		o.newOpen, o.newDialog.Open = true, true
		if info, ok := page.projectInfo(e); ok {
			o.newBase.SetText(info.Settings.BaseRef)
			o.newBranch.SetText(info.Settings.BranchPrefix)
		}
		var log protocol.GitLogResult
		e.Call(protocol.MGitLog, protocol.GitLogParams{Project: page.project, Limit: 1}, &log, func(err error) {
			if err == nil {
				o.refs = log.Refs
			}
		})
	}
	if o.repairButton.Clicked(gtx) {
		var result protocol.WorktreesResult
		e.Call(protocol.MWorktreesRepair, protocol.ProjectRef{Project: page.project}, &result, func(err error) {
			if err == nil {
				e.Store.SetWorktrees(result)
			} else {
				page.error = err.Error()
			}
		})
	}
	if o.pruneButton.Clicked(gtx) {
		e.Call(protocol.MWorktreesPrune, protocol.PruneParams{Project: page.project, DryRun: true}, &o.prune, func(err error) {
			if err == nil {
				o.pruneOpen, o.pruneConfirm.Dialog.Open = true, true
			} else {
				page.error = err.Error()
			}
		})
	}
	if o.cleanupButton.Clicked(gtx) {
		e.Call(protocol.MWorktreesCleanup, protocol.CleanupParams{Project: page.project, DryRun: true}, &o.cleanup, func(err error) {
			if err == nil {
				o.cleanupOpen, o.cleanupConfirm.Dialog.Open = true, true
			} else {
				page.error = err.Error()
			}
		})
	}
}

func (o *overviewState) layoutToolbar(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	o.newButton.Kind, o.newButton.Label, o.newButton.Small = kit.PlaqueGold, i18n.T("New worktree"), true
	o.cleanupButton.Kind, o.cleanupButton.Label, o.cleanupButton.Small = kit.PlaqueQuiet, i18n.T("Clean up"), true
	o.pruneButton.Kind, o.pruneButton.Label, o.pruneButton.Small = kit.PlaqueQuiet, i18n.T("Prune"), true
	o.repairButton.Kind, o.repairButton.Label, o.repairButton.Small = kit.PlaqueQuiet, i18n.T("Repair"), true
	buttons := []*kit.Plaque{
		&o.newButton,
		&o.cleanupButton,
		&o.pruneButton,
		&o.repairButton,
	}
	children := make([]layout.FlexChild, 0, len(buttons)*2)
	for index, button := range buttons {
		if index != 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		}
		current := button
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return current.Layout(gtx, e.Theme.Theme)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (o *overviewState) layoutWorktree(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
) layout.Dimensions {
	row := o.rows[worktree.Path]
	if row == nil {
		row = &worktreeRow{sessionOpen: make(map[string]*kit.Plaque)}
		o.rows[worktree.Path] = row
	}
	o.handleWorktreeActions(gtx, e, page, worktree, row)
	return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(gtx, e.Theme.R.Stone2, e.Theme.Cut.Small, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(e.Theme.Sp.S3).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return o.layoutWorktreeHeader(gtx, e, worktree)
					}),
				}
				if status := statusSummary(worktree.Status); status != "" {
					children = append(
						children,
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, status)),
					)
				}
				if state := effectiveSetupState(worktree, row); state != protocol.SetupDone && state != "" {
					children = append(
						children,
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return o.layoutSetupState(gtx, e, worktree, row, state)
						}),
					)
				}
				if len(row.portsResult.Ports) > 0 {
					children = append(
						children,
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return o.layoutPorts(gtx, e, row)
						}),
					)
				}
				children = append(
					children,
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return o.layoutWorktreeButtons(gtx, e, worktree, row)
					}),
				)
				if len(worktree.Sessions) > 0 {
					children = append(
						children,
						layout.Rigid(layout.Spacer{Height: 4}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return o.layoutSessions(gtx, e, worktree, row)
						}),
					)
				}
				if row.setupDisclosure.Open {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return row.setupDisclosure.Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
							return o.layoutSetup(gtx, e, worktree, row)
						})
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
}

func (o *overviewState) layoutWorktreeHeader(
	gtx layout.Context,
	e *env.Env,
	worktree protocol.WorktreeInfo,
) layout.Dimensions {
	branch := worktree.Branch
	if worktree.Detached {
		branch = i18n.Tf("Detached at %s", shortHash(worktree.Head))
	}
	facts := []string{worktreeKind(worktree), branch}
	if worktree.Status != nil {
		status := worktree.Status
		if status.Upstream != "" || status.Ahead != 0 || status.Behind != 0 {
			facts = append(facts, i18n.Tf("upstream +%d -%d", status.Ahead, status.Behind))
		}
		if worktree.Base != "" || status.AheadBase != 0 || status.BehindBase != 0 {
			facts = append(facts, i18n.Tf("base +%d -%d", status.AheadBase, status.BehindBase))
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := kit.Label{Style: e.Theme.T.Section, Color: e.Theme.R.Fg, MaxLines: 1}
			return label.Layout(gtx, e.Theme.Theme, worktree.Name)
		}),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, strings.Join(facts, " · "))),
	)
}

func (o *overviewState) handleWorktreeActions(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) {
	if row.openFolder.Clicked(gtx) {
		env.Spawn("xdg-open", worktree.Path)
	}
	if row.openTerminal.Clicked(gtx) {
		env.Spawn("ryoku-app", "terminal", "--cwd", worktree.Path)
	}
	if row.newSession.Clicked(gtx) {
		page.newSession(e, worktree.Path)
	}
	if row.more.Clicked(gtx) {
		o.openWorktreeMenu(gtx, e, page, worktree, row)
	}
	if row.showSetup.Clicked(gtx) {
		row.setupDisclosure.Open = !row.setupDisclosure.Open
		if row.setupResult.State == "" {
			o.loadSetup(e, page, worktree, row, false)
		}
	}
	if row.trust.Clicked(gtx) {
		e.Call(
			protocol.MWorktreesTrust,
			protocol.TrustParams{Path: worktree.Path, Hash: row.setupResult.Hash},
			&row.setupResult,
			func(err error) {
				if err != nil {
					page.error = err.Error()
				}
				e.LoadWorktrees(page.project, nil)
			},
		)
	}
}

func (o *overviewState) openWorktreeMenu(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) {
	if e.Overlays == nil {
		return
	}
	if e.Overlays.IsOpen(&row.moreTag) {
		e.Overlays.Close(&row.moreTag)
		return
	}
	lockWord := i18n.T("Lock")
	if worktree.Locked {
		lockWord = i18n.T("Unlock")
	}
	items := []kit.MenuItem{
		{Label: lockWord},
		{Label: i18n.T("Move")},
		{Label: i18n.T("Run setup")},
		{Label: i18n.T("Ports")},
		{Label: i18n.T("Repair")},
		{Label: i18n.T("Remove"), Danger: true},
	}
	content := func(gtx layout.Context) layout.Dimensions {
		dimensions := row.moreMenu.Layout(gtx, e.Theme.Theme, items)
		if index, clicked := row.moreMenu.Clicked(gtx); clicked {
			e.Overlays.Close(&row.moreTag)
			o.handleWorktreeMenu(e, page, worktree, row, index)
			gtx.Execute(op.InvalidateCmd{})
		}
		return dimensions
	}
	e.Overlays.OpenAt(&row.moreTag, e.Overlays.Press(), content)
}

func (o *overviewState) handleWorktreeMenu(
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
	index int,
) {
	switch index {
	case 0:
		var changed protocol.WorktreeInfo
		params := protocol.WorktreeLockParams{
			Path:   worktree.Path,
			Lock:   !worktree.Locked,
			Reason: i18n.T("Locked in Rashin"),
		}
		e.Call(protocol.MWorktreesLock, params, &changed, func(error) {
			e.LoadWorktrees(page.project, nil)
		})
	case 1:
		o.movePath, o.moveTo, o.moveDialog.Open = worktree.Path, "", true
		o.moveField.SetText(worktree.Path)
	case 2:
		o.runSetup(e, page, worktree, row)
	case 3:
		e.Call(
			protocol.MWorktreesPorts,
			protocol.WorktreeRef{Path: worktree.Path},
			&row.portsResult,
			func(err error) {
				if err != nil {
					page.error = err.Error()
				}
			},
		)
	case 4:
		var result protocol.WorktreesResult
		e.Call(
			protocol.MWorktreesRepair,
			protocol.ProjectRef{Project: page.project},
			&result,
			func(err error) {
				if err != nil {
					page.error = err.Error()
					return
				}
				e.Store.SetWorktrees(result)
			},
		)
	case 5:
		e.Call(
			protocol.MWorktreesPreviewRemove,
			protocol.WorktreeRef{Path: worktree.Path},
			&o.removePreview,
			func(err error) {
				if err != nil {
					page.error = err.Error()
					return
				}
				o.removeOpen = true
			},
		)
	}
}

func (o *overviewState) runSetup(
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) {
	row.setupResult.State = protocol.SetupRunning
	o.loadSetup(e, page, worktree, row, true)
}

func (o *overviewState) loadSetup(
	e *env.Env,
	page *pageState,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
	rerun bool,
) {
	params := protocol.WorktreeSetupParams{Path: worktree.Path, Rerun: rerun}
	e.Call(protocol.MWorktreesSetup, params, &row.setupResult, func(err error) {
		if err != nil {
			page.error = err.Error()
		}
		row.setupDisclosure.Open = row.setupResult.State == protocol.SetupFailed ||
			row.setupResult.State == protocol.SetupUntrusted
		e.LoadWorktrees(page.project, nil)
	})
}

func (o *overviewState) layoutWorktreeButtons(
	gtx layout.Context,
	e *env.Env,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) layout.Dimensions {
	buttons := []struct {
		button *kit.Plaque
		label  string
	}{
		{button: &row.openFolder, label: i18n.T("Open folder")},
		{button: &row.openTerminal, label: i18n.T("Open terminal")},
		{button: &row.newSession, label: i18n.T("New session here")},
		{button: &row.more, label: i18n.T("More")},
	}
	children := make([]layout.FlexChild, 0, len(buttons)*2)
	for index, item := range buttons {
		item.button.Kind, item.button.Label, item.button.Small = kit.PlaqueQuiet, item.label, true
		if index != 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		}
		button := item.button
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return button.Layout(gtx, e.Theme.Theme)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (o *overviewState) layoutSetupState(
	gtx layout.Context,
	e *env.Env,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
	state string,
) layout.Dimensions {
	row.showSetup.Kind, row.showSetup.Small = kit.PlaqueQuiet, true
	switch state {
	case protocol.SetupRunning:
		working := kit.Working{Active: true}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return working.Layout(gtx, e.Theme.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("Setup running"))),
		)
	case protocol.SetupFailed:
		row.showSetup.Label = i18n.T("Show log")
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Jasper, i18n.T("Setup failed"))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.showSetup.Layout(gtx, e.Theme.Theme)
			}),
		)
	case protocol.SetupUntrusted:
		row.showSetup.Label = i18n.T("Review and trust")
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Amber, i18n.T("Setup needs review"))),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.showSetup.Layout(gtx, e.Theme.Theme)
			}),
		)
	default:
		return kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg3, i18n.T("Setup not run"))(gtx)
	}
}

func (o *overviewState) layoutSetup(
	gtx layout.Context,
	e *env.Env,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) layout.Dimensions {
	result := row.setupResult
	children := []layout.FlexChild{layout.Rigid(layout.Spacer{Height: 4}.Layout)}
	if result.Command != "" {
		children = append(
			children,
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, result.Command)),
		)
	}
	if result.Script != "" {
		children = append(
			children,
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, result.Script)),
		)
	}
	if result.Log != "" {
		children = append(
			children,
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg3, result.Log)),
		)
	}
	if result.State == protocol.SetupUntrusted {
		row.trust.Kind, row.trust.Label, row.trust.Small = kit.PlaqueLine, i18n.T("Trust and run"), true
		children = append(
			children,
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return row.trust.Layout(gtx, e.Theme.Theme)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (o *overviewState) layoutPorts(
	gtx layout.Context,
	e *env.Env,
	row *worktreeRow,
) layout.Dimensions {
	if len(row.portOpen) < len(row.portsResult.Ports) {
		missing := len(row.portsResult.Ports) - len(row.portOpen)
		row.portOpen = append(row.portOpen, make([]kit.Plaque, missing)...)
	}
	children := make([]layout.FlexChild, 0, len(row.portsResult.Ports)*2)
	for index, port := range row.portsResult.Ports {
		button := &row.portOpen[index]
		if button.Clicked(gtx) {
			env.Spawn("xdg-open", port.URL)
		}
		button.Kind, button.Label, button.Small = kit.PlaqueQuiet, i18n.T("Open"), true
		if index != 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: 4}.Layout))
		}
		label := i18n.Tf("%d %s", port.Port, port.Command)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Slab(gtx, e.Theme.R.Stone3, e.Theme.Cut.Small, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, label)),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return button.Layout(gtx, e.Theme.Theme)
						}),
					)
				})
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (o *overviewState) layoutSessions(
	gtx layout.Context,
	e *env.Env,
	worktree protocol.WorktreeInfo,
	row *worktreeRow,
) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(worktree.Sessions))
	for _, id := range worktree.Sessions {
		button := row.sessionOpen[id]
		if button == nil {
			button = new(kit.Plaque)
			row.sessionOpen[id] = button
		}
		if button.Clicked(gtx) && e.Navigate != nil {
			e.Navigate(protocol.TabAgent, id)
		}
		button.Kind, button.Label, button.Small = kit.PlaqueQuiet, i18n.T("Open"), true
		info := e.Snapshot().Sessions[id]
		title := info.Title
		if title == "" {
			title = id
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := kit.Label{Style: e.Theme.T.Caption, Color: e.Theme.R.Fg2, MaxLines: 1}
					return label.Layout(gtx, e.Theme.Theme, title)
				}),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg3, string(info.State))),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return button.Layout(gtx, e.Theme.Theme)
				}),
			)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (o *overviewState) layoutRemovePreview(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	if !o.removeOpen {
		return layout.Dimensions{}
	}
	if o.removeCancel.Clicked(gtx) {
		o.removeOpen = false
	}
	if o.removeApprove.Clicked(gtx) {
		o.removeConfirm.Danger, o.removeConfirm.Dialog.Open = true, true
	}
	o.removeDeleteBranch.Label = i18n.T("Delete branch")
	o.removeForceBranch.Label = i18n.T("Force delete branch")
	o.removeApprove.Kind, o.removeApprove.Label = kit.PlaqueDanger, i18n.T("Remove")
	o.removeCancel.Kind, o.removeCancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	body := removePreviewText(o.removePreview)
	return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return kit.Slab(
			gtx,
			e.Theme.R.Field(palette.Attention),
			e.Theme.Cut.Mid,
			func(gtx layout.Context) layout.Dimensions {
				inset := layout.UniformInset(e.Theme.Sp.S4)
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return o.layoutRemovePreviewContent(gtx, e, body)
				})
			},
		)
	})
}

func (o *overviewState) layoutRemovePreviewContent(
	gtx layout.Context,
	e *env.Env,
	body string,
) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(
			e.Theme.Theme,
			e.Theme.T.Section,
			e.Theme.R.Fg,
			i18n.T("Remove worktree"),
		)),
		layout.Rigid(layout.Spacer{Height: 5}.Layout),
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Body, e.Theme.R.Fg2, body)),
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return o.removeDeleteBranch.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !o.removeDeleteBranch.Value {
				gtx = gtx.Disabled()
			}
			return o.removeForceBranch.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Height: 6}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return o.removeCancel.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 6}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return o.removeApprove.Layout(gtx, e.Theme.Theme)
				}),
			)
		}),
	)
}

func (o *overviewState) layoutDialogs(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktrees []protocol.WorktreeInfo,
) layout.Dimensions {
	o.layoutNewDialog(gtx, e, page, worktrees)
	o.layoutMoveDialog(gtx, e, page)
	o.layoutRemoveConfirm(gtx, e, page)
	o.layoutPruneConfirm(gtx, e, page)
	o.layoutCleanupConfirm(gtx, e, page)
	return layout.Dimensions{}
}

func (o *overviewState) layoutRemoveConfirm(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) {
	if !o.removeConfirm.Dialog.Open {
		return
	}
	proof := ""
	if o.removePreview.NeedsForce {
		proof = o.removePreview.Branch
		if proof == "" {
			proof = o.removePreview.Path
		}
	}
	confirmed := o.removeConfirm.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Confirm worktree removal"),
		removePreviewText(o.removePreview),
		i18n.T("Remove"),
		proof,
	)
	if !confirmed {
		return
	}
	params := removeParams(
		o.removePreview,
		o.removeDeleteBranch.Value,
		o.removeForceBranch.Value,
	)
	e.Call(protocol.MWorktreesRemove, params, new(protocol.Empty), func(err error) {
		if err != nil {
			page.error = err.Error()
			return
		}
		o.removeOpen = false
		e.LoadWorktrees(page.project, nil)
	})
}

func (o *overviewState) layoutPruneConfirm(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) {
	if !o.pruneOpen {
		return
	}
	body := strings.Join(o.prune.Pruned, "\n")
	if body == "" {
		body = i18n.T("Nothing needs pruning.")
	}
	confirmed := o.pruneConfirm.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Prune stale worktrees"),
		body,
		i18n.T("Prune"),
		"",
	)
	if !confirmed {
		return
	}
	params := protocol.PruneParams{Project: page.project}
	e.Call(protocol.MWorktreesPrune, params, new(protocol.PruneResult), func(error) {
		e.LoadWorktrees(page.project, nil)
	})
	o.pruneOpen = false
}

func (o *overviewState) layoutCleanupConfirm(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) {
	if !o.cleanupOpen {
		return
	}
	lines := make([]string, len(o.cleanup.Items))
	for index, item := range o.cleanup.Items {
		action := i18n.T("Keep")
		if item.Removed {
			action = i18n.T("Remove")
		}
		lines[index] = action + " · " + item.Path + " · " + item.Reason
	}
	body := strings.Join(lines, "\n")
	if body == "" {
		body = i18n.T("No owned worktrees match the cleanup policy.")
	}
	confirmed := o.cleanupConfirm.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Clean up worktrees"),
		body,
		i18n.T("Run cleanup"),
		"",
	)
	if !confirmed {
		return
	}
	params := protocol.CleanupParams{Project: page.project}
	e.Call(protocol.MWorktreesCleanup, params, new(protocol.CleanupResult), func(error) {
		e.LoadWorktrees(page.project, nil)
	})
	o.cleanupOpen = false
}

func (o *overviewState) layoutNewDialog(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	worktrees []protocol.WorktreeInfo,
) {
	if !o.newOpen {
		return
	}
	modes := []string{
		i18n.T("New branch"),
		i18n.T("Existing branch"),
		i18n.T("Detached"),
	}
	modeIDs := []string{
		protocol.WorktreeNewBranch,
		protocol.WorktreeExistingBranch,
		protocol.WorktreeDetached,
	}
	parents := []string{i18n.T("None")}
	parentIDs := []string{""}
	for _, worktree := range worktrees {
		parents = append(parents, worktree.Name)
		parentIDs = append(parentIDs, worktree.Path)
	}
	refLabels := []string{i18n.T("Project default")}
	refIDs := []string{""}
	for _, ref := range o.refs {
		refLabels = append(refLabels, ref.Name)
		refIDs = append(refIDs, ref.Name)
	}
	if o.newCancel.Clicked(gtx) {
		o.newOpen = false
		o.newDialog.Dismiss(gtx)
	}
	if o.newCreate.Clicked(gtx) {
		o.createWorktree(gtx, e, page, modeIDs, parentIDs)
	}
	o.newCancel.Kind, o.newCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	o.newCreate.Kind, o.newCreate.Label = kit.PlaqueGold, i18n.T("Create worktree")
	body := func(gtx layout.Context) layout.Dimensions {
		return o.layoutNewDialogBody(gtx, e, modes, parents, refLabels, refIDs)
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return o.newCancel.Layout(gtx, e.Theme.Theme)
	}
	create := func(gtx layout.Context) layout.Dimensions {
		return o.newCreate.Layout(gtx, e.Theme.Theme)
	}
	o.newDialog.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("New worktree"),
		body,
		cancel,
		create,
	)
}

func (o *overviewState) createWorktree(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
	modeIDs []string,
	parentIDs []string,
) {
	name := strings.TrimSpace(o.newName.Text())
	branch := strings.TrimSpace(o.newBranch.Text())
	info, hasInfo := page.projectInfo(e)
	useDefaultBranch := branch == "" || branch == info.Settings.BranchPrefix
	if hasInfo && o.newModeSelected == 0 && useDefaultBranch {
		branch = info.Settings.BranchPrefix + name
	}
	params := protocol.WorktreeCreateParams{
		Project: page.project,
		Name:    name,
		Mode:    modeIDs[o.newModeSelected],
		Base:    strings.TrimSpace(o.newBase.Text()),
		Branch:  branch,
		Parent:  parentIDs[o.newParentSelected],
		Lock:    o.newLock.Value,
		Setup:   o.newSetup.Value,
	}
	var result protocol.WorktreeInfo
	e.Call(protocol.MWorktreesCreate, params, &result, func(err error) {
		if err != nil {
			page.error = err.Error()
			return
		}
		o.newOpen = false
		o.newDialog.Dismiss(gtx)
		page.worktree = result.Path
		e.LoadWorktrees(page.project, nil)
	})
}

func (o *overviewState) layoutNewDialogBody(
	gtx layout.Context,
	e *env.Env,
	modes []string,
	parents []string,
	refLabels []string,
	refIDs []string,
) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			o.newName.Label = i18n.T("Name")
			return o.newName.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := o.newMode.Layout(
				gtx,
				e.Theme,
				e.Overlays,
				modes,
				o.newModeSelected,
			)
			if index, changed := o.newMode.Changed(); changed {
				o.newModeSelected = index
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := o.newRef.Layout(
				gtx,
				e.Theme,
				e.Overlays,
				refLabels,
				o.newRefSelected,
			)
			if index, changed := o.newRef.Changed(); changed {
				o.newRefSelected = index
				if refIDs[index] != "" {
					o.newBase.SetText(refIDs[index])
				}
			}
			return dimensions
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			o.newBase.Label = i18n.T("Base ref")
			return o.newBase.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			o.newBranch.Label = i18n.T("Branch")
			return o.newBranch.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := o.newParent.Layout(
				gtx,
				e.Theme,
				e.Overlays,
				parents,
				o.newParentSelected,
			)
			if index, changed := o.newParent.Changed(); changed {
				o.newParentSelected = index
			}
			return dimensions
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			o.newLock.Label = i18n.T("Lock after creating")
			return o.newLock.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			o.newSetup.Label = i18n.T("Run setup")
			return o.newSetup.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func (o *overviewState) layoutMoveDialog(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) {
	if !o.moveDialog.Open {
		return
	}
	if o.moveCancel.Clicked(gtx) {
		o.moveDialog.Dismiss(gtx)
	}
	if o.moveDo.Clicked(gtx) {
		var result protocol.WorktreeInfo
		params := protocol.WorktreeMoveParams{
			Path: o.movePath,
			To:   strings.TrimSpace(o.moveField.Text()),
		}
		e.Call(protocol.MWorktreesMove, params, &result, func(err error) {
			if err != nil {
				page.error = err.Error()
				return
			}
			o.moveDialog.Dismiss(gtx)
			e.LoadWorktrees(page.project, nil)
		})
	}
	o.moveCancel.Kind, o.moveCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	o.moveDo.Kind, o.moveDo.Label = kit.PlaqueGold, i18n.T("Move")
	body := func(gtx layout.Context) layout.Dimensions {
		o.moveField.Label = i18n.T("New folder")
		o.moveField.Mono = true
		return o.moveField.Layout(gtx, e.Theme.Theme)
	}
	cancel := func(gtx layout.Context) layout.Dimensions {
		return o.moveCancel.Layout(gtx, e.Theme.Theme)
	}
	move := func(gtx layout.Context) layout.Dimensions {
		return o.moveDo.Layout(gtx, e.Theme.Theme)
	}
	o.moveDialog.Layout(
		gtx,
		e.Theme.Theme,
		i18n.T("Move worktree"),
		body,
		cancel,
		move,
	)
}

func (s *settingsState) layout(
	gtx layout.Context,
	e *env.Env,
	page *pageState,
) layout.Dimensions {
	info, ok := page.projectInfo(e)
	if !ok {
		return layout.Dimensions{}
	}
	if s.project != page.project {
		s.load(page.project, info)
	}
	if s.save.Clicked(gtx) {
		s.saveSettings(e, page)
	}
	s.save.Kind, s.save.Label = kit.PlaqueLine, i18n.T("Save project settings")
	s.mergeDays.Min = 0
	s.mergeDays.Max = 365
	s.mergeDays.Step = 1
	s.mergeDays.Unit = i18n.T("days after merge")
	s.inactiveDays.Min = 0
	s.inactiveDays.Max = 365
	s.inactiveDays.Step = 1
	s.inactiveDays.Unit = i18n.T("inactive days")
	return layout.Inset{Top: 8, Bottom: 8}.Layout(
		gtx,
		func(gtx layout.Context) layout.Dimensions {
			return kit.SlabContent(
				gtx,
				e.Theme.Theme,
				func(gtx layout.Context) layout.Dimensions {
					return s.layoutContent(gtx, e)
				},
			)
		},
	)
}

func (s *settingsState) load(project string, info protocol.ProjectInfo) {
	s.project = project
	s.base.SetText(info.Settings.BaseRef)
	s.root.SetText(info.Settings.WorktreeRoot)
	s.prefix.SetText(info.Settings.BranchPrefix)
	s.setup.SetText(info.Settings.Setup)
	s.copyIgnored.SetText(strings.Join(info.Settings.CopyIgnored, ", "))
	s.auto.Value = info.Settings.Cleanup.Auto
	s.mergeDays.Value = info.Settings.Cleanup.AfterMergeDays
	s.inactiveDays.Value = info.Settings.Cleanup.InactiveDays
	if info.Settings.Cleanup.Protect == protocol.ProtectAny {
		s.protectSelected = 1
		return
	}
	s.protectSelected = 0
}

func (s *settingsState) saveSettings(e *env.Env, page *pageState) {
	protect := protocol.ProtectUncommitted
	if s.protectSelected == 1 {
		protect = protocol.ProtectAny
	}
	patch := protocol.ProjectSettings{
		BaseRef:      s.base.Text(),
		WorktreeRoot: s.root.Text(),
		BranchPrefix: s.prefix.Text(),
		Setup:        s.setup.Text(),
		CopyIgnored:  splitList(s.copyIgnored.Text()),
		Cleanup: protocol.CleanupPolicy{
			Auto:           s.auto.Value,
			AfterMergeDays: s.mergeDays.Value,
			InactiveDays:   s.inactiveDays.Value,
			Protect:        protect,
		},
	}
	var changed protocol.ProjectInfo
	params := protocol.ProjectSettingsParams{
		Project: page.project,
		Patch:   rawPatch(patch),
	}
	e.Call(protocol.MProjectsSettings, params, &changed, func(err error) {
		if err != nil {
			page.error = err.Error()
		}
	})
}

func (s *settingsState) layoutContent(
	gtx layout.Context,
	e *env.Env,
) layout.Dimensions {
	protect := []string{
		i18n.T("Protect uncommitted changes"),
		i18n.T("Protect any local file"),
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(
			e.Theme.Theme,
			e.Theme.T.Section,
			e.Theme.R.Fg,
			i18n.T("Project settings"),
		)),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.base.Label = i18n.T("Base ref")
			return s.base.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.root.Label = i18n.T("Worktree folder")
			return s.root.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.prefix.Label = i18n.T("Branch prefix")
			return s.prefix.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.setup.Label = i18n.T("Setup command")
			return s.setup.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.copyIgnored.Label = i18n.T("Files copied into new worktrees")
			return s.copyIgnored.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			s.auto.Label = i18n.T("Automatic cleanup")
			return s.auto.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.mergeDays.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.inactiveDays.Layout(gtx, e.Theme.Theme)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := s.protect.Layout(
				gtx,
				e.Theme,
				e.Overlays,
				protect,
				s.protectSelected,
			)
			if index, changed := s.protect.Changed(); changed {
				s.protectSelected = index
			}
			return dimensions
		}),
		layout.Rigid(layout.Spacer{Height: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.save.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func removeParams(preview protocol.RemovePreview, deleteBranch, forceBranch bool) protocol.WorktreeRemoveParams {
	return protocol.WorktreeRemoveParams{Path: preview.Path, Force: preview.NeedsForce, DeleteBranch: deleteBranch, ForceBranch: deleteBranch && forceBranch}
}

func removePreviewText(preview protocol.RemovePreview) string {
	parts := []string{}
	if len(preview.Dirty) > 0 {
		parts = append(parts, i18n.Tf("Dirty: %s", strings.Join(preview.Dirty, ", ")))
	}
	if len(preview.Untracked) > 0 {
		parts = append(parts, i18n.Tf("Untracked: %s", strings.Join(preview.Untracked, ", ")))
	}
	if len(preview.Unpushed) > 0 {
		parts = append(parts, i18n.Tf("Unpushed: %s", strings.Join(preview.Unpushed, ", ")))
	}
	if len(preview.Running) > 0 {
		parts = append(parts, i18n.Tf("Running sessions: %s", strings.Join(preview.Running, ", ")))
	}
	if preview.Locked {
		parts = append(parts, i18n.T("This worktree is locked."))
	}
	if preview.Unmerged {
		parts = append(parts, i18n.T("The branch is not merged."))
	}
	if preview.BlockReason != "" {
		parts = append(parts, preview.BlockReason)
	}
	if len(parts) == 0 {
		parts = append(parts, i18n.T("The worktree is clean."))
	}
	return strings.Join(parts, "\n")
}

func worktreeKind(worktree protocol.WorktreeInfo) string {
	kinds := make([]string, 0, 5)
	if worktree.Main {
		kinds = append(kinds, i18n.T("Main"))
	}
	if worktree.Owned {
		kinds = append(kinds, i18n.T("Owned"))
	}
	if worktree.Locked {
		kinds = append(kinds, i18n.T("Locked"))
	}
	if worktree.Missing {
		kinds = append(kinds, i18n.T("Missing"))
	}
	if worktree.Prunable {
		kinds = append(kinds, i18n.T("Prunable"))
	}
	return strings.Join(kinds, " · ")
}

func statusSummary(status *protocol.WorktreeStatus) string {
	if status == nil {
		return ""
	}
	parts := make([]string, 0, 7)
	if status.Staged != 0 {
		parts = append(parts, i18n.Tf("%d staged", status.Staged))
	}
	if status.Modified != 0 {
		parts = append(parts, i18n.Tf("%d modified", status.Modified))
	}
	if status.Untracked != 0 {
		parts = append(parts, i18n.Tf("%d untracked", status.Untracked))
	}
	if status.Conflicted != 0 {
		parts = append(parts, i18n.Tf("%d conflicts", status.Conflicted))
	}
	if status.Added != 0 || status.Removed != 0 {
		parts = append(parts, fmt.Sprintf("+%d -%d", status.Added, status.Removed))
	}
	if status.Operation != "" {
		parts = append(parts, i18n.Tf("%s in progress", status.Operation))
	}
	return strings.Join(parts, " · ")
}

func effectiveSetupState(worktree protocol.WorktreeInfo, row *worktreeRow) string {
	if row.setupResult.State != "" {
		return row.setupResult.State
	}
	return worktree.Setup
}
func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}
