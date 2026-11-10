package agent

import (
	"image"
	"path/filepath"
	"sort"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"
	material "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var moreIcon = kit.Icon(material.NavigationMoreVert)

const (
	groupNeeds   = "needs"
	groupWorking = "working"
	groupDone    = "done"
	groupIdle    = "idle"
)

type Page struct {
	selected                                                  string
	showList                                                  bool
	rows                                                      map[string]*look.SummaryRow
	scroll                                                    look.Scroller
	newOpen                                                   bool
	newAgent, stopAll, back, toggle                           kit.Plaque
	dialog                                                    kit.Dialog
	revertConfirm                                             kit.Confirm
	revertTurn                                                int
	create, cancel                                            kit.Plaque
	folder, worktreeName, base, branch                        kit.TextField
	projectChoice, worktreeChoice, harnessChoice, model, mode look.Choice
	projectIndex, worktreeIndex, harnessIndex                 int
	openFolder, openTerminal, changes                         kit.Plaque
	more                                                      kit.IconPlaque
	moreMenu                                                  kit.Menu
	moreSize                                                  image.Point
	rename                                                    kit.TextField
	renameDo, renameCancel                                    kit.Plaque
	renameDialog                                              kit.Dialog
	renameOpen                                                bool
	deleteConfirm                                             kit.Confirm
	deleteOpen                                                bool
	view                                                      chat.View
	composer                                                  chat.Composer
	error                                                     string
}

type hubGroup struct {
	id, label string
	sessions  []string
}
type listEntry struct {
	header         string
	count          int
	session, group string
}

func (p *Page) Layout(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	if p.rows == nil {
		p.rows = make(map[string]*look.SummaryRow)
	}
	groups := groupSessions(e.Snapshot())
	if p.newAgent.Clicked(gtx) {
		p.openDialog(e)
	}
	if p.stopAll.Clicked(gtx) {
		p.stopEveryAgent(e)
	}
	if p.back.Clicked(gtx) {
		p.selected, p.showList = "", true
	}
	if p.toggle.Clicked(gtx) {
		p.showList = !p.showList
	}
	if p.selected == "" {
		return p.layoutHub(gtx, e, groups)
	}
	entries := flattenGroups(groups)
	if class == kit.FrameExpanded {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(384), gtx.Dp(384)
				return p.layoutList(gtx, e, entries)
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutWork(gtx, e, class) }),
		)
	}
	if p.showList {
		return p.layoutList(gtx, e, entries)
	}
	return p.layoutWork(gtx, e, class)
}

func (p *Page) NeedsYou(e *env.Env) int {
	for _, group := range groupSessions(e.Snapshot()) {
		if group.id == groupNeeds {
			return len(group.sessions)
		}
	}
	return 0
}

func (p *Page) Overlay(gtx layout.Context, e *env.Env) layout.Dimensions {
	if p.revertTurn > 0 {
		p.revertConfirm.Danger = true
		p.revertConfirm.Dialog.Open = true
		if p.revertConfirm.Layout(gtx, e.Theme.Theme, i18n.T("Revert turn files"), i18n.T("Restore the worktree files to their state before this turn?"), i18n.T("Revert files"), "") {
			var result protocol.GitOpResult
			e.Call(protocol.MSessionRevertTurn, protocol.TurnParams{ID: p.selected, Turn: p.revertTurn}, &result, nil)
			p.revertTurn = 0
		}
		if p.revertConfirm.Closed() {
			p.revertTurn = 0
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	if p.renameOpen {
		p.rename.Label = i18n.T("Session name")
		p.renameDo.Kind, p.renameDo.Label = kit.PlaqueGold, i18n.T("Rename")
		p.renameCancel.Kind, p.renameCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
		if p.renameCancel.Clicked(gtx) {
			p.renameOpen = false
			p.renameDialog.Dismiss(gtx)
		}
		if p.renameDo.Clicked(gtx) && strings.TrimSpace(p.rename.Text()) != "" {
			var changed protocol.SessionInfo
			e.Call(protocol.MSessionRename, protocol.RenameParams{ID: p.selected, Title: strings.TrimSpace(p.rename.Text())}, &changed, nil)
			p.renameOpen = false
			p.renameDialog.Dismiss(gtx)
		}
		return p.renameDialog.Layout(gtx, e.Theme.Theme, i18n.T("Rename session"), func(gtx layout.Context) layout.Dimensions {
			return p.rename.Layout(gtx, e.Theme.Theme)
		}, func(gtx layout.Context) layout.Dimensions {
			return p.renameCancel.Layout(gtx, e.Theme.Theme)
		}, func(gtx layout.Context) layout.Dimensions {
			return p.renameDo.Layout(gtx, e.Theme.Theme)
		})
	}
	if p.deleteOpen {
		p.deleteConfirm.Danger = true
		p.deleteConfirm.Dialog.Open = true
		if p.deleteConfirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete session"), i18n.T("Delete this Agent session and its transcript?"), i18n.T("Delete"), "") {
			id := p.selected
			p.deleteOpen = false
			e.Call(protocol.MSessionDelete, protocol.SessionRef{ID: id}, new(protocol.Empty), func(err error) {
				if err == nil {
					p.selected, p.showList = "", true
				}
			})
		}
		if p.deleteConfirm.Closed() {
			p.deleteOpen = false
		}
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	if !p.newOpen {
		return layout.Dimensions{}
	}
	snapshot := e.Snapshot()
	projects := snapshot.Projects
	projectLabels := []string{i18n.T("No project")}
	for _, project := range projects {
		projectLabels = append(projectLabels, project.Name)
	}
	harnesses := installedHarnesses(snapshot)
	harnessLabels := make([]string, len(harnesses))
	for i, harness := range harnesses {
		harnessLabels[i] = harness.Name
	}
	if len(harnessLabels) == 0 {
		harnessLabels = []string{i18n.T("No installed harness")}
	}
	if p.projectIndex > len(projects) {
		p.projectIndex = 0
	}
	if p.harnessIndex >= len(harnesses) {
		p.harnessIndex = 0
	}

	p.cancel.Kind, p.cancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	p.create.Kind, p.create.Label = kit.PlaqueGold, i18n.T("Create")
	if p.cancel.Clicked(gtx) {
		p.closeDialog(gtx)
	}
	if p.create.Clicked(gtx) && len(harnesses) > 0 {
		p.createSession(e, projects, harnesses)
		p.closeDialog(gtx)
	}
	return p.dialog.Layout(gtx, e.Theme.Theme, i18n.T("New agent session"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Project"))),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := p.projectChoice.Layout(gtx, e.Theme, e.Overlays, projectLabels, p.projectIndex)
				if n, ok := p.projectChoice.Changed(); ok {
					p.projectIndex, p.worktreeIndex = n, 0
				}
				return d
			}),
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
		}
		if p.projectIndex == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.folder.Label, p.folder.Placeholder = i18n.T("Folder"), paths.Home()
				return p.folder.Layout(gtx, e.Theme.Theme)
			}))
		} else {
			project := projects[p.projectIndex-1]
			worktreeLabels := []string{i18n.T("Project folder"), i18n.T("New worktree")}
			for _, worktree := range snapshot.Worktrees[project.ID] {
				worktreeLabels = append(worktreeLabels, worktree.Name)
			}
			if p.worktreeIndex >= len(worktreeLabels) {
				p.worktreeIndex = 0
			}
			children = append(children,
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Worktree"))),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					d := p.worktreeChoice.Layout(gtx, e.Theme, e.Overlays, worktreeLabels, p.worktreeIndex)
					if n, ok := p.worktreeChoice.Changed(); ok {
						p.worktreeIndex = n
					}
					return d
				}),
			)
			if p.worktreeIndex == 1 {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: 13}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.worktreeName.Label = i18n.T("Name")
						return p.worktreeName.Layout(gtx, e.Theme.Theme)
					}),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.base.Label, p.base.Placeholder = i18n.T("Base"), project.BaseRef
						return p.base.Layout(gtx, e.Theme.Theme)
					}),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.branch.Label = i18n.T("Branch")
						return p.branch.Layout(gtx, e.Theme.Theme)
					}),
				)
			}
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Harness"))),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				d := p.harnessChoice.Layout(gtx, e.Theme, e.Overlays, harnessLabels, p.harnessIndex)
				if n, ok := p.harnessChoice.Changed(); ok {
					p.harnessIndex = n
				}
				return d
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, approvalsHint(snapshot.Settings.Agent.Approvals))),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, func(gtx layout.Context) layout.Dimensions { return p.cancel.Layout(gtx, e.Theme.Theme) }, func(gtx layout.Context) layout.Dimensions { return p.create.Layout(gtx, e.Theme.Theme) })
}

func (p *Page) NewSession(e *env.Env) { p.openDialog(e) }
func (p *Page) Select(e *env.Env, id string) {
	if id != "" {
		p.selectSession(e, id)
	}
}
func (p *Page) FocusComposer() { p.composer.Focus() }

func (p *Page) openDialog(e *env.Env) {
	if p.folder.Text() == "" {
		p.folder.SetText(paths.Home())
	}
	defaultHarness := e.Snapshot().Settings.Agent.Harness
	for index, harness := range installedHarnesses(e.Snapshot()) {
		if harness.ID == defaultHarness {
			p.harnessIndex = index
		}
	}
	for _, project := range e.Snapshot().Projects {
		e.LoadWorktrees(project.ID, nil)
	}
	p.newOpen, p.dialog.Open = true, true
}
func (p *Page) closeDialog(gtx layout.Context) { p.newOpen = false; p.dialog.Dismiss(gtx) }

func (p *Page) createSession(e *env.Env, projects []protocol.ProjectInfo, harnesses []protocol.HarnessInfo) {
	params := protocol.SessionNewParams{Kind: protocol.KindAgent, Harness: harnesses[p.harnessIndex].ID, Origin: protocol.OriginApp}
	if p.projectIndex == 0 {
		params.Cwd = strings.TrimSpace(p.folder.Text())
	} else {
		project := projects[p.projectIndex-1]
		params.Project = project.ID
		worktrees := e.Snapshot().Worktrees[project.ID]
		switch {
		case p.worktreeIndex == 1:
			params.Worktree, params.Name = "new", strings.TrimSpace(p.worktreeName.Text())
			params.Base, params.Branch = strings.TrimSpace(p.base.Text()), strings.TrimSpace(p.branch.Text())
		case p.worktreeIndex >= 2 && p.worktreeIndex-2 < len(worktrees):
			params.Worktree = worktrees[p.worktreeIndex-2].Path
		default:
			params.Worktree = ""
		}
	}
	var info protocol.SessionInfo
	e.Call(protocol.MSessionNew, params, &info, func(err error) {
		if err != nil {
			p.error = err.Error()
			return
		}
		p.selectSession(e, info.ID)
	})
}

func (p *Page) selectSession(e *env.Env, id string) {
	p.selected, p.showList = id, false
	e.Attach(id, func(err error) {
		if err != nil {
			p.error = err.Error()
			return
		}
		e.Call(protocol.MSessionRead, protocol.SessionRef{ID: id}, new(protocol.Empty), nil)
	})
}

func (p *Page) layoutHub(gtx layout.Context, e *env.Env, groups []hubGroup) layout.Dimensions {
	entries := flattenGroups(groups)
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 34, Bottom: 13, Left: 34}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					p.newAgent.Kind, p.newAgent.Label = kit.PlaqueGold, i18n.T("New session")
					p.stopAll.Kind, p.stopAll.Label = kit.PlaqueLine, i18n.T("Stop all")
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, i18n.T("Agents"))), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.stopAll.Layout(gtx, e.Theme.Theme) }), layout.Rigid(layout.Spacer{Width: 8}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.newAgent.Layout(gtx, e.Theme.Theme) }))
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutEntries(gtx, e, entries) }),
			)
		})
	})
}

func (p *Page) layoutList(gtx layout.Context, e *env.Env, entries []listEntry) layout.Dimensions {
	return (kit.Pane{Kind: kit.PaneList}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 10, Bottom: 13, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			p.back.Kind, p.back.Label = kit.PlaqueQuiet, i18n.T("All agents")
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.back.Layout(gtx, e.Theme.Theme) }), layout.Rigid(layout.Spacer{Height: 8}.Layout), layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutEntries(gtx, e, entries) }))
		})
	})
}

func (p *Page) layoutEntries(gtx layout.Context, e *env.Env, entries []listEntry) layout.Dimensions {
	if len(entries) == 0 {
		return (kit.Empty{Title: i18n.T("No agent sessions"), Body: i18n.T("Create a session to start work in a folder or project.")}).Layout(gtx, e.Theme.Theme)
	}
	return p.scroll.Layout(gtx, e.Theme, len(entries), func(gtx layout.Context, index int) layout.Dimensions {
		entry := entries[index]
		if entry.header != "" {
			return kit.GroupHeader(e.Theme.Theme, entry.header, entry.count)(gtx)
		}
		info := e.Snapshot().Sessions[entry.session]
		row := p.rows[entry.session]
		if row == nil {
			row = new(look.SummaryRow)
			p.rows[entry.session] = row
		}
		if row.Clicked(gtx) || row.DoubleClicked(gtx) {
			p.selectSession(e, entry.session)
		}
		row.Title = info.Title
		row.State = groupLabel(entry.group)
		row.Meta = strings.Trim(strings.Join([]string{harnessName(e.Snapshot(), info.Harness), info.Model, sessionPlace(e.Snapshot(), info)}, " · "), " ·")
		row.Excerpt = sessionExcerpt(e.Snapshot(), info, entry.group)
		row.Tone = groupTone(entry.group)
		return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return row.Layout(gtx, e.Theme, entry.session == p.selected)
		})
	})
}

func (p *Page) layoutWork(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		attached, ok := e.Snapshot().Attached[p.selected]
		if !ok {
			body := i18n.T("Open a session to see its conversation.")
			if p.error != "" {
				body = p.error
			}
			return (kit.Empty{Title: i18n.T("Opening agent session"), Body: body}).Layout(gtx, e.Theme.Theme)
		}
		info := attached.Info
		side := 34
		if class == kit.FrameCompact {
			side = 21
		}
		return layout.Inset{Top: 21, Right: unit.Dp(side), Bottom: 13, Left: unit.Dp(side)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.header(gtx, e, info) }),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					d := p.view.Layout(gtx, e.Theme.Theme, attached.Model)
					p.handleActions(gtx, e, info.ID, p.view.Actions())
					return d
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					modelLabels, modelIDs, selectedModel := choiceOptions(attached.Model.ModelChoice(), info.Model)
					modeLabels, modeIDs, selectedMode := choiceOptions(attached.Model.ModeChoice(), info.Mode)
					modelControl := func(gtx layout.Context) layout.Dimensions {
						dimensions := p.model.Layout(gtx, e.Theme, e.Overlays, modelLabels, selectedModel)
						if index, changed := p.model.Changed(); changed && index < len(modelIDs) {
							e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: info.ID, Model: modelIDs[index]}, new(protocol.Empty), nil)
						}
						return dimensions
					}
					modeControl := func(gtx layout.Context) layout.Dimensions {
						dimensions := p.mode.Layout(gtx, e.Theme, e.Overlays, modeLabels, selectedMode)
						if index, changed := p.mode.Changed(); changed && index < len(modeIDs) {
							e.Call(protocol.MSessionSetMode, protocol.SetModeParams{ID: info.ID, Mode: modeIDs[index]}, new(protocol.Empty), nil)
						}
						return dimensions
					}
					dimensions := p.composer.Layout(gtx, e.Theme.Theme, chat.ComposerOptions{
						Placeholder: i18n.T("Tell the agent what to do"), Busy: attached.Model.Busy(), Queue: info.Queue,
						Controls: []layout.Widget{modelControl, modeControl}, CanAttach: true,
					})
					if submission, ok := p.composer.Submitted(); ok {
						p.submit(e, info, attached.Model.Busy(), submission)
					}
					p.handleActions(gtx, e, info.ID, p.composer.Actions())
					if p.composer.StopClicked() {
						e.Call(protocol.MSessionCancel, protocol.SessionRef{ID: info.ID}, new(protocol.Empty), nil)
					}
					return dimensions
				}),
			)
		})
	})
}

func (p *Page) header(gtx layout.Context, e *env.Env, info protocol.SessionInfo) layout.Dimensions {
	p.openFolder.Kind, p.openFolder.Label, p.openFolder.Small = kit.PlaqueQuiet, i18n.T("Open folder"), true
	p.openTerminal.Kind, p.openTerminal.Label, p.openTerminal.Small = kit.PlaqueQuiet, i18n.T("Open terminal"), true
	p.changes.Kind, p.changes.Label, p.changes.Small = kit.PlaqueQuiet, i18n.T("Changes"), true
	p.toggle.Kind, p.toggle.Label, p.toggle.Small = kit.PlaqueQuiet, i18n.T("Sessions"), true
	p.more.Kind, p.more.Icon, p.more.Description, p.more.Small = kit.PlaqueQuiet, moreIcon, i18n.T("More session actions"), true
	cwd := info.Worktree
	if cwd == "" {
		cwd = info.Cwd
	}
	if p.openFolder.Clicked(gtx) && cwd != "" {
		env.Spawn("xdg-open", cwd)
	}
	if p.openTerminal.Clicked(gtx) && cwd != "" {
		env.Spawn("ryoku-app", "terminal", "--", "sh", "-c", `cd "$1" && exec "${SHELL:-/bin/sh}"`, "sh", cwd)
	}
	if p.changes.Clicked(gtx) && info.Project != "" {
		e.NavigateWork(info.Project, "changes", info.Worktree)
	}
	if p.more.Clicked(gtx) {
		tag := &p.more.Clickable
		content := func(gtx layout.Context) layout.Dimensions {
			items := []kit.MenuItem{{Label: i18n.T("Rename")}, {Label: i18n.T("Delete")}}
			dimensions := p.moreMenu.Layout(gtx, e.Theme.Theme, items)
			if selected, ok := p.moreMenu.Clicked(gtx); ok {
				e.Overlays.Close(tag)
				switch selected {
				case 0:
					p.rename.SetText(info.Title)
					p.renameOpen, p.renameDialog.Open = true, true
				case 1:
					p.deleteOpen = true
				}
			}
			return dimensions
		}
		e.Overlays.Open(tag, e.Overlays.AnchorOf(&p.more.Clickable, p.moreSize), look.Placement{Align: look.End}, content)
	}
	group := groupIdle
	if sessionNeedsYou(e.Snapshot(), info) {
		group = groupNeeds
	} else if info.State == protocol.StateWorking {
		group = groupWorking
	} else if info.Unread {
		group = groupDone
	}
	meta := strings.Trim(strings.Join([]string{projectName(e.Snapshot(), info.Project), filepath.Base(cwd), info.Branch, harnessName(e.Snapshot(), info.Harness)}, " · "), " ·")
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, info.Title)),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Label, e.Theme.R.Stone(groupTone(group)), groupLabel(group))),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.openFolder.Layout(gtx, e.Theme.Theme) }),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.openTerminal.Layout(gtx, e.Theme.Theme) }),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if info.Project == "" {
						return layout.Dimensions{}
					}
					return p.changes.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Width: 4}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.more.Layout(gtx, e.Theme.Theme)
					p.moreSize = dimensions.Size
					return dimensions
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.toggle.Layout(gtx, e.Theme.Theme) }),
				layout.Rigid(layout.Spacer{Width: 8}.Layout),
				layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, meta)),
			)
		}),
	)
}

func (p *Page) submit(e *env.Env, info protocol.SessionInfo, busy bool, submission chat.Submission) {
	if busy && !submission.Steer {
		var changed protocol.SessionInfo
		e.Call(protocol.MSessionQueue, protocol.QueueParams{ID: info.ID, Text: submission.Text}, &changed, nil)
		return
	}
	images := make([]protocol.Attachment, len(submission.Images))
	for i, path := range submission.Images {
		images[i] = protocol.Attachment{Path: path}
	}
	var result protocol.PromptResult
	method := protocol.MSessionPrompt
	if submission.Steer {
		method = protocol.MSessionSteer
	}
	e.Call(method, protocol.PromptParams{ID: info.ID, Text: submission.Text, Images: images}, &result, nil)
}

func (p *Page) handleActions(gtx layout.Context, e *env.Env, id string, actions []chat.Action) {
	var common []chat.Action
	for _, action := range actions {
		switch action.Kind {
		case chat.ActUnqueue:
			var changed protocol.SessionInfo
			e.Call(protocol.MSessionUnqueue, protocol.UnqueueParams{ID: id, Queue: action.Value}, &changed, nil)
		case chat.ActFork:
			var info protocol.SessionInfo
			e.Call(protocol.MSessionFork, protocol.ForkParams{ID: id, Seq: action.Seq}, &info, func(err error) {
				if err == nil {
					p.selectSession(e, info.ID)
				}
			})
		case chat.ActTurnChanges:
			if info, ok := e.Snapshot().Sessions[id]; ok && info.Project != "" {
				e.NavigateWork(info.Project, "changes", info.Worktree)
			}
		case chat.ActRevertTurn:
			p.revertTurn = action.Turn
		default:
			common = append(common, action)
		}
	}
	env.HandleChatActions(gtx, e, id, common)
}

func (p *Page) stopEveryAgent(e *env.Env) {
	for _, info := range e.Snapshot().Sessions {
		if info.Kind == protocol.KindAgent && (info.Alive || info.State == protocol.StateWorking || info.State == protocol.StateWaiting) {
			e.Call(protocol.MSessionStop, protocol.SessionRef{ID: info.ID}, new(protocol.Empty), nil)
		}
	}
}

func groupSessions(snapshot state.Snapshot) []hubGroup {
	groups := []hubGroup{{id: groupNeeds, label: i18n.T("Needs you")}, {id: groupWorking, label: i18n.T("Working")}, {id: groupDone, label: i18n.T("Done")}, {id: groupIdle, label: i18n.T("Idle")}}
	for _, id := range snapshot.Order {
		info := snapshot.Sessions[id]
		if info.Kind != protocol.KindAgent {
			continue
		}
		group := groupIdle
		if sessionNeedsYou(snapshot, info) {
			group = groupNeeds
		} else if info.State == protocol.StateWorking {
			group = groupWorking
		} else if info.Unread {
			group = groupDone
		}
		for index := range groups {
			if groups[index].id == group {
				groups[index].sessions = append(groups[index].sessions, id)
				break
			}
		}
	}
	return groups
}

func sessionNeedsYou(snapshot state.Snapshot, info protocol.SessionInfo) bool {
	if info.Pending > 0 || info.State == protocol.StateWaiting || info.State == protocol.StateFailed {
		return true
	}
	if info.Task != "" {
		for _, task := range snapshot.Tasks[info.Project] {
			if task.ID == info.Task {
				if task.Status == protocol.TaskNeedsYou {
					return true
				}
				for _, gate := range task.Gates {
					if gate.Resolution == "" {
						return true
					}
				}
			}
		}
	}
	if info.Worktree != "" {
		for _, worktree := range snapshot.Worktrees[info.Project] {
			if worktree.Path == info.Worktree && worktree.Status != nil && worktree.Status.Conflicted > 0 {
				return true
			}
		}
	}
	return false
}

func flattenGroups(groups []hubGroup) []listEntry {
	var entries []listEntry
	for _, group := range groups {
		entries = append(entries, listEntry{header: group.label, count: len(group.sessions), group: group.id})
		for _, id := range group.sessions {
			entries = append(entries, listEntry{session: id, group: group.id})
		}
	}
	return entries
}
func installedHarnesses(snapshot state.Snapshot) []protocol.HarnessInfo {
	result := make([]protocol.HarnessInfo, 0)
	for _, harness := range snapshot.Harnesses {
		if harness.Installed {
			result = append(result, harness)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Default != result[j].Default {
			return result[i].Default
		}
		return result[i].Name < result[j].Name
	})
	return result
}
func harnessName(snapshot state.Snapshot, id string) string {
	for _, harness := range snapshot.Harnesses {
		if harness.ID == id {
			return harness.Name
		}
	}
	return id
}
func projectName(snapshot state.Snapshot, id string) string {
	if id == "" {
		return i18n.T("No project")
	}
	for _, project := range snapshot.Projects {
		if project.ID == id {
			return project.Name
		}
	}
	return id
}
func sessionPlace(snapshot state.Snapshot, info protocol.SessionInfo) string {
	parts := []string{}
	if info.Project != "" {
		parts = append(parts, projectName(snapshot, info.Project))
	}
	if info.Worktree != "" {
		parts = append(parts, filepath.Base(info.Worktree))
	}
	if info.Branch != "" {
		parts = append(parts, info.Branch)
	}
	if len(parts) == 0 {
		parts = append(parts, info.Cwd)
	}
	return strings.Join(parts, " · ")
}
func groupLabel(group string) string {
	switch group {
	case groupNeeds:
		return i18n.T("Needs you")
	case groupWorking:
		return i18n.T("Working")
	case groupDone:
		return i18n.T("Done")
	default:
		return i18n.T("Idle")
	}
}
func groupTone(group string) palette.Tone {
	switch group {
	case groupNeeds:
		return palette.Attention
	case groupWorking:
		return palette.Becoming
	case groupDone:
		return palette.Alive
	default:
		return palette.Idle
	}
}
func sessionExcerpt(snapshot state.Snapshot, info protocol.SessionInfo, group string) string {
	if group == groupWorking {
		return info.Activity
	}
	if group != groupNeeds {
		return ""
	}
	if attached, ok := snapshot.Attached[info.ID]; ok {
		if pending := attached.Model.Pending(); len(pending) > 0 {
			return pending[0].Title
		}
	}
	for _, task := range snapshot.Tasks[info.Project] {
		if task.ID != info.Task {
			continue
		}
		for _, gate := range task.Gates {
			if gate.Resolution == "" {
				return gate.Question
			}
		}
	}
	if info.Error != "" {
		return info.Error
	}
	return info.Preview
}

func approvalsHint(value string) string {
	switch value {
	case protocol.ApprovalsAuto:
		return i18n.T("Auto approval is the current default. The harness may act without asking.")
	case protocol.ApprovalsAsk:
		return i18n.T("Ask is the current default. Every action asks first.")
	default:
		return i18n.T("Read only is the current default. Writes ask first.")
	}
}
func choiceOptions(choice protocol.ChoiceData, fallback string) ([]string, []string, int) {
	current := choice.Current
	if current == "" {
		current = fallback
	}
	if len(choice.Options) == 0 {
		if current == "" {
			return []string{i18n.T("Default")}, []string{""}, 0
		}
		return []string{current}, []string{current}, 0
	}
	labels, ids, selected := make([]string, len(choice.Options)), make([]string, len(choice.Options)), 0
	for i, option := range choice.Options {
		labels[i], ids[i] = option.Name, option.ID
		if option.ID == current {
			selected = i
		}
	}
	return labels, ids, selected
}
