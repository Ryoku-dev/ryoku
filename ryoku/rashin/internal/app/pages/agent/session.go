package agent

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const (
	actionChanges = iota
	actionTerminal
	actionFolder
	actionScreen
)

const (
	menuRename = iota
	menuStop
	_
	menuDelete
)

func (p *Page) layoutSession(gtx layout.Context, e *env.Env, info protocol.SessionInfo) layout.Dimensions {
	attached, open := e.Snapshot().Attached[p.selected]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !open {
				return layout.Dimensions{Size: gtx.Constraints.Max}
			}
			if screen, ok := e.Snapshot().Screens[info.ID]; ok && p.screen.large {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(1100))
					return p.screen.layout(gtx, e, screen)
				})
			}
			p.screen.large = false
			return p.layoutConversation(gtx, e, attached)
		}),
	)
}

func (p *Page) layoutHeader(gtx layout.Context, e *env.Env, info protocol.SessionInfo) layout.Dimensions {
	snapshot := e.Snapshot()
	content := look.PaneHeaderContent{
		Title: sessionTitle(info), Facts: sessionFacts(snapshot, info), Scrolled: p.view.Scrolled(),
		Claim: sessionClaim(snapshot, info), ClaimTone: sessionTone(snapshot, info),
	}
	worktree, tracked := worktreeOf(snapshot, info)
	folder := sessionFolder(info)
	actions := make([]int, 0, 4)
	screen, screened := snapshot.Screens[info.ID]
	if screened {
		if screen.Ask != "" && screen.Ask != p.asked {
			p.asked = screen.Ask
			p.side = sideScreen
			if !e.PanelOpen {
				e.TogglePanel()
			}
		}
		action := look.HeaderAction{Icon: look.Icon.Screen, Label: i18n.T("Screen"), Description: i18n.T("Watch the agent's screen"), On: e.PanelOpen && (p.side == sideScreen || !tracked)}
		if screen.Ask != "" {
			action.Trail = func(gtx layout.Context) layout.Dimensions {
				return kit.Flake(gtx, e.Theme.Theme, palette.Attention, 1, 9)
			}
		}
		content.Actions, actions = append(content.Actions, action), append(actions, actionScreen)
	}
	if tracked {
		change := look.HeaderAction{Icon: look.Icon.Diff, Label: i18n.T("Changes"), Description: i18n.T("Show what changed") + "  Ctrl \\", On: e.PanelOpen && (p.side == sideChanges || !screened)}
		if status := worktree.Status; status != nil && status.Added+status.Removed > 0 {
			change.Trail = look.DiffCount(e.Theme, status.Added, status.Removed, e.PanelOpen)
		}
		content.Actions, actions = append(content.Actions, change), append(actions, actionChanges)
	}
	if folder != "" {
		content.Actions = append(content.Actions,
			look.HeaderAction{Icon: look.Icon.Terminal, Description: i18n.T("Open a terminal here")},
			look.HeaderAction{Icon: look.Icon.Folder, Description: i18n.T("Open the folder")},
		)
		actions = append(actions, actionTerminal, actionFolder)
	}
	running := info.Alive || info.State == protocol.StateWorking || info.State == protocol.StateWaiting
	content.Menu = []kit.MenuItem{
		menuRename: {Icon: look.Icon.Rename, Label: i18n.T("Rename")},
		menuStop:   {Icon: look.Icon.Stop, Label: i18n.T("Stop the agent"), Disabled: !running},
		2:          {Divider: true},
		menuDelete: {Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
	}
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if index, ok := p.header.ActionClicked(); ok && index < len(actions) {
		switch actions[index] {
		case actionScreen:
			p.showSide(e, sideScreen)
		case actionChanges:
			p.showSide(e, sideChanges)
		case actionTerminal:
			e.OpenTerminal(folder)
		case actionFolder:
			e.OpenFolder(folder)
		}
	}
	if _, ok := p.header.LinkClicked(); ok && info.Project != "" {
		e.NavigateWork(info.Project, env.ViewOverview, info.Worktree)
	}
	if item, ok := p.header.MenuClicked(); ok {
		switch item {
		case menuRename:
			p.rename.open(info)
		case menuStop:
			stopSession(e, info.ID)
		case menuDelete:
			p.remove.open(info.ID)
		}
	}
	return dimensions
}

func sessionFacts(snapshot state.Snapshot, info protocol.SessionInfo) []look.HeaderFact {
	facts := make([]look.HeaderFact, 0, 4)
	if project := projectName(snapshot, info.Project); project != "" {
		facts = append(facts, look.HeaderFact{Text: project, Kind: look.FactLink})
		if worktree, ok := worktreeOf(snapshot, info); ok && !worktree.Main {
			facts = append(facts, look.HeaderFact{Text: worktree.Name, Kind: look.FactLink, Crumb: true})
		}
	} else if place := sessionPlace(snapshot, info); place != "" {
		facts = append(facts, look.HeaderFact{Text: place})
	}
	if info.Branch != "" {
		facts = append(facts, look.HeaderFact{Text: info.Branch, Kind: look.FactToken})
	}
	if harness := harnessName(snapshot, info.Harness); harness != "" {
		facts = append(facts, look.HeaderFact{Text: harness})
	}
	return facts
}

func (p *Page) layoutConversation(gtx layout.Context, e *env.Env, attached state.Attached) layout.Dimensions {
	size := gtx.Constraints.Max
	inset := gtx.Dp(look.ContentInset(e.Class))
	width := min(size.X-2*inset, gtx.Dp(look.ReadingColumnWidth))

	record := op.Record(gtx.Ops)
	composer := p.layoutComposer(gtx, e, attached, width)
	call := record.Stop()
	y := size.Y - gtx.Dp(21) - composer.Size.Y

	// the goal stands on the composer, both outside the scrolling transcript
	top := y
	record = op.Record(gtx.Ops)
	stripGtx := gtx
	stripGtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, size.Y)}
	strip := p.goal.layout(stripGtx, e, attached.Info)
	stripCall := record.Stop()
	if strip.Size.Y > 0 {
		top = y - gtx.Dp(8) - strip.Size.Y
	}

	view := gtx
	view.Constraints = layout.Exact(image.Pt(size.X, max(0, top-gtx.Dp(8))))
	p.view.Layout(view, e.Theme, e.Overlays, attached.Model)
	p.handleActions(gtx, e, attached.Info, p.view.Actions())

	if strip.Size.Y > 0 {
		offset := op.Offset(image.Pt((size.X-width)/2, top)).Push(gtx.Ops)
		stripCall.Add(gtx.Ops)
		offset.Pop()
	}
	offset := op.Offset(image.Pt((size.X-width)/2, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

func (p *Page) layoutComposer(gtx layout.Context, e *env.Env, attached state.Attached, width int) layout.Dimensions {
	info := attached.Info
	busy := attached.Model.Busy()
	p.controls.sync(attached)
	p.controls.wake(e, info)
	options := chat.ComposerOptions{
		Placeholder: i18n.T("Tell the agent what to do"), Busy: busy, Queue: info.Queue, CanAttach: true,
		History: attached.Model.Prompts, Commands: func() []protocol.Command { return sessionCommands(attached.Model.Commands()) },
		Controls: p.controls.widgets(e, e.Snapshot(), info, p.composer.FocusTag()),
	}
	options.Answer = p.view.Asking(attached.Model)
	e.ComposerDesktop(&p.composer, &options)
	p.controls.update(e, info.ID)
	gtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	dimensions := p.composer.Layout(gtx, e.Theme, options)
	if submission, ok := p.composer.Submitted(); ok {
		e.Submit(info.ID, busy, submission)
	}
	if p.composer.StopClicked() {
		e.Call(protocol.MSessionCancel, protocol.SessionRef{ID: info.ID}, new(protocol.Empty), report(e))
	}
	env.HandleChatActions(gtx, e, info.ID, env.Answered(gtx, &p.view, attached.Model, p.composer.Actions()))
	return dimensions
}

func (p *Page) handleActions(gtx layout.Context, e *env.Env, info protocol.SessionInfo, actions []chat.Action) {
	var common []chat.Action
	for _, action := range actions {
		switch action.Kind {
		case chat.ActFork:
			var forked protocol.SessionInfo
			e.Call(protocol.MSessionFork, protocol.ForkParams{ID: info.ID, Seq: action.Seq}, &forked, func(err error) {
				if err != nil {
					e.Notify(err.Error())
					return
				}
				open(e, forked.ID)
			})
		case chat.ActTurnChanges:
			p.panel.showTurn(action.Turn)
			if !e.PanelOpen {
				e.TogglePanel()
			}
		case chat.ActRevertTurn:
			p.revert.open(info.ID, action.Turn)
		case chat.ActQuote:
			p.composer.Quote(action.Value)
		default:
			common = append(common, action)
		}
	}
	// answering a request or copying is not a reason to click the field again before the next message
	if len(common) > 0 {
		p.composer.Focus()
	}
	env.HandleChatActions(gtx, e, info.ID, common)
}
