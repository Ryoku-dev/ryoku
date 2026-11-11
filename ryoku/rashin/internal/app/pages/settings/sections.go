package settings

import (
	"strings"

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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.Monument(gtx, th, i18n.T("Rashin is on."), rashinProof(snapshot))
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S5}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSection(gtx, th, i18n.T("Rashin"), []look.Setting{
				{
					Title:   i18n.T("Running"),
					Detail:  i18n.T("While Rashin is off nothing runs, nothing is indexed and it uses no memory."),
					Control: func(gtx layout.Context) layout.Dimensions { return p.turnOff.Layout(gtx, th.Theme) },
				},
				{
					Title:   i18n.Tf("Version %s", version),
					Detail:  i18n.Tf("The daemon writes what it does to %s.", paths.Short(paths.DaemonLog())),
					Control: func(gtx layout.Context) layout.Dimensions { return p.log.Layout(gtx, th.Theme) },
				},
			})
		}),
	)
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
	labels, ids, selected := p.routeOptions(settings.Model)
	p.habits.Value = settings.Habits
	mayIDs := [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	mayLabels := []string{i18n.T("Never"), i18n.T("After asking you"), i18n.T("Without asking (YOLO)")}
	selectedMay := 0
	for index, id := range mayIDs {
		if id == settings.Approvals {
			selectedMay = index
		}
	}
	return look.SettingsSection(gtx, th, i18n.T("Ask"), []look.Setting{
		{
			Title:  i18n.T("Answers come from"),
			Detail: i18n.T("Auto picks a free model that is up and fits the question. A named model is used whenever it is available."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(260)
				dimensions := p.route.Layout(gtx, th, e.Overlays, labels, selected)
				if index, changed := p.route.Changed(); changed && index < len(ids) {
					set(e, "ask", map[string]any{"model": ids[index]})
				}
				return dimensions
			},
		},
		{
			Title:  i18n.T("Ask changes this machine"),
			Detail: i18n.T("What a new ask in this app starts with: it only proposes commands, runs them once you allow each, or runs them by itself. Asks from the bar and the terminal never change anything."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(260)
				dimensions := p.askMay.Layout(gtx, th, e.Overlays, mayLabels, selectedMay)
				if index, changed := p.askMay.Changed(); changed && index < len(mayIDs) {
					set(e, "ask", map[string]any{"approvals": mayIDs[index]})
				}
				return dimensions
			},
		},
		{
			Title:  i18n.T("Learn from the shell"),
			Detail: i18n.T("Ask reads the commands you run often, so its answers fit how you work. They stay on this machine."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Learn from the shell"), &p.habits, func(on bool) {
					set(e, "ask", map[string]any{"habits": on})
				})
			},
		},
	})
}

func (p *Page) routeOptions(current string) (labels, ids []string, selected int) {
	for _, route := range p.models.Routes {
		labels, ids = append(labels, route.Name), append(ids, route.ID)
	}
	for _, model := range p.models.Models {
		if model.Available {
			labels, ids = append(labels, model.Name), append(ids, model.Provider+"/"+model.ID)
		}
	}
	if len(ids) == 0 {
		return []string{i18n.T("Auto")}, []string{protocol.RouteAuto}, 0
	}
	for index, id := range ids {
		if id == current {
			selected = index
		}
	}
	return labels, ids, selected
}

func (p *Page) layoutAgents(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	settings := snapshot.Settings.Agent
	harnessLabels, harnessIDs, selectedHarness := []string{}, []string{}, 0
	for _, harness := range snapshot.Harnesses {
		if !harness.Installed {
			continue
		}
		if harness.ID == settings.Harness {
			selectedHarness = len(harnessIDs)
		}
		harnessLabels, harnessIDs = append(harnessLabels, harness.Name), append(harnessIDs, harness.ID)
	}
	if len(harnessIDs) == 0 {
		harnessLabels, harnessIDs = []string{i18n.T("None installed")}, []string{""}
	}
	approvalIDs := [...]string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	approvalLabels := []string{i18n.T("Before it changes anything"), i18n.T("Before every action"), i18n.T("Never (YOLO)")}
	selectedApproval := 0
	for index, id := range approvalIDs {
		if id == settings.Approvals {
			selectedApproval = index
		}
	}
	p.idle.Value, p.idle.Min, p.idle.Max, p.idle.Step, p.idle.Unit = settings.IdleStopMinutes, 1, 120, 1, i18n.T("min")
	p.notices.Value = e.Notices
	rows := []look.Setting{
		{
			Title:  i18n.T("New sessions run"),
			Detail: i18n.T("The harness a new session starts with. Picking another when you start one changes it here too."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(260)
				dimensions := p.harness.Layout(gtx, th, e.Overlays, harnessLabels, selectedHarness)
				if index, changed := p.harness.Changed(); changed && index < len(harnessIDs) && harnessIDs[index] != "" {
					set(e, "agent", map[string]any{"harness": harnessIDs[index]})
				}
				return dimensions
			},
		},
		{
			Title:  i18n.T("An agent asks you"),
			Detail: i18n.T("Never lets it edit files and run commands without a question. Use it only where you can undo what it does."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Dp(260)
				dimensions := p.approvals.Layout(gtx, th, e.Overlays, approvalLabels, selectedApproval)
				if index, changed := p.approvals.Changed(); changed && index < len(approvalIDs) {
					set(e, "agent", map[string]any{"approvals": approvalIDs[index]})
				}
				return dimensions
			},
		},
		{
			Title:  i18n.T("Tell me when an agent needs me"),
			Detail: i18n.T("A desktop notification when one asks, fails or finishes while Rashin is not the window in front."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Tell me when an agent needs me"), &p.notices, e.SetNotices)
			},
		},
		{
			Title:  i18n.T("Stop an idle agent after"),
			Detail: i18n.T("Its process ends and frees its memory. The session resumes when you write to it."),
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := p.idle.Layout(gtx, th.Theme)
				if p.idle.Changed() {
					set(e, "agent", map[string]any{"idleStopMinutes": p.idle.Value})
				}
				return dimensions
			},
		},
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return look.SettingsSection(gtx, th, i18n.T("Agents"), rows)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutHarnesses(gtx, e, snapshot) }),
	)
}

func (p *Page) layoutHarnesses(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	// what can run stands above what still has to be installed
	rows := make([]look.Setting, 0, len(snapshot.Harnesses))
	for _, harness := range snapshot.Harnesses {
		if harness.Installed && !harness.Custom {
			rows = append(rows, look.Setting{
				Title: harness.Name, Detail: harnessDetail(harness),
				Control: kit.Text(th.Theme, th.T.Label, th.R.Fg2, i18n.T("Installed")),
			})
		}
	}
	rows = append(rows, p.install.Rows(gtx, e, snapshot)...)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Harnesses"))),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.install.Again(gtx, e) }),
			)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("Your own harnesses"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2,
			i18n.T("Any program that speaks the Agent Client Protocol can run sessions. Give it a name and the command that starts it."))),
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutInstances(gtx, e) }),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func harnessDetail(harness protocol.HarnessInfo) string {
	parts := make([]string, 0, 7)
	if harness.Version != "" {
		parts = append(parts, harness.Version)
	}
	if capabilities := harness.Capabilities; capabilities != nil {
		add := func(has bool, word string) {
			if has {
				parts = append(parts, word)
			}
		}
		add(capabilities.LoadSession, i18n.T("resumes sessions"))
		add(capabilities.Models, i18n.T("offers models"))
		add(len(capabilities.Modes) > 0, i18n.T("has modes"))
		add(capabilities.Images, i18n.T("reads images"))
	}
	detail := strings.Join(parts, ", ")
	if facts := env.HarnessFacts(harness); len(facts) > 0 {
		if detail != "" {
			detail += ". "
		}
		detail += env.FactsSentence(facts)
	}
	return detail
}

func (p *Page) layoutIndex(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	settings := e.Snapshot().Settings.Index
	p.system.Value, p.semantic.Value = settings.System, settings.Semantic
	return look.SettingsSection(gtx, th, i18n.T("Index"), []look.Setting{
		{
			Title:  i18n.T("Read this system"),
			Detail: i18n.T("Ask answers from your configuration, the manuals and Ryoku's documentation instead of from memory."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Read this system"), &p.system, func(on bool) {
					set(e, "index", map[string]any{"system": on})
				})
			},
		},
		{
			Title:  i18n.T("Search by meaning"),
			Detail: i18n.T("Finds code and text that mean what you asked, not only what you typed. It takes more disk and a provider that makes embeddings."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Search by meaning"), &p.semantic, func(on bool) {
					set(e, "index", map[string]any{"semantic": on})
				})
			},
		},
	})
}

func (p *Page) layoutAppearance(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	p.zoom.Value, p.zoom.Min, p.zoom.Max, p.zoom.Step, p.zoom.Unit = e.Zoom, state.MinZoom, state.MaxZoom, state.ZoomStep, "%"
	p.motion.Value = e.ReduceMotion
	contrasts := [...]string{state.ContrastStandard, state.ContrastHigh}
	tints := [...]string{state.TintOff, state.TintLow, state.TintMedium}
	p.contrast.Labels = []string{i18n.T("Standard"), i18n.T("High")}
	p.contrast.Selected = choiceIndex(contrasts[:], e.Contrast)
	p.tint.Labels = []string{i18n.T("Off"), i18n.T("Low"), i18n.T("Medium")}
	p.tint.Selected = choiceIndex(tints[:], e.Tint)
	segmented := func(control *kit.Segmented, picked func(index int)) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(260)
			dimensions := control.Layout(gtx, th.Theme)
			if index, changed := control.Changed(); changed {
				picked(index)
			}
			return dimensions
		}
	}
	return look.SettingsSection(gtx, th, i18n.T("Appearance"), []look.Setting{
		{
			Title:  i18n.T("Zoom"),
			Detail: i18n.T("Ctrl + and Ctrl - change it from anywhere. Ctrl 0 sets it back."),
			Control: func(gtx layout.Context) layout.Dimensions {
				dimensions := p.zoom.Layout(gtx, th.Theme)
				if p.zoom.Changed() {
					e.SetZoom(p.zoom.Value)
				}
				return dimensions
			},
		},
		{
			Title:  i18n.T("Reduce motion"),
			Detail: i18n.T("Everything changes at once instead of moving."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, i18n.T("Reduce motion"), &p.motion, e.SetReduceMotion)
			},
		},
		{
			Title:  i18n.T("Colours"),
			Detail: i18n.T("Rashin takes its colours from your wallpaper. Contrast and Tint change how its surfaces use them."),
		},
		{
			Title:  i18n.T("Contrast"),
			Detail: i18n.T("High spreads the surfaces apart and strengthens edges."),
			Control: segmented(&p.contrast, func(index int) {
				e.SetLook(contrasts[index], tints[p.tint.Selected])
			}),
		},
		{
			Title:  i18n.T("Tint"),
			Detail: i18n.T("Tint draws your wallpaper's colour into the surfaces."),
			Control: segmented(&p.tint, func(index int) {
				e.SetLook(contrasts[p.contrast.Selected], tints[index])
			}),
		},
	})
}

// the stored words are normalised when read, so falling back to the first option is never seen
func choiceIndex(ids []string, value string) int {
	for index, id := range ids {
		if id == value {
			return index
		}
	}
	return 0
}

// read right after it is drawn: by the next frame the setting has overwritten the flip
func namedSwitch(gtx layout.Context, th *look.Theme, name string, toggle *kit.Toggle, flipped func(on bool)) layout.Dimensions {
	dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) })
	if toggle.Changed() && flipped != nil {
		flipped(toggle.Value)
	}
	return dimensions
}
