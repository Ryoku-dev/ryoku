package agent

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
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
	actions := make([]int, 0, 3)
	if tracked {
		change := look.HeaderAction{Icon: look.Icon.Diff, Label: i18n.T("Changes"), Description: i18n.T("Show what changed") + "  Ctrl \\", On: e.PanelOpen}
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
		case actionChanges:
			e.TogglePanel()
		case actionTerminal:
			e.OpenTerminal(folder)
		case actionFolder:
			e.Open(folder)
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

	view := gtx
	view.Constraints = layout.Exact(image.Pt(size.X, max(0, y-gtx.Dp(8))))
	p.view.Layout(view, e.Theme, e.Overlays, attached.Model)
	p.handleActions(gtx, e, attached.Info, p.view.Actions())

	offset := op.Offset(image.Pt((size.X-width)/2, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

func (p *Page) layoutComposer(gtx layout.Context, e *env.Env, attached state.Attached, width int) layout.Dimensions {
	info := attached.Info
	busy := attached.Model.Busy()
	modelLabels, modelIDs, selectedModel := choiceOptions(attached.Model.ModelChoice(), info.Model)
	modeLabels, modeIDs, selectedMode := choiceOptions(attached.Model.ModeChoice(), info.Mode)
	p.model.Form, p.model.Label, p.model.Return = look.SelectQuiet, i18n.T("Model"), p.composer.FocusTag()
	p.mode.Form, p.mode.Label, p.mode.Return = look.SelectQuiet, i18n.T("Mode"), p.composer.FocusTag()
	options := chat.ComposerOptions{
		Placeholder: i18n.T("Tell the agent what to do"), Busy: busy, Queue: info.Queue, CanAttach: true,
		History: attached.Model.Prompts, Commands: attached.Model.Commands,
		Controls: []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				return p.model.Layout(gtx, e.Theme, e.Overlays, modelLabels, selectedModel)
			},
			func(gtx layout.Context) layout.Dimensions {
				return p.mode.Layout(gtx, e.Theme, e.Overlays, modeLabels, selectedMode)
			},
		},
	}
	options.Answer = p.view.Asking(attached.Model)
	e.ComposerDesktop(&p.composer, &options)
	if index, changed := p.model.Changed(); changed && index < len(modelIDs) {
		e.Call(protocol.MSessionSetModel, protocol.SetModelParams{ID: info.ID, Model: modelIDs[index]}, new(protocol.Empty), report(e))
	}
	if index, changed := p.mode.Changed(); changed && index < len(modeIDs) {
		e.Call(protocol.MSessionSetMode, protocol.SetModeParams{ID: info.ID, Mode: modeIDs[index]}, new(protocol.Empty), report(e))
	}
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
