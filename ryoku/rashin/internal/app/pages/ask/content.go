package ask

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const homeColumn = 680

// an ask that was just made isn't lost, the session list arrives after the answer that made it
func (p *Page) lost(known bool) bool {
	if known {
		p.starting = false
		return false
	}
	return p.selected != "" && !p.starting
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.loadModels(e)
	snapshot := e.Snapshot()
	info, known := snapshot.Sessions[p.selected]
	if p.lost(known) {
		p.selected = ""
		e.Navigate(protocol.TabAsk, "")
	}
	attached, open := snapshot.Attached[p.selected]
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info, known) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutBody(gtx, e, attached, open) }),
	)
}

func (p *Page) layoutHeader(gtx layout.Context, e *env.Env, info protocol.SessionInfo, known bool) layout.Dimensions {
	content := look.PaneHeaderContent{Title: i18n.T("New ask"), Scrolled: known && p.view.Scrolled()}
	if known {
		content.Title = sessionTitle(info)
		if origin := originLabel(info.Origin); origin != "" {
			content.Facts = []look.HeaderFact{{Text: origin}}
		}
		content.Actions = []look.HeaderAction{{Icon: look.Icon.Agents, Description: i18n.T("Move to Agent")}}
		content.Menu = []kit.MenuItem{
			{Icon: look.Icon.Rename, Label: i18n.T("Rename")},
			{Divider: true},
			{Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
		}
	}
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := p.header.ActionClicked(); ok && known {
		p.handoff.open(e, info.ID, "")
	}
	if item, ok := p.header.MenuClicked(); ok && known {
		switch item {
		case 0:
			p.rename.open(info)
		case 2:
			p.remove.open(info.ID)
		}
	}
	return dimensions
}

func (p *Page) layoutBody(gtx layout.Context, e *env.Env, attached state.Attached, open bool) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	inset := gtx.Dp(look.ContentInset(e.Class))
	column := min(size.X-2*inset, gtx.Dp(look.ReadingColumnWidth))
	home := min(size.X-2*inset, gtx.Dp(homeColumn))
	travel := look.Transition(gtx, &p.travel, p.selected != "" || p.starting, th.Motion.Select, th.Motion.Select)

	width := home + int(float32(column-home)*travel+.5)
	record := op.Record(gtx.Ops)
	composer := p.layoutComposer(gtx, e, attached, open, width)
	composerCall := record.Stop()

	foot := size.Y - gtx.Dp(21) - composer.Size.Y
	lead := gtx.Sp(th.T.Sentence.Line) + gtx.Dp(8) + gtx.Sp(th.T.Proof.Line) + gtx.Dp(21)
	centre := min(max(size.Y*42/100, lead+gtx.Dp(21)), max(lead, foot))
	y := centre + int(float32(foot-centre)*travel+.5)
	x := (size.X - width) / 2

	if open {
		p.layoutTranscript(gtx, e, attached, image.Pt(size.X, max(0, y-gtx.Dp(8))), travel)
	}
	if travel < 1 {
		p.layoutHome(gtx, e, image.Rect(x, y, x+width, y+composer.Size.Y), 1-travel)
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	composerCall.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

func (p *Page) layoutTranscript(gtx layout.Context, e *env.Env, attached state.Attached, size image.Point, travel float32) {
	opacity := paint.PushOpacity(gtx.Ops, travel)
	defer opacity.Pop()
	view := gtx
	view.Constraints = layout.Exact(size)
	p.rise.Layout(view, e.Theme, p.selected, 8, func(gtx layout.Context) layout.Dimensions {
		return p.view.Layout(gtx, e.Theme, e.Overlays, attached.Model)
	})
	p.handleActions(gtx, e, p.view.Actions())
}

func (p *Page) layoutHome(gtx layout.Context, e *env.Env, composer image.Rectangle, alpha float32) {
	th := e.Theme
	opacity := paint.PushOpacity(gtx.Ops, alpha)
	defer opacity.Pop()
	if alpha < 1 {
		gtx = gtx.Disabled()
	}
	proof, sentence := gtx.Sp(th.T.Proof.Line), gtx.Sp(th.T.Sentence.Line)
	proofTop := composer.Min.Y - gtx.Dp(21) - proof
	sentenceTop := proofTop - gtx.Dp(8) - sentence
	look.Place(gtx, image.Rect(composer.Min.X, sentenceTop, composer.Max.X, sentenceTop+sentence),
		kit.Text(th.Theme, th.T.Sentence, th.R.Fg, i18n.T("Ask about this machine")))
	look.Place(gtx, image.Rect(composer.Min.X, proofTop, composer.Max.X, proofTop+proof),
		kit.Text(th.Theme, th.T.Proof, th.R.Fg2, i18n.T("Rashin reads your system to answer. It never changes it.")))

	texts := suggestionTexts()
	offset := op.Offset(image.Pt(composer.Min.X, composer.Max.Y+gtx.Dp(13))).Push(gtx.Ops)
	row := gtx
	row.Constraints = layout.Constraints{Max: image.Pt(composer.Dx(), gtx.Constraints.Max.Y)}
	tries := look.Flow{Gap: 8, LineGap: 8}.Layout(row, len(texts), func(gtx layout.Context, index int) layout.Dimensions {
		plaque := &p.suggestions[index]
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueQuiet, true, texts[index]
		if plaque.Clicked(gtx) {
			p.composer.SetText(texts[index])
			p.composer.Focus()
		}
		return plaque.Layout(gtx, th.Theme)
	})
	offset.Pop()
	offset = op.Offset(image.Pt(composer.Min.X, composer.Max.Y+gtx.Dp(13)+tries.Size.Y+gtx.Dp(21))).Push(gtx.Ops)
	p.ready.layout(row, e)
	offset.Pop()
}

func suggestionTexts() [4]string {
	return [4]string{
		i18n.T("What is using my disk?"),
		i18n.T("Why did the last update fail?"),
		i18n.T("Which apps start at login?"),
		i18n.T("How do I change the bar style?"),
	}
}

func (p *Page) layoutComposer(gtx layout.Context, e *env.Env, attached state.Attached, open bool, width int) layout.Dimensions {
	options := chat.ComposerOptions{Placeholder: i18n.T("Ask anything about this machine"), CanAttach: true}
	e.ComposerDesktop(&p.composer, &options)
	current := p.draftRoute
	if open {
		info := attached.Info
		options.Placeholder = i18n.T("Ask a follow-up")
		options.Busy, options.Queue = attached.Model.Busy(), info.Queue
		options.History = attached.Model.Prompts
		options.Answer = p.view.Asking(attached.Model)
		current = info.Model
	}
	labels, ids, selected := p.routeOptions(current)
	p.route.Form, p.route.Label, p.route.Return = look.SelectQuiet, i18n.T("Model"), p.composer.FocusTag()
	options.Controls = []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return p.route.Layout(gtx, e.Theme, e.Overlays, labels, selected)
	}}
	if index, changed := p.route.Changed(); changed && index >= 0 && index < len(ids) {
		if open {
			params := protocol.SetModelParams{ID: p.selected, Model: ids[index]}
			e.Call(protocol.MSessionSetModel, params, new(protocol.Empty), nil)
		} else {
			p.draftRoute = ids[index]
		}
	}
	gtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
	dimensions := p.composer.Layout(gtx, e.Theme, options)
	if submission, submitted := p.composer.Submitted(); submitted {
		switch {
		case p.selected != "":
			e.Submit(p.selected, options.Busy, submission)
		case !p.starting:
			p.start(e, submission)
		}
	}
	if p.composer.StopClicked() && p.selected != "" {
		e.Call(protocol.MSessionCancel, protocol.SessionRef{ID: p.selected}, new(protocol.Empty), nil)
	}
	actions := p.composer.Actions()
	if open {
		actions = env.Answered(gtx, &p.view, attached.Model, actions)
	}
	env.HandleChatActions(gtx, e, p.selected, actions)
	return dimensions
}

func (p *Page) handleActions(gtx layout.Context, e *env.Env, actions []chat.Action) {
	var common []chat.Action
	for _, action := range actions {
		switch action.Kind {
		case chat.ActHandoff:
			p.handoff.open(e, p.selected, action.Value)
		case chat.ActQuote:
			p.composer.Quote(action.Value)
		default:
			common = append(common, action)
		}
	}
	// copying or retrying is not a reason to click the field again before the next question
	if len(common) > 0 {
		p.composer.Focus()
	}
	env.HandleChatActions(gtx, e, p.selected, common)
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
		return ""
	}
}
