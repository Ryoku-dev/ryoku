package ask

import (
	"image"
	"sort"
	"strings"

	"gioui.org/layout"
	material "golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

var moreIcon = kit.Icon(material.NavigationMoreVert)

type Page struct {
	selected                     string
	showList                     bool
	rows                         map[string]*look.SummaryRow
	scroll                       look.Scroller
	view                         chat.View
	composer                     chat.Composer
	newAsk                       kit.Plaque
	toggle                       kit.Plaque
	more                         kit.IconPlaque
	moreMenu                     kit.Menu
	moreSize                     image.Point
	rename                       kit.TextField
	renameDo, renameCancel       kit.Plaque
	renameDialog                 kit.Dialog
	renameOpen                   bool
	deleteConfirm                kit.Confirm
	deleteOpen                   bool
	choice                       look.Choice
	models                       protocol.ModelsResult
	loaded                       bool
	error                        string
	handoffOpen                  bool
	handoffDialog                kit.Dialog
	handoffCreate, handoffCancel kit.Plaque
	handoffProject               look.Choice
	handoffWorktree              look.Choice
	handoffHarness               look.Choice
	handoffProjectIndex          int
	handoffWorktreeIndex         int
	handoffHarnessIndex          int
	handoffFolder, handoffName   kit.TextField
	handoffBase, handoffBranch   kit.TextField
	handoffText                  string
}

func New() *Page { return &Page{showList: true, rows: make(map[string]*look.SummaryRow)} }

func (p *Page) FocusComposer() { p.composer.Focus() }

func (p *Page) Select(e *env.Env, id string) {
	if id == "" || p.selected == id {
		return
	}
	p.selected = id
	p.showList = false
	e.Attach(id, func(err error) {
		if err != nil {
			p.error = err.Error()
		}
	})
}

func (p *Page) NewSession(e *env.Env) {
	var info protocol.SessionInfo
	e.Call(protocol.MSessionNew, protocol.SessionNewParams{Kind: protocol.KindAsk, Origin: protocol.OriginApp}, &info, func(err error) {
		if err != nil {
			p.error = err.Error()
			return
		}
		p.Select(e, info.ID)
	})
}

func (p *Page) Layout(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	if p.rows == nil {
		p.rows = make(map[string]*look.SummaryRow)
	}
	p.loadModels(e)
	ids := p.sessionIDs(e)
	if p.selected == "" && len(ids) > 0 {
		p.Select(e, ids[0])
	}
	if p.newAsk.Clicked(gtx) {
		p.NewSession(e)
	}
	if p.toggle.Clicked(gtx) {
		p.showList = !p.showList
	}

	if class == kit.FrameExpanded {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(384), gtx.Dp(384)
				return p.layoutList(gtx, e, ids)
			}),
			layout.Rigid(layout.Spacer{Width: 8}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutWork(gtx, e) }),
		)
	}
	if p.showList || p.selected == "" {
		return p.layoutList(gtx, e, ids)
	}
	return p.layoutWork(gtx, e)
}

func (p *Page) layoutList(gtx layout.Context, e *env.Env, ids []string) layout.Dimensions {
	return kit.Pane{Kind: kit.PaneList}.Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 21, Right: 10, Bottom: 13, Left: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Section, e.Theme.R.Fg, i18n.T("Ask"))),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					p.newAsk.Kind, p.newAsk.Label = kit.PlaqueGold, i18n.T("New ask")
					return p.newAsk.Layout(gtx, e.Theme.Theme)
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return p.scroll.Layout(gtx, e.Theme, len(ids), func(gtx layout.Context, index int) layout.Dimensions {
						id := ids[index]
						info := e.Snapshot().Sessions[id]
						row := p.rows[id]
						if row == nil {
							row = new(look.SummaryRow)
							p.rows[id] = row
						}
						if row.Clicked(gtx) || row.DoubleClicked(gtx) {
							p.Select(e, id)
						}
						row.Title = info.Title
						row.State = stateLabel(info.State)
						row.Meta = originLabel(info.Origin)
						row.Excerpt = info.Preview
						row.Tone = sessionTone(info.State)
						return layout.Inset{Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return row.Layout(gtx, e.Theme, id == p.selected)
						})
					})
				}),
			)
		})
	})
}

func (p *Page) layoutWork(gtx layout.Context, e *env.Env) layout.Dimensions {
	return kit.Pane{Kind: kit.PaneWork}.Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		attached, ok := e.Snapshot().Attached[p.selected]
		if !ok {
			return p.layoutWaiting(gtx, e)
		}
		info := attached.Info
		return layout.Inset{Top: 21, Right: 34, Bottom: 13, Left: 34}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info) }),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					dimensions := p.view.Layout(gtx, e.Theme.Theme, attached.Model)
					p.handleActions(gtx, e, p.selected, p.view.Actions())
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					labels, ids, selected := p.modelOptions(info.Model)
					control := func(gtx layout.Context) layout.Dimensions {
						dimensions := p.choice.Layout(gtx, e.Theme, e.Overlays, labels, selected)
						if index, changed := p.choice.Changed(); changed && index >= 0 && index < len(ids) {
							e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: info.ID, Model: ids[index]}, new(protocol.Empty), nil)
						}
						return dimensions
					}
					dimensions := p.composer.Layout(gtx, e.Theme.Theme, chat.ComposerOptions{
						Placeholder: i18n.T("Ask about this machine"), Busy: attached.Model.Busy(), Controls: []layout.Widget{control}, CanAttach: true,
					})
					if submission, submitted := p.composer.Submitted(); submitted {
						p.prompt(e, p.selected, submission)
					}
					if p.composer.StopClicked() {
						e.Call(protocol.MSessionCancel, protocol.SessionRef{ID: p.selected}, new(protocol.Empty), nil)
					}
					return dimensions
				}),
			)
		})
	})
}

func (p *Page) layoutHeader(gtx layout.Context, e *env.Env, info protocol.SessionInfo) layout.Dimensions {
	p.toggle.Kind, p.toggle.Label, p.toggle.Small = kit.PlaqueQuiet, i18n.T("Sessions"), true
	p.more.Kind, p.more.Icon, p.more.Description, p.more.Small = kit.PlaqueQuiet, moreIcon, i18n.T("More session actions"), true
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
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.Title, e.Theme.R.Fg, info.Title)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.toggle.Layout(gtx, e.Theme.Theme) }),
		layout.Rigid(layout.Spacer{Width: 8}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := p.more.Layout(gtx, e.Theme.Theme)
			p.moreSize = dimensions.Size
			return dimensions
		}),
	)
}

func (p *Page) layoutWaiting(gtx layout.Context, e *env.Env) layout.Dimensions {
	body := i18n.T("Open a session to see its conversation.")
	if p.error != "" {
		body = p.error
	}
	empty := kit.Empty{Title: i18n.T("No ask open"), Body: body}
	return empty.Layout(gtx, e.Theme.Theme)
}

func (p *Page) prompt(e *env.Env, id string, submission chat.Submission) {
	images := make([]protocol.Attachment, len(submission.Images))
	for index, path := range submission.Images {
		images[index] = protocol.Attachment{Path: path}
	}
	var result protocol.PromptResult
	e.Call(protocol.MSessionPrompt, protocol.PromptParams{ID: id, Text: submission.Text, Images: images}, &result, func(err error) {
		if err != nil {
			p.error = err.Error()
		}
	})
}

func (p *Page) handleActions(gtx layout.Context, e *env.Env, id string, actions []chat.Action) {
	var common []chat.Action
	for _, action := range actions {
		if action.Kind == chat.ActHandoff {
			p.handoffOpen, p.handoffDialog.Open, p.handoffText = true, true, action.Value
			for _, project := range e.Snapshot().Projects {
				e.LoadWorktrees(project.ID, nil)
			}
			continue
		}
		common = append(common, action)
	}
	env.HandleChatActions(gtx, e, id, common)
}

// Overlay lays out every page-owned dialog while it is open.
func (p *Page) Overlay(gtx layout.Context, e *env.Env) layout.Dimensions {
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
		if p.deleteConfirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete session"), i18n.T("Delete this Ask session and its transcript?"), i18n.T("Delete"), "") {
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
	if !p.handoffOpen {
		return layout.Dimensions{}
	}
	snapshot := e.Snapshot()
	projectLabels := []string{i18n.T("No project")}
	for _, project := range snapshot.Projects {
		projectLabels = append(projectLabels, project.Name)
	}
	harnesses := make([]protocol.HarnessInfo, 0)
	for _, harness := range snapshot.Harnesses {
		if harness.Installed {
			harnesses = append(harnesses, harness)
		}
	}
	harnessLabels := make([]string, len(harnesses))
	for index, harness := range harnesses {
		harnessLabels[index] = harness.Name
	}
	if len(harnessLabels) == 0 {
		harnessLabels = []string{i18n.T("No installed harness")}
	}
	if p.handoffProjectIndex > len(snapshot.Projects) {
		p.handoffProjectIndex = 0
	}
	if p.handoffHarnessIndex >= len(harnesses) {
		p.handoffHarnessIndex = 0
	}
	p.handoffCancel.Kind, p.handoffCancel.Label = kit.PlaqueLine, i18n.T("Cancel")
	p.handoffCreate.Kind, p.handoffCreate.Label = kit.PlaqueGold, i18n.T("Move")
	if p.handoffCancel.Clicked(gtx) {
		p.handoffOpen = false
		p.handoffDialog.Dismiss(gtx)
	}
	if p.handoffCreate.Clicked(gtx) && len(harnesses) > 0 {
		params := protocol.HandoffParams{ID: p.selected, Harness: harnesses[p.handoffHarnessIndex].ID, Run: true, Text: p.handoffText}
		if p.handoffProjectIndex == 0 {
			params.Cwd = strings.TrimSpace(p.handoffFolder.Text())
		} else {
			project := snapshot.Projects[p.handoffProjectIndex-1]
			params.Project = project.ID
			worktrees := snapshot.Worktrees[project.ID]
			switch {
			case p.handoffWorktreeIndex == 1:
				params.Worktree, params.Name = "new", strings.TrimSpace(p.handoffName.Text())
				params.Base, params.Branch = strings.TrimSpace(p.handoffBase.Text()), strings.TrimSpace(p.handoffBranch.Text())
			case p.handoffWorktreeIndex >= 2 && p.handoffWorktreeIndex-2 < len(worktrees):
				params.Worktree = worktrees[p.handoffWorktreeIndex-2].Path
			}
		}
		var info protocol.SessionInfo
		e.Call(protocol.MSessionHandoff, params, &info, func(err error) {
			if err != nil {
				p.error = err.Error()
				return
			}
			e.Attach(info.ID, func(error) {})
			e.Navigate(protocol.TabAgent, info.ID)
		})
		p.handoffOpen = false
		p.handoffDialog.Dismiss(gtx)
	}
	return p.handoffDialog.Layout(gtx, e.Theme.Theme, i18n.T("Move to Agent"), func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Project"))),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dimensions := p.handoffProject.Layout(gtx, e.Theme, e.Overlays, projectLabels, p.handoffProjectIndex)
				if index, changed := p.handoffProject.Changed(); changed {
					p.handoffProjectIndex, p.handoffWorktreeIndex = index, 0
				}
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
		}
		if p.handoffProjectIndex == 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				p.handoffFolder.Label, p.handoffFolder.Placeholder = i18n.T("Folder"), paths.Home()
				if p.handoffFolder.Text() == "" {
					p.handoffFolder.SetText(paths.Home())
				}
				return p.handoffFolder.Layout(gtx, e.Theme.Theme)
			}))
		} else {
			project := snapshot.Projects[p.handoffProjectIndex-1]
			labels := []string{i18n.T("Project folder"), i18n.T("New worktree")}
			for _, worktree := range snapshot.Worktrees[project.ID] {
				labels = append(labels, worktree.Name)
			}
			children = append(children,
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Worktree"))),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.handoffWorktree.Layout(gtx, e.Theme, e.Overlays, labels, p.handoffWorktreeIndex)
					if index, changed := p.handoffWorktree.Changed(); changed {
						p.handoffWorktreeIndex = index
					}
					return dimensions
				}),
			)
			if p.handoffWorktreeIndex == 1 {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: 13}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.handoffName.Label = i18n.T("Name")
						return p.handoffName.Layout(gtx, e.Theme.Theme)
					}),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.handoffBase.Label, p.handoffBase.Placeholder = i18n.T("Base"), project.BaseRef
						return p.handoffBase.Layout(gtx, e.Theme.Theme)
					}),
					layout.Rigid(layout.Spacer{Height: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						p.handoffBranch.Label = i18n.T("Branch")
						return p.handoffBranch.Layout(gtx, e.Theme.Theme)
					}),
				)
			}
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: 13}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Harness"))),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dimensions := p.handoffHarness.Layout(gtx, e.Theme, e.Overlays, harnessLabels, p.handoffHarnessIndex)
				if index, changed := p.handoffHarness.Changed(); changed {
					p.handoffHarnessIndex = index
				}
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("The new Agent session keeps this question and answer as its starting context."))),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}, func(gtx layout.Context) layout.Dimensions { return p.handoffCancel.Layout(gtx, e.Theme.Theme) }, func(gtx layout.Context) layout.Dimensions { return p.handoffCreate.Layout(gtx, e.Theme.Theme) })
}

func (p *Page) sessionIDs(e *env.Env) []string {
	snapshot := e.Snapshot()
	ids := make([]string, 0, len(snapshot.Order))
	for _, id := range snapshot.Order {
		if snapshot.Sessions[id].Kind == protocol.KindAsk {
			ids = append(ids, id)
		}
	}
	return ids
}

func (p *Page) loadModels(e *env.Env) {
	if p.loaded {
		return
	}
	p.loaded = true
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &p.models, func(err error) {
		if err != nil {
			p.loaded = false
			p.error = err.Error()
		}
	})
}

func (p *Page) modelOptions(current string) ([]string, []string, int) {
	labels, ids := make([]string, 0, len(p.models.Routes)+len(p.models.Models)), make([]string, 0, len(p.models.Routes)+len(p.models.Models))
	for _, route := range p.models.Routes {
		labels, ids = append(labels, route.Name), append(ids, route.ID)
	}
	models := append([]protocol.ModelInfo(nil), p.models.Models...)
	sort.SliceStable(models, func(i, j int) bool { return models[i].Provider+models[i].Name < models[j].Provider+models[j].Name })
	for _, model := range models {
		if model.Available {
			labels, ids = append(labels, model.Name), append(ids, model.Provider+"/"+model.ID)
		}
	}
	selected := 0
	for index, id := range ids {
		if id == current {
			selected = index
		}
	}
	return labels, ids, selected
}

func sessionTone(value protocol.SessionState) palette.Tone {
	switch value {
	case protocol.StateWorking:
		return palette.Becoming
	case protocol.StateWaiting:
		return palette.Attention
	case protocol.StateFailed:
		return palette.Failed
	case protocol.StateStopped:
		return palette.Idle
	default:
		return palette.Alive
	}
}

func stateLabel(value protocol.SessionState) string {
	switch value {
	case protocol.StateWorking:
		return i18n.T("Working")
	case protocol.StateWaiting:
		return i18n.T("Waiting")
	case protocol.StateFailed:
		return i18n.T("Failed")
	case protocol.StateStopped:
		return i18n.T("Stopped")
	default:
		return i18n.T("Ready")
	}
}

func originLabel(origin string) string {
	switch origin {
	case protocol.OriginBar:
		return i18n.T("From the bar")
	case protocol.OriginCLI:
		return i18n.T("From the command line")
	case protocol.OriginTerminal:
		return i18n.T("From a terminal")
	case protocol.OriginFix:
		return i18n.T("From Fix")
	case protocol.OriginImport:
		return i18n.T("Imported")
	default:
		return i18n.T("From the app")
	}
}

func modelLabel(model string) string {
	if model == "" || model == protocol.RouteAuto {
		return i18n.T("Auto")
	}
	return model
}
