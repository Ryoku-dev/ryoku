package settings

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func (p *Page) layoutRashin(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	status := snapshot.Status
	p.turnOff.Kind, p.turnOff.Small, p.turnOff.Label = kit.PlaqueTonal, true, i18n.T("Turn off")
	if p.turnOff.Clicked(gtx) {
		p.AskTurnOff()
	}
	p.log.Kind, p.log.Small, p.log.Label = kit.PlaqueTonal, true, i18n.T("Open log")
	if p.log.Clicked(gtx) {
		e.Open(paths.DaemonLog())
	}
	version := status.Version
	if version == "" {
		version = i18n.T("unknown")
	}
	rows := []look.Setting{
		{
			Title:   i18n.T("Running"),
			Detail:  i18n.T("While Hyakki is off nothing runs, nothing is indexed and it uses no memory."),
			Control: func(gtx layout.Context) layout.Dimensions { return p.turnOff.Layout(gtx, th.Theme) },
		},
		{
			Title:   i18n.Tf("Version %s", version),
			Detail:  i18n.Tf("The daemon writes what it does to %s.", paths.Short(paths.DaemonLog())),
			Control: func(gtx layout.Context) layout.Dimensions { return p.log.Layout(gtx, th.Theme) },
		},
		{
			Title: i18n.T("A tour of Hyakki"), Detail: i18n.T("Explore local models, Oni and what stays on this computer. Every optional feature stays off unless you choose it."),
			Control: func(gtx layout.Context) layout.Dimensions {
				p.tour.Kind, p.tour.Small, p.tour.Label = kit.PlaqueTonal, true, i18n.T("Re-run the tour")
				if p.tour.Clicked(gtx) && e.OpenTour != nil {
					e.OpenTour()
				}
				if e.OpenTour == nil {
					gtx = gtx.Disabled()
				}
				return p.tour.Layout(gtx, th.Theme)
			},
		},
	}
	return look.Section(gtx, th, ordinal(sectionRashin), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, i18n.T("Hyakki is on."), rashinProof(snapshot))
			}),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.featuresOff(gtx, e) }),
		)
	})
}

// the proof counts what the daemon reports, so the claim never outruns it
func rashinProof(snapshot state.Snapshot) string {
	ready := 0
	for _, provider := range snapshot.Providers {
		if provider.Enabled && provider.Available > 0 {
			ready++
		}
	}
	providers := i18n.Tf("%d providers are ready.", ready)
	switch ready {
	case 0:
		providers = i18n.T("No provider is ready yet.")
	case 1:
		providers = i18n.T("1 provider is ready.")
	}
	api := i18n.T("The local API is off.")
	if snapshot.Settings.API.Enabled {
		api = i18n.T("The local API is not answering.")
		if snapshot.Status.API.Listening {
			api = i18n.T("The local API answers.")
		}
	}
	return providers + " " + api
}

func (p *Page) layoutAsk(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	settings := e.Snapshot().Settings.Ask
	stock := p.defaults.Ask
	// filled again only when the stored model moved, so a pick stands until the daemon has it
	if !p.askFilled || settings.Model != p.askStored {
		p.askModel.Set(settings.Model, nil)
		p.askStored, p.askFilled = settings.Model, true
	}
	p.habits.Value = settings.Habits
	mayIDs := [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	mayLabels := []string{i18n.T("Never"), i18n.T("After asking you"), i18n.T("Without asking (YOLO)")}
	selectedMay := 0
	for index, id := range mayIDs {
		if id == settings.Approvals {
			selectedMay = index
		}
	}
	return look.SettingsSection(gtx, th, ordinal(sectionAsk), []look.Setting{
		{
			Title:   i18n.T("Ask answers with"),
			Detail:  i18n.T("Auto tries a model on this computer first, then a free provider. Name a model to use only that one; paid models run only when you pick them."),
			Below:   true,
			Changed: p.changed.askModel,
			Reset:   func() { set(e, "ask", map[string]any{"model": stock.Model}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := p.askModel.Layout(gtx, e)
				if _, picked := p.askModel.Changed(); picked {
					model, _ := p.askModel.Value()
					set(e, "ask", map[string]any{"model": routed(model)})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Ask changes this machine"),
			Detail:  i18n.T("What a new ask in this app starts with: it only proposes commands, runs them once you allow each, or runs them by itself. Asks from the bar and the terminal never change anything."),
			Changed: p.changed.askApprovals,
			Reset:   func() { set(e, "ask", map[string]any{"approvals": stock.Approvals}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dimensions := p.askMay.Layout(gtx, th, e.Overlays, mayLabels, selectedMay)
				if index, changed := p.askMay.Changed(); changed && index < len(mayIDs) {
					set(e, "ask", map[string]any{"approvals": mayIDs[index]})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Learn from the shell"),
			Detail:  i18n.T("Ask reads the commands you run often, so its answers fit how you work. They stay on this machine."),
			Changed: p.changed.habits,
			Reset:   func() { set(e, "ask", map[string]any{"habits": stock.Habits}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Learn from the shell"), &p.habits, func(on bool) {
					set(e, "ask", map[string]any{"habits": on})
				})
			},
		},
	})
}

func (p *Page) layoutAgents(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	settings := e.Snapshot().Settings.Agent
	stock := p.defaults.Agent
	// the form keeps itself on agent.model and agent.options and saves every pick there
	p.agentModel.Parts, p.agentModel.Agents = env.ModelOptions|env.ModelCheck, true
	p.agentModel.FollowAgent(e)
	approvalIDs := [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	approvalLabels := []string{i18n.T("Before it changes anything"), i18n.T("Before every action"), i18n.T("Never (YOLO)")}
	selectedApproval := 0
	for index, id := range approvalIDs {
		if id == settings.Approvals {
			selectedApproval = index
		}
	}
	p.plan.Value = settings.Plan
	p.idle.Value, p.idle.Min, p.idle.Max, p.idle.Step, p.idle.Unit = settings.IdleStopMinutes, 1, 120, 1, i18n.T("min")
	p.notices.Value = e.Notices
	return look.SettingsSection(gtx, th, ordinal(sectionAgents), []look.Setting{
		{
			Title:   i18n.T("New sessions use"),
			Detail:  i18n.T("The model a new session starts on and how hard it thinks. Auto takes one on this computer first. Picking another when you start a session changes it here too."),
			Below:   true,
			Changed: p.changed.agentModel,
			Reset: func() {
				// the patch merges options key by key, so each one picked is sent as null to go
				options := make(map[string]any, len(settings.Options))
				for id := range settings.Options {
					options[id] = nil
				}
				set(e, "agent", map[string]any{"model": stock.Model, "options": options})
			},
			Control: func(gtx layout.Context) layout.Dimensions { return p.agentModel.Layout(gtx, e) },
		},
		{
			Title:   i18n.T("Plan first"),
			Detail:  i18n.T("A new session reads, then writes a plan, and changes nothing until you approve it."),
			Changed: p.changed.plan,
			Reset:   func() { set(e, "agent", map[string]any{"plan": stock.Plan}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Plan first"), &p.plan, func(on bool) {
					set(e, "agent", map[string]any{"plan": on})
				})
			},
		},
		{
			Title:   i18n.T("An agent asks you"),
			Detail:  i18n.T("Never lets it edit files and run commands without a question. Use it only where you can undo what it does."),
			Changed: p.changed.agentApprovals,
			Reset:   func() { set(e, "agent", map[string]any{"approvals": stock.Approvals}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(selectWidth)
				dimensions := p.approvals.Layout(gtx, th, e.Overlays, approvalLabels, selectedApproval)
				if index, changed := p.approvals.Changed(); changed && index < len(approvalIDs) {
					set(e, "agent", map[string]any{"approvals": approvalIDs[index]})
				}
				return dimensions
			},
		},
		{
			Title:   i18n.T("Tell me when an agent needs me"),
			Detail:  i18n.T("A desktop notification when one asks, fails or finishes while Hyakki is not the window in front."),
			Changed: p.changed.notices,
			Reset:   func() { e.SetNotices(true) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Tell me when an agent needs me"), &p.notices, e.SetNotices)
			},
		},
		{
			Title:   i18n.T("Stop an idle agent after"),
			Detail:  i18n.T("Its tools and background commands stop and free their memory. The session picks up again when you write to it."),
			Changed: p.changed.idle,
			Reset:   func() { set(e, "agent", map[string]any{"idleStopMinutes": stock.IdleStopMinutes}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := p.idle.Layout(gtx, th.Theme)
				if p.idle.Changed() {
					set(e, "agent", map[string]any{"idleStopMinutes": p.idle.Value})
				}
				return dimensions
			},
		},
	})
}

func (p *Page) layoutIndex(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	settings := e.Snapshot().Settings.Index
	stock := p.defaults.Index
	p.system.Value, p.semantic.Value = settings.System, settings.Semantic
	return look.SettingsSection(gtx, th, ordinal(sectionIndex), []look.Setting{
		{
			Title:   i18n.T("Read this system"),
			Detail:  i18n.T("Ask answers from your configuration, the manuals and Ryoku's documentation instead of from memory."),
			Changed: p.changed.system,
			Reset:   func() { set(e, "index", map[string]any{"system": stock.System}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Read this system"), &p.system, func(on bool) {
					set(e, "index", map[string]any{"system": on})
				})
			},
		},
		{
			Title:   i18n.T("Search by meaning"),
			Detail:  i18n.T("Finds code and text that mean what you asked, not only what you typed. It takes more disk and a provider that makes embeddings."),
			Changed: p.changed.semantic,
			Reset:   func() { set(e, "index", map[string]any{"semantic": stock.Semantic}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Search by meaning"), &p.semantic, func(on bool) {
					set(e, "index", map[string]any{"semantic": on})
				})
			},
		},
	})
}
