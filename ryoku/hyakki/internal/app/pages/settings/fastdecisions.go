package settings

import (
	"encoding/json"
	"strings"

	"gioui.org/layout"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	i18n "ryoku-i18n"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type decisionFields struct {
	address, model string
	budget         int
}

type fastDecisionPanel struct {
	route                                look.Select
	allow                                kit.Toggle
	address, model                       kit.TextField
	budget                               kit.Stepper
	save, reload, checkLocal, checkEmber kit.Plaque
	filled, testing                      bool
	stored                               decisionFields
	request                              uint64
	result, problem                      string
	reviewedAccount                      string
}

func (p *fastDecisionPanel) fill(fields decisionFields) {
	p.address.SetText(fields.address)
	p.model.SetText(fields.model)
	p.budget.Value = fields.budget
	p.stored, p.filled = fields, true
}

func storedDecisionFields(cfg protocol.DecisionRoutingSettings) decisionFields {
	fields := decisionFields{cfg.LocalAddress, cfg.LocalModel, cfg.BudgetMS}
	if fields.address == "" {
		fields.address = protocol.DefaultDecisionAddress
	}
	if fields.budget == 0 {
		fields.budget = protocol.DefaultDecisionBudgetMS
	}
	return fields
}

func (p *fastDecisionPanel) blocks(e *env.Env, blocks []layout.Widget) []layout.Widget {
	th := e.Theme
	if !p.filled {
		p.fill(storedDecisionFields(e.Snapshot().Settings.Providers.Decisions))
	}
	return append(blocks, func(gtx layout.Context) layout.Dimensions {
		cfg := e.Snapshot().Settings.Providers.Decisions
		if cfg.Route == "" {
			cfg.Route = protocol.DecisionRules
		}
		current := storedDecisionFields(cfg)
		edited := decisionFields{strings.TrimSpace(p.address.Text()), strings.TrimSpace(p.model.Text()), p.budget.Value}
		changed := edited != p.stored
		stale := current != p.stored
		if stale && !changed && !p.save.Busy {
			p.fill(current)
			edited, stale = current, false
		}
		p.address.Editor.SingleLine, p.model.Editor.SingleLine = true, true
		p.address.Mono, p.model.Mono = true, true
		p.address.Placeholder = protocol.DefaultDecisionAddress
		p.model.Placeholder = i18n.T("An installed native decision model")
		p.budget.Min, p.budget.Max, p.budget.Step, p.budget.Unit = 100, 5000, 100, i18n.T("ms")
		p.allow.Value = cfg.AllowHosted
		values := [...]string{protocol.DecisionRules, protocol.DecisionLocal, protocol.DecisionAuto, protocol.DecisionEmber}
		options := []look.SelectOption{
			{Label: i18n.T("Built-in rules"), Detail: i18n.T("No model, account or credits")},
			{Label: i18n.T("Local decision model"), Detail: i18n.T("Loopback only; nothing leaves this computer")},
			{Label: i18n.T("Automatic local-first"), Detail: i18n.T("Ember is excluded unless separately allowed below")},
			{Label: i18n.T("Ryoku Ember"), Detail: i18n.T("Sends state and typed questions off-device; spends finite Ember credits")},
		}
		rows := []look.Setting{
			{Title: i18n.T("Decision route"), Detail: i18n.T("Accelerates typed choices, not permission. A failed classifier falls back to rules or clarification."), Control: func(gtx layout.Context) layout.Dimensions {
				selected := 0
				for index, value := range values {
					if value == cfg.Route {
						selected = index
					}
				}
				p.route.Label = i18n.T("Decision route")
				dimensions := p.route.LayoutOptions(gtx, th, e.Overlays, options, selected)
				if index, ok := p.route.Changed(); ok && index >= 0 && index < len(values) {
					e.SetSettings(map[string]any{"providers": map[string]any{"decisions": map[string]any{"route": values[index]}}}, e.Report)
				}
				return dimensions
			}},
			{Title: i18n.T("Allow automatic decisions to use Ember credits"), Detail: i18n.T("Off by default. Allowing this sends decision state and questions to Ryoku Ember when the local route cannot answer. It never grants tools or actions."), Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Allow automatic decisions to use Ember credits"), &p.allow, func(on bool) {
					e.SetSettings(map[string]any{"providers": map[string]any{"decisions": map[string]any{"allowHosted": on}}}, e.Report)
				})
			}},
			{Title: i18n.T("Local decision address"), Detail: i18n.T("Only this computer's loopback addresses are accepted. Proxies and redirects cannot carry local data elsewhere."), Control: func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, i18n.T("Local decision address"), func(gtx layout.Context) layout.Dimensions { return p.address.Layout(gtx, th.Theme) })
			}},
			{Title: i18n.T("Local decision model"), Detail: i18n.T("Install and choose a native decision model explicitly. This does not download or start another service."), Control: func(gtx layout.Context) layout.Dimensions {
				return look.Named(gtx, i18n.T("Local decision model"), func(gtx layout.Context) layout.Dimensions { return p.model.Layout(gtx, th.Theme) })
			}},
			{Title: i18n.T("Decision time budget"), Detail: i18n.T("The total time across routes. A timeout asks or uses rules; it never silently selects a paid route."), Control: func(gtx layout.Context) layout.Dimensions { return p.budget.Layout(gtx, th.Theme) }},
		}
		save := func(gtx layout.Context) layout.Dimensions {
			if p.save.Clicked(gtx) && changed && !stale && !p.save.Busy {
				p.save.Busy, p.problem = true, ""
				e.SetSettings(map[string]any{"providers": map[string]any{"decisions": map[string]any{"localAddress": edited.address, "localModel": edited.model, "budgetMs": edited.budget}}}, func(err error) {
					p.save.Busy = false
					if err != nil {
						p.problem = err.Error()
						return
					}
					p.filled = false
				})
			}
			return groupSave(th, &p.save, changed && !stale)(gtx)
		}
		return layout.Inset{Top: th.Sp.S5}.Layout(gtx, group(th, i18n.T("Fast decisions"), i18n.T("Optional native classifiers share one typed contract. Built-in rules stay available without a model or account. Confidence is not proof of calibration and never authorizes work."), rows, save))
	}, func(gtx layout.Context) layout.Dimensions {
		cfg := e.Snapshot().Settings.Providers.Decisions
		current := storedDecisionFields(cfg)
		changed := decisionFields{strings.TrimSpace(p.address.Text()), strings.TrimSpace(p.model.Text()), p.budget.Value} != p.stored
		stale := current != p.stored
		p.reload.Kind, p.reload.Small, p.reload.Label = kit.PlaqueQuiet, true, i18n.T("Reload decision fields and discard edits")
		if p.reload.Clicked(gtx) {
			p.fill(current)
			p.problem = ""
			changed, stale = false, false
		}
		p.checkLocal.Kind, p.checkLocal.Small, p.checkLocal.Label = kit.PlaqueTonal, true, i18n.T("Check local decision model")
		p.checkEmber.Kind, p.checkEmber.Small, p.checkEmber.Label = kit.PlaqueTonal, true, i18n.T("Use Ember credits for a check")
		currentAccount := ""
		if identity := e.Snapshot().Account.Account; identity != nil {
			currentAccount = identity.ID
		}
		sameAccount := currentAccount == p.reviewedAccount
		p.reviewedAccount = currentAccount
		localClicked, emberClicked := p.checkLocal.Clicked(gtx), p.checkEmber.Clicked(gtx)
		if !changed && !stale && !p.testing {
			if localClicked && cfg.LocalModel != "" {
				p.check(e, protocol.DecisionLocal)
			}
			if emberClicked && sameAccount && e.Snapshot().Account.State == protocol.AccountSignedIn {
				p.check(e, protocol.DecisionEmber)
			}
		}
		widgets := []layout.FlexChild{
			layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, i18n.T("The check sends only this fixed text: ‘Read this note without changing anything.’ Its two choices are read and stop. No screen, files or conversation are included. The Ember button explicitly sends that fixture off-device and may spend Ember credits."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if changed || stale || p.testing || cfg.LocalModel == "" {
							gtx = gtx.Disabled()
						}
						return p.checkLocal.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if changed || stale || p.testing || e.Snapshot().Account.State != protocol.AccountSignedIn {
							gtx = gtx.Disabled()
						}
						return p.checkEmber.Layout(gtx, th.Theme)
					}))
			}),
		}
		if stale {
			widgets = append(widgets, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Saved decision fields changed while you edited. Your draft is kept; reload deliberately before saving or checking."))), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.reload.Layout(gtx, th.Theme) }))
		} else if changed {
			widgets = append(widgets, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Save the fields before checking the configured route."))))
		}
		if p.testing {
			widgets = append(widgets, layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg2, i18n.T("Waiting for the bounded classifier request…"))))
		}
		if p.problem != "" {
			widgets = append(widgets, layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, p.problem)))
		}
		if p.result != "" {
			widgets = append(widgets, layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg2, p.result)))
		}
		return layout.Inset{Top: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, widgets...)
		})
	})
}

func (p *fastDecisionPanel) check(e *env.Env, route string) {
	p.request++
	request := p.request
	account := ""
	if identity := e.Snapshot().Account.Account; identity != nil {
		account = identity.ID
	}
	p.testing, p.problem, p.result = true, "", ""
	params := protocol.FastDecisionParams{Route: route, AccountID: account, Feature: "settings.check", DecisionRequest: protocol.DecisionRequest{
		State:     json.RawMessage(`"Read this note without changing anything."`),
		Questions: map[string]protocol.DecisionQuestion{"intent": {Type: "choice", Criteria: json.RawMessage(`{"read":"Read it without changing anything","stop":"Do not proceed"}`)}},
	}}
	var result protocol.FastDecisionResult
	e.Call(protocol.MFastDecide, params, &result, func(err error) {
		if request != p.request {
			return
		}
		p.testing = false
		current := ""
		if identity := e.Snapshot().Account.Account; identity != nil {
			current = identity.ID
		}
		if route == protocol.DecisionEmber && current != account {
			p.problem = i18n.T("The signed-in Account changed; the previous check result was discarded.")
			return
		}
		if err != nil {
			p.problem = err.Error()
			return
		}
		p.result = result.Notice
		if result.Resolved && result.Decision != nil {
			p.result = i18n.Tf("Route: %s. Choice: %s. Input: %d tokens. Elapsed: %d ms. %s", result.Route, result.Decision.Answers["intent"].Choice, result.Decision.Usage.Input, result.ElapsedMS, result.Notice)
		}
	})
}
