package ask

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

const homeColumn = 680

// the home's chart reads the router's last two weeks
const answerDays = 14

// the composer rises to give the chart this much line, and the chart is left out below the least
const (
	trendRoom  unit.Dp = 72
	trendLeast unit.Dp = 48
)

// an ask that was just made isn't lost, the session list arrives after the answer that made it
func (p *Page) lost(known bool) bool {
	if known {
		p.starting = false
		return false
	}
	return p.selected != "" && !p.starting
}

func (p *Page) Content(gtx layout.Context, e *env.Env) layout.Dimensions {
	snapshot := e.Snapshot()
	p.menu.Load(e, snapshot)
	info, known := snapshot.Sessions[p.selected]
	if p.lost(known) {
		p.selected = ""
		e.Navigate(protocol.TabAsk, "")
	}
	attached, open := snapshot.Attached[p.selected]
	if p.temporary.Pressed(gtx) {
		p.pressTemporary(e)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, e, info, known) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutBody(gtx, e, attached, open) }),
	)
}

func (p *Page) layoutHeader(gtx layout.Context, e *env.Env, info protocol.SessionInfo, known bool) layout.Dimensions {
	content := look.PaneHeaderContent{Title: i18n.T("New ask"), Scrolled: known && p.view.Scrolled()}
	switch {
	case known && info.Temporary:
		content.Title = sessionTitle(info)
		content.Facts = []look.HeaderFact{env.TemporaryMark()}
		content.Actions = []look.HeaderAction{{Icon: look.Icon.Close, Description: i18n.T("Close this conversation")}}
		content.Menu = []kit.MenuItem{{Icon: look.Icon.Check, Label: i18n.T("Keep this conversation")}}
	case known:
		content.Title = sessionTitle(info)
		if origin := originLabel(info.Origin); origin != "" {
			content.Facts = []look.HeaderFact{{Text: origin}}
		}
		if info.Memory == protocol.MemoryNone {
			content.Facts = append(content.Facts, look.HeaderFact{Text: i18n.T("No memory")})
		}
		content.Actions = []look.HeaderAction{{Icon: look.Icon.Agents, Description: i18n.T("Move to Agent")}}
		content.Menu = []kit.MenuItem{
			{Icon: look.Icon.Rename, Label: i18n.T("Rename")},
			{Divider: true},
			{Icon: look.Icon.Delete, Label: i18n.T("Delete"), Danger: true},
		}
	default:
		if p.temporary.On {
			content.Facts = []look.HeaderFact{env.TemporaryMark()}
		}
		content.Actions = []look.HeaderAction{p.temporary.Action()}
	}
	dimensions := p.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := p.header.ActionClicked(); ok {
		switch {
		case known && info.Temporary:
			e.Navigate(protocol.TabAsk, "")
		case known:
			p.handoff.open(e, info.ID, "")
		default:
			p.pressTemporary(e)
		}
	}
	if item, ok := p.header.MenuClicked(); ok && known {
		switch {
		case info.Temporary:
			p.keep.open(info)
		case item == 0:
			p.rename.open(info)
		case item == 2:
			p.remove.open(e, info.ID)
		}
	}
	return dimensions
}

// the switch stands on a new conversation; from an open one it starts a new temporary one
func (p *Page) pressTemporary(e *env.Env) {
	p.composer.Focus()
	if p.selected == "" && !p.starting {
		p.temporary.On = !p.temporary.On
		return
	}
	p.temporary.On = true
	e.Navigate(protocol.TabAsk, "")
}

func (p *Page) layoutBody(gtx layout.Context, e *env.Env, attached state.Attached, open bool) layout.Dimensions {
	th := e.Theme
	size := gtx.Constraints.Max
	inset := gtx.Dp(look.ContentInset(e.Class))
	column := min(size.X-2*inset, gtx.Dp(look.ReadingColumnWidth))
	home := min(size.X-2*inset, gtx.Dp(homeColumn))
	travel := look.Transition(gtx, &p.travel, p.selected != "" || p.starting, th.Motion.Select, th.Motion.Select)
	if p.selected == "" && !p.starting {
		p.usage.Load(e, answerDays, gtx.Now)
	} else {
		p.usage.Forget()
	}

	width := home + int(float32(column-home)*travel+.5)
	record := op.Record(gtx.Ops)
	composer := p.layoutComposer(gtx, e, attached, open, width)
	composerCall := record.Stop()
	var tail layout.Dimensions
	var tailCall op.CallOp
	if travel < 1 {
		faded := gtx
		if travel > 0 {
			faded = gtx.Disabled()
		}
		record = op.Record(gtx.Ops)
		tail = p.layoutTail(faded, e, width)
		tailCall = record.Stop()
	}

	foot := size.Y - gtx.Dp(21) - composer.Size.Y
	lead := gtx.Sp(th.T.Sentence.Line) + gtx.Dp(8) + gtx.Sp(th.T.Proof.Line) + gtx.Dp(21)
	// what lies under the composer before the chart's line: the suggestions, the readiness and the chart's own words
	under := composer.Size.Y + gtx.Dp(13) + tail.Size.Y + gtx.Dp(th.Sp.S5) + trendFrame(gtx, th)
	centre := max(size.Y*42/100, lead+gtx.Dp(21))
	// the chart gets its room by raising the composer, never by pushing what is under it off the pane
	centre = max(min(centre, size.Y-under-gtx.Dp(trendRoom)), lead+gtx.Dp(21))
	centre = min(centre, max(lead, foot))
	plot := min(gtx.Dp(120), size.Y-centre-under)
	y := centre + int(float32(foot-centre)*travel+.5)
	x := (size.X - width) / 2

	if open {
		p.layoutTranscript(gtx, e, attached, image.Pt(size.X, max(0, y-gtx.Dp(8))), travel)
	}
	if travel < 1 {
		p.layoutHome(gtx, e, image.Rect(x, y, x+width, y+composer.Size.Y), 1-travel, tailCall, tail.Size.Y, plot)
	}
	offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	composerCall.Add(gtx.Ops)
	offset.Pop()
	return layout.Dimensions{Size: size}
}

// what a titled trend takes besides its line: the title, the proof, the ticks and the gap under it
func trendFrame(gtx layout.Context, th *look.Theme) int {
	return gtx.Sp(th.T.Section.Line) + gtx.Dp(tokens.SizeSlabTitleGap) + gtx.Sp(th.T.Detail.Line) + gtx.Dp(th.Sp.S3) +
		gtx.Dp(tokens.SizeChartTick) + gtx.Sp(th.T.MonoSm.Line) + gtx.Dp(tokens.SizeSlabBelow)
}

func (p *Page) layoutTranscript(gtx layout.Context, e *env.Env, attached state.Attached, size image.Point, travel float32) {
	opacity := paint.PushOpacity(gtx.Ops, travel)
	defer opacity.Pop()
	view := gtx
	view.Constraints = layout.Exact(size)
	p.lines.Sync(e, e.Snapshot(), &p.view, attached.Model)
	p.view.SetMemoryText(chat.MemoryText{Fact: memoryui.FactText, Plain: env.PlainMemory})
	p.rise.Layout(view, e.Theme, p.selected, 8, func(gtx layout.Context) layout.Dimensions {
		return p.view.Layout(gtx, e.Theme, e.Overlays, attached.Model)
	})
	p.handleActions(gtx, e, p.view.Actions())
}

func (p *Page) layoutHome(gtx layout.Context, e *env.Env, composer image.Rectangle, alpha float32, tail op.CallOp, tailHeight, plot int) {
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
	// the mode select in the composer carries the state, the sentence stays in one ink
	note := env.ApprovalsNote(env.AskApprovals, p.approvals.Value("", e.Snapshot().Settings.Ask.Approvals))
	if p.temporary.On {
		note = i18n.T("Nothing here is saved or remembered.")
	}
	look.Place(gtx, image.Rect(composer.Min.X, proofTop, composer.Max.X, proofTop+proof),
		kit.Text(th.Theme, th.T.Proof, th.R.Fg2, note))

	offset := op.Offset(image.Pt(composer.Min.X, composer.Max.Y+gtx.Dp(13))).Push(gtx.Ops)
	tail.Add(gtx.Ops)
	offset.Pop()

	trend, loaded := p.usage.Trend()
	if !loaded || len(trend.Values) == 0 || plot < gtx.Dp(trendLeast) {
		return
	}
	trend.Height = unit.Dp(float32(plot) / gtx.Metric.PxPerDp)
	top := composer.Max.Y + gtx.Dp(13) + tailHeight + gtx.Dp(th.Sp.S5)
	look.Place(gtx, image.Rect(composer.Min.X, top, composer.Max.X, top+plot+trendFrame(gtx, th)), func(gtx layout.Context) layout.Dimensions {
		return trend.Layout(gtx, th)
	})
}

// the suggestions, or once the first-run card in their place, and the readiness line, measured before the composer is placed so the chart's room is known
func (p *Page) layoutTail(gtx layout.Context, e *env.Env, width int) layout.Dimensions {
	th := e.Theme
	gtx.Constraints = layout.Constraints{Max: image.Pt(width, gtx.Constraints.Max.Y)}
	texts := suggestionTexts()
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if firstRunShown(e.Snapshot()) {
				return p.first.layout(gtx, e)
			}
			return look.Flow{Gap: 8, LineGap: 8}.Layout(gtx, len(texts), func(gtx layout.Context, index int) layout.Dimensions {
				plaque := &p.suggestions[index]
				plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, texts[index]
				if plaque.Clicked(gtx) {
					p.composer.SetText(texts[index])
					p.composer.Focus()
				}
				return plaque.Layout(gtx, th.Theme)
			})
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.ready.layout(gtx, e) }),
	)
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
	// with no conversation open the choice is what the next one starts with
	settings := e.Snapshot().Settings.Ask
	current, may := p.draftRoute, settings.Approvals
	remembers, temporary, locked := p.memory.draft, p.temporary.On, false
	if open {
		info := attached.Info
		options.Placeholder = i18n.T("Ask a follow-up")
		options.Busy, options.Queue = attached.Model.Busy(), info.Queue
		options.History = attached.Model.Prompts
		options.Answer = p.view.Asking(attached.Model)
		current, may = info.Model, info.Mode
		remembers, temporary, locked = info.Memory, info.Temporary, len(attached.Model.Prompts()) > 0
	}
	// the field itself says what a temporary conversation is, where it always fits
	if temporary {
		options.Placeholder = env.TemporaryHint()
	}
	snapshot := e.Snapshot()
	harness := answeringHarness(snapshot, attached, open)
	var model layout.Widget
	var ids []string
	if harness == "" {
		// nothing picked is what the daemon falls back to: the model Settings names
		if current == "" {
			current = settings.Model
		}
		routes, routeIDs, selected := p.menu.Choices(current)
		ids = routeIDs
		p.route.Form, p.route.Label, p.route.Return, p.route.Searched = look.SelectQuiet, i18n.T("Model"), p.composer.FocusTag(), true
		model = func(gtx layout.Context) layout.Dimensions {
			dimensions := p.route.LayoutOptions(gtx, e.Theme, e.Overlays, routes, selected)
			if p.route.Opened(e.Overlays) {
				p.limits.Show(e, gtx.Now)
			}
			return dimensions
		}
	} else {
		p.engine.sync(snapshot, attached, open)
		model = p.engine.control(e, p.composer.FocusTag())
	}
	options.Controls = nil
	if model != nil {
		options.Controls = append(options.Controls, model)
	}
	options.Controls = append(options.Controls, func(gtx layout.Context) layout.Dimensions {
		return p.approvals.Layout(gtx, e, env.AskApprovals, p.selected, may, p.composer.FocusTag())
	})
	if memoryOn(e.Snapshot()) {
		options.Controls = append(options.Controls, func(gtx layout.Context) layout.Dimensions {
			return p.memory.layout(gtx, e, remembers, temporary, locked, p.composer.FocusTag())
		})
	}
	options.Controls = append(options.Controls, engineLabel(e, env.AskEngineName(snapshot, protocol.SessionInfo{Harness: harness})))
	if value, changed := p.memory.Changed(); changed {
		p.memory.set(e, p.selected, value)
	}
	if value, changed := p.approvals.Changed(); changed {
		if open {
			e.Call(protocol.MSessionSetMode, protocol.SetModeParams{ID: p.selected, Mode: value}, new(protocol.Empty), func(err error) {
				if err != nil {
					e.Notify(err.Error())
				}
			})
		}
		e.SetApprovals(env.AskApprovals, value)
	}
	if index, changed := p.route.Changed(); changed && index >= 0 && index < len(ids) {
		if open {
			params := protocol.SetModelParams{ID: p.selected, Model: ids[index]}
			e.Call(protocol.MSessionSetModel, params, new(protocol.Empty), nil)
		} else {
			p.draftRoute = ids[index]
		}
	}
	p.engine.update(e, p.selected, open)
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
