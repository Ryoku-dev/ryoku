package settings

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type providerControls struct {
	row    look.SummaryRow
	toggle kit.Toggle
	key    kit.TextField
	add    kit.Plaque
	test   kit.Plaque
	signup kit.Plaque
	remove map[string]*kit.Plaque
	status string
}

type instanceControls struct {
	id, name, command, environment kit.TextField
	save, remove                   kit.Plaque
	ready                          bool
}

type Page struct {
	scroll           look.Scroller
	section          kit.SectionIndex
	providers        map[string]*providerControls
	expandedProvider string
	instances        map[string]*instanceControls
	newInstance      instanceControls
	models           protocol.ModelsResult
	modelsReady      bool
	askModel         look.Choice
	habits           kit.Toggle
	harness          look.Choice
	approvals        look.Choice
	idle             kit.Stepper
	system           kit.Toggle
	semantic         kit.Toggle
	turnOff          kit.Plaque
	current          int
	error            string
}

func (p *Page) OpenSection(section int) {
	if section < 0 || section > 4 {
		return
	}
	p.current = section
	p.scroll.ScrollTo(section)
}

func (p *Page) Layout(gtx layout.Context, e *env.Env, class kit.FrameClass) layout.Dimensions {
	p.loadModels(e)
	labels := []string{i18n.T("Providers"), i18n.T("Ask"), i18n.T("Agent"), i18n.T("Index"), i18n.T("About")}
	return (kit.Pane{Kind: kit.PaneWork}).Layout(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 34, Right: 36, Bottom: 34, Left: 36}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if class != kit.FrameExpanded {
						return layout.Dimensions{}
					}
					gtx.Constraints.Min.X, gtx.Constraints.Max.X = gtx.Dp(200), gtx.Dp(200)
					dimensions := p.section.Layout(gtx, e.Theme.Theme, labels, p.current, class)
					if selected, changed := p.section.Selected(); changed {
						p.current = selected
						p.scroll.ScrollTo(selected)
					}
					return dimensions
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if class == kit.FrameExpanded {
						return layout.Spacer{Width: 34}.Layout(gtx)
					}
					return layout.Dimensions{}
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if class != kit.FrameMedium {
								return layout.Dimensions{}
							}
							dimensions := p.section.Layout(gtx, e.Theme.Theme, labels, p.current, class)
							if selected, changed := p.section.Selected(); changed {
								p.current = selected
								p.scroll.ScrollTo(selected)
							}
							return dimensions
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return p.scroll.Layout(gtx, e.Theme, len(labels), func(gtx layout.Context, index int) layout.Dimensions {
								return layout.Inset{Bottom: 55}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									switch index {
									case 0:
										return p.providersSection(gtx, e)
									case 1:
										return p.askSection(gtx, e)
									case 2:
										return p.agentSection(gtx, e)
									case 3:
										return p.indexSection(gtx, e)
									default:
										return p.aboutSection(gtx, e)
									}
								})
							})
						}),
					)
				}),
			)
		})
	})
}

func (p *Page) providersSection(gtx layout.Context, e *env.Env) layout.Dimensions {
	providers := orderedProviders(e.Snapshot().Providers)
	children := []layout.FlexChild{
		layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Section, e.Theme.R.Fg, i18n.T("Providers"))),
		layout.Rigid(layout.Spacer{Height: 13}.Layout),
	}
	for _, provider := range providers {
		provider := provider
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.provider(gtx, e, provider) }),
			layout.Rigid(layout.Spacer{Height: 4}.Layout),
		)
	}
	if len(providers) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (kit.Empty{Title: i18n.T("No providers"), Body: i18n.T("The provider catalogue is not available yet.")}).Layout(gtx, e.Theme.Theme)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (p *Page) provider(gtx layout.Context, e *env.Env, provider protocol.ProviderInfo) layout.Dimensions {
	controls := p.providerControl(provider.ID)
	controls.toggle.Value, controls.toggle.Label = provider.Enabled, i18n.T("Enabled")
	controls.key.Label, controls.key.Placeholder, controls.key.Secret = i18n.T("API key"), provider.KeyHint, true
	controls.add.Kind, controls.add.Label = kit.PlaqueLine, i18n.T("Add key")
	controls.test.Kind, controls.test.Label = kit.PlaqueLine, i18n.T("Test")
	controls.signup.Kind, controls.signup.Label = kit.PlaqueQuiet, i18n.T("Sign up")
	if controls.toggle.Changed() {
		var changed protocol.ProviderInfo
		e.Call(protocol.MProvidersEnable, protocol.EnableProviderParams{Provider: provider.ID, Enabled: controls.toggle.Value}, &changed, nil)
	}
	if controls.add.Clicked(gtx) && strings.TrimSpace(controls.key.Text()) != "" {
		var changed protocol.ProviderInfo
		e.Call(protocol.MProvidersAddKey, protocol.AddKeyParams{Provider: provider.ID, Key: strings.TrimSpace(controls.key.Text())}, &changed, func(err error) {
			if err == nil {
				controls.key.SetText("")
			}
		})
	}
	if controls.test.Clicked(gtx) {
		var result protocol.ProviderTestResult
		e.Call(protocol.MProvidersTest, protocol.ProviderParams{Provider: provider.ID}, &result, func(err error) {
			if err != nil {
				controls.status = err.Error()
			} else {
				controls.status = result.Message
				if controls.status == "" && result.OK {
					controls.status = i18n.T("Ready")
				}
			}
		})
	}
	if controls.signup.Clicked(gtx) && provider.Signup != "" {
		env.Spawn("xdg-open", provider.Signup)
	}

	stateWord, tone := providerState(provider)
	controls.row.Title = provider.Name
	controls.row.State = stateWord
	controls.row.Meta = providerMeta(provider)
	controls.row.Excerpt = ""
	controls.row.Tone = tone
	if controls.row.Clicked(gtx) || controls.row.DoubleClicked(gtx) {
		if p.expandedProvider == provider.ID {
			p.expandedProvider = ""
		} else {
			p.expandedProvider = provider.ID
		}
	}
	expanded := p.expandedProvider == provider.ID
	row := func(gtx layout.Context) layout.Dimensions {
		return controls.row.Layout(gtx, e.Theme, expanded)
	}
	if !expanded {
		return row(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(row),
		layout.Rigid(layout.Spacer{Height: 4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return controls.toggle.Layout(gtx, e.Theme.Theme)
					}),
				}
				for _, key := range provider.Keys {
					key := key
					button := controls.remove[key.ID]
					if button == nil {
						button = new(kit.Plaque)
						controls.remove[key.ID] = button
					}
					children = append(children,
						layout.Rigid(layout.Spacer{Height: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							button.Kind, button.Label = kit.PlaqueDangerLine, i18n.T("Remove")
							if button.Clicked(gtx) {
								var changed protocol.ProviderInfo
								e.Call(protocol.MProvidersRemoveKey, protocol.RemoveKeyParams{Provider: provider.ID, Key: key.ID}, &changed, nil)
							}
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, kit.Text(e.Theme.Theme, e.Theme.T.MonoSm, e.Theme.R.Fg2, key.Masked+" · "+i18n.T(key.Status))),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions { return button.Layout(gtx, e.Theme.Theme) }),
							)
						}),
					)
				}
				if !provider.Keyless {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: 13}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.key.Layout(gtx, e.Theme.Theme) }),
						layout.Rigid(layout.Spacer{Height: 8}.Layout),
					)
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if provider.Keyless {
								return layout.Dimensions{}
							}
							return controls.add.Layout(gtx, e.Theme.Theme)
						}),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.test.Layout(gtx, e.Theme.Theme) }),
						layout.Rigid(layout.Spacer{Width: 8}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if provider.Signup == "" {
								return layout.Dimensions{}
							}
							return controls.signup.Layout(gtx, e.Theme.Theme)
						}),
					)
				}))
				if controls.status != "" {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: 8}.Layout),
						layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, controls.status)),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}),
	)
}

func orderedProviders(providers []protocol.ProviderInfo) []protocol.ProviderInfo {
	ordered := append([]protocol.ProviderInfo(nil), providers...)
	sort.SliceStable(ordered, func(left, right int) bool {
		leftPriority, rightPriority := providerPriority(ordered[left]), providerPriority(ordered[right])
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		if ordered[left].Name != ordered[right].Name {
			return providerNameLess(ordered[left].Name, ordered[right].Name)
		}
		return ordered[left].ID < ordered[right].ID
	})
	return ordered
}

func providerNameLess(left, right string) bool {
	limit := min(len(left), len(right))
	for index := range limit {
		leftByte, rightByte := left[index], right[index]
		if leftByte >= 'A' && leftByte <= 'Z' {
			leftByte += 'a' - 'A'
		}
		if rightByte >= 'A' && rightByte <= 'Z' {
			rightByte += 'a' - 'A'
		}
		if leftByte != rightByte {
			return leftByte < rightByte
		}
	}
	return len(left) < len(right)
}

func providerPriority(provider protocol.ProviderInfo) int {
	switch {
	case provider.Enabled && provider.Available > 0 && !provider.Keyless:
		return 0
	case provider.Enabled && provider.Available > 0 && provider.Keyless:
		return 1
	default:
		return 2
	}
}

func providerState(provider protocol.ProviderInfo) (string, palette.Tone) {
	switch {
	case !provider.Enabled:
		return i18n.T("Disabled"), palette.Idle
	case provider.Available > 0:
		return i18n.T("Ready"), palette.Alive
	case provider.CooldownUntil != nil:
		return i18n.T("Cooling down"), palette.Paused
	case !provider.Configured:
		return i18n.T("Needs key"), palette.Attention
	default:
		return i18n.T("Unavailable"), palette.Failed
	}
}

func providerMeta(provider protocol.ProviderInfo) string {
	keys := i18n.Tf("%d keys", len(provider.Keys))
	if len(provider.Keys) == 1 {
		keys = i18n.T("1 key")
	}
	models := i18n.Tf("%d models", provider.Models)
	if provider.Models == 1 {
		models = i18n.T("1 model")
	}
	return keys + " · " + models
}

func (p *Page) askSection(gtx layout.Context, e *env.Env) layout.Dimensions {
	settings := e.Snapshot().Settings.Ask
	labels, ids, selected := p.modelOptions(settings.Model)
	p.habits.Value, p.habits.Label = settings.Habits, i18n.T("Include shell habits")
	return kit.Section(e.Theme.Theme, i18n.T("Ask"), func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			p.askModel.Label = i18n.T("Default model")
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.askModel.Layout(gtx, e.Theme, e.Overlays, labels, selected)
					if index, changed := p.askModel.Changed(); changed && index < len(ids) {
						e.SetSettings(map[string]any{"ask": map[string]any{"model": ids[index]}}, nil)
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.habits.Layout(gtx, e.Theme.Theme)
					if p.habits.Changed() {
						e.SetSettings(map[string]any{"ask": map[string]any{"habits": p.habits.Value}}, nil)
					}
					return dimensions
				}),
			)
		})
	})(gtx)
}

func (p *Page) agentSection(gtx layout.Context, e *env.Env) layout.Dimensions {
	settings := e.Snapshot().Settings.Agent
	installed := make([]protocol.HarnessInfo, 0)
	for _, harness := range e.Snapshot().Harnesses {
		if harness.Installed {
			installed = append(installed, harness)
		}
	}
	harnessLabels, harnessIDs, selectedHarness := []string{}, []string{}, 0
	for index, harness := range installed {
		harnessLabels, harnessIDs = append(harnessLabels, harness.Name), append(harnessIDs, harness.ID)
		if harness.ID == settings.Harness {
			selectedHarness = index
		}
	}
	if len(harnessLabels) == 0 {
		harnessLabels, harnessIDs = []string{i18n.T("No installed harness")}, []string{""}
	}
	approvalIDs := []string{protocol.ApprovalsReadOnly, protocol.ApprovalsAsk, protocol.ApprovalsAuto}
	approvalLabels := []string{i18n.T("Read only"), i18n.T("Ask"), i18n.T("Auto")}
	selectedApproval := indexOf(approvalIDs, settings.Approvals)
	p.idle.Value, p.idle.Min, p.idle.Max, p.idle.Step, p.idle.Unit = settings.IdleStopMinutes, 1, 120, 1, i18n.T("minutes")
	return kit.Section(e.Theme.Theme, i18n.T("Agent"), func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			p.harness.Label = i18n.T("Default harness")
			p.approvals.Label = i18n.T("Approvals")
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.harness.Layout(gtx, e.Theme, e.Overlays, harnessLabels, selectedHarness)
					if index, changed := p.harness.Changed(); changed && index < len(harnessIDs) && harnessIDs[index] != "" {
						e.SetSettings(map[string]any{"agent": map[string]any{"harness": harnessIDs[index]}}, nil)
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("Read only runs reads and asks before writes. Ask requests every action. Auto runs without asking."))),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.approvals.Layout(gtx, e.Theme, e.Overlays, approvalLabels, selectedApproval)
					if index, changed := p.approvals.Changed(); changed && index < len(approvalIDs) {
						e.SetSettings(map[string]any{"agent": map[string]any{"approvals": approvalIDs[index]}}, nil)
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Idle agent timeout"))),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("Stop an idle agent after this many minutes."))),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.idle.Layout(gtx, e.Theme.Theme)
					if p.idle.Changed() {
						e.SetSettings(map[string]any{"agent": map[string]any{"idleStopMinutes": p.idle.Value}}, nil)
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 21}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Harness capabilities"))),
				layout.Rigid(layout.Spacer{Height: 8}.Layout),
			}
			for _, harness := range e.Snapshot().Harnesses {
				harness := harness
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return kit.Facts(e.Theme.Theme, []kit.Fact{{Label: harness.Name, Value: capabilityText(harness)}})(gtx)
				}))
			}
			children = append(children,
				layout.Rigid(layout.Spacer{Height: 21}.Layout),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Row, e.Theme.R.Fg, i18n.T("Custom harness instances"))),
				layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Fg2, i18n.T("Add an ACP command, with one KEY=value environment entry per line."))),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
			)
			for _, instance := range settings.Instances {
				instance := instance
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.instanceEditor(gtx, e, instance, false) }),
					layout.Rigid(layout.Spacer{Height: 13}.Layout),
				)
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.instanceEditor(gtx, e, protocol.HarnessInstance{}, true)
			}))
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})(gtx)
}

func (p *Page) indexSection(gtx layout.Context, e *env.Env) layout.Dimensions {
	settings := e.Snapshot().Settings.Index
	p.system.Value, p.system.Label = settings.System, i18n.T("Index system files")
	p.semantic.Value, p.semantic.Label = settings.Semantic, i18n.T("Use semantic vectors")
	return kit.Section(e.Theme.Theme, i18n.T("Index"), func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.system.Layout(gtx, e.Theme.Theme)
					if p.system.Changed() {
						e.SetSettings(map[string]any{"index": map[string]any{"system": p.system.Value}}, nil)
					}
					return dimensions
				}),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					dimensions := p.semantic.Layout(gtx, e.Theme.Theme)
					if p.semantic.Changed() {
						e.SetSettings(map[string]any{"index": map[string]any{"semantic": p.semantic.Value}}, nil)
					}
					return dimensions
				}),
			)
		})
	})(gtx)
}

func (p *Page) aboutSection(gtx layout.Context, e *env.Env) layout.Dimensions {
	p.turnOff.Kind, p.turnOff.Label = kit.PlaqueDangerLine, i18n.T("Turn off")
	if p.turnOff.Clicked(gtx) {
		env.Spawn("rashin", "off")
	}
	version := e.Snapshot().Status.Version
	if version == "" {
		version = i18n.T("Not connected")
	}
	return kit.Section(e.Theme.Theme, i18n.T("About"), func(gtx layout.Context) layout.Dimensions {
		return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(kit.Facts(e.Theme.Theme, []kit.Fact{{Label: i18n.T("Daemon version"), Value: version, Mono: true}, {Label: i18n.T("State"), Value: i18n.T("On")}})),
				layout.Rigid(layout.Spacer{Height: 13}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.turnOff.Layout(gtx, e.Theme.Theme) }),
			)
		})
	})(gtx)
}

func (p *Page) providerControl(id string) *providerControls {
	if p.providers == nil {
		p.providers = make(map[string]*providerControls)
	}
	controls := p.providers[id]
	if controls == nil {
		controls = &providerControls{remove: make(map[string]*kit.Plaque)}
		p.providers[id] = controls
	}
	return controls
}

func (p *Page) loadModels(e *env.Env) {
	if p.modelsReady {
		return
	}
	p.modelsReady = true
	e.Call(protocol.MModelsList, protocol.ModelsParams{}, &p.models, func(err error) {
		if err != nil {
			p.modelsReady = false
			p.error = err.Error()
		}
	})
}

func (p *Page) modelOptions(current string) ([]string, []string, int) {
	var labels, ids []string
	for _, route := range p.models.Routes {
		labels, ids = append(labels, route.Name), append(ids, route.ID)
	}
	for _, model := range p.models.Models {
		if model.Available {
			labels = append(labels, fmt.Sprintf("%s · %s", model.Provider, model.Name))
			ids = append(ids, model.Provider+"/"+model.ID)
		}
	}
	if len(ids) == 0 {
		labels, ids = []string{i18n.T("Auto")}, []string{protocol.RouteAuto}
	}
	return labels, ids, indexOf(ids, current)
}

func indexOf(values []string, value string) int {
	for index, candidate := range values {
		if candidate == value {
			return index
		}
	}
	return 0
}

func (p *Page) instanceEditor(gtx layout.Context, e *env.Env, instance protocol.HarnessInstance, create bool) layout.Dimensions {
	var controls *instanceControls
	if create {
		controls = &p.newInstance
	} else {
		if p.instances == nil {
			p.instances = make(map[string]*instanceControls)
		}
		controls = p.instances[instance.ID]
		if controls == nil {
			controls = new(instanceControls)
			p.instances[instance.ID] = controls
		}
	}
	if !controls.ready {
		controls.ready = true
		controls.id.SetText(instance.ID)
		controls.name.SetText(instance.Name)
		controls.command.SetText(strings.Join(instance.Command, " "))
		controls.environment.SetText(environmentText(instance.Env))
	}
	controls.id.Label, controls.name.Label = i18n.T("ID"), i18n.T("Name")
	controls.command.Label, controls.command.Placeholder = i18n.T("Command"), i18n.T("agent --acp")
	controls.environment.Label, controls.environment.Placeholder = i18n.T("Environment"), "KEY=value"
	controls.save.Kind = kit.PlaqueLine
	if create {
		controls.save.Label = i18n.T("Add instance")
	} else {
		controls.save.Label = i18n.T("Save")
	}
	controls.remove.Kind, controls.remove.Label = kit.PlaqueDangerLine, i18n.T("Remove")
	if controls.save.Clicked(gtx) {
		id := strings.TrimSpace(controls.id.Text())
		command, commandErr := splitCommandLine(controls.command.Text())
		environment, environmentErr := parseEnvironment(controls.environment.Text())
		if id == "" || strings.TrimSpace(controls.name.Text()) == "" || len(command) == 0 {
			p.error = i18n.T("ID, name and command are required.")
		} else if commandErr != nil {
			p.error = commandErr.Error()
		} else if environmentErr != nil {
			p.error = environmentErr.Error()
		} else {
			replacement := protocol.HarnessInstance{ID: id, Name: strings.TrimSpace(controls.name.Text()), Command: command, Env: environment}
			instances := append([]protocol.HarnessInstance(nil), e.Snapshot().Settings.Agent.Instances...)
			if create {
				instances = append(instances, replacement)
			} else {
				for index := range instances {
					if instances[index].ID == instance.ID {
						instances[index] = replacement
					}
				}
			}
			e.SetSettings(map[string]any{"agent": map[string]any{"instances": instances}}, func(err error) {
				if err != nil {
					p.error = err.Error()
				} else if create {
					p.newInstance = instanceControls{}
				}
			})
		}
	}
	if !create && controls.remove.Clicked(gtx) {
		instances := make([]protocol.HarnessInstance, 0, len(e.Snapshot().Settings.Agent.Instances)-1)
		for _, candidate := range e.Snapshot().Settings.Agent.Instances {
			if candidate.ID != instance.ID {
				instances = append(instances, candidate)
			}
		}
		e.SetSettings(map[string]any{"agent": map[string]any{"instances": instances}}, nil)
		delete(p.instances, instance.ID)
	}
	return kit.SlabContent(gtx, e.Theme.Theme, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.id.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.name.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.command.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.environment.Layout(gtx, e.Theme.Theme) }),
			layout.Rigid(layout.Spacer{Height: 8}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return controls.save.Layout(gtx, e.Theme.Theme) }),
					layout.Rigid(layout.Spacer{Width: 8}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if create {
							return layout.Dimensions{}
						}
						return controls.remove.Layout(gtx, e.Theme.Theme)
					}),
				)
			}),
		}
		if p.error != "" {
			children = append(children, layout.Rigid(layout.Spacer{Height: 8}.Layout), layout.Rigid(kit.Text(e.Theme.Theme, e.Theme.T.Caption, e.Theme.R.Jasper, p.error)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func capabilityText(harness protocol.HarnessInfo) string {
	if harness.Capabilities == nil {
		return i18n.T("Capabilities not learned yet")
	}
	capabilities := harness.Capabilities
	var parts []string
	if capabilities.LoadSession {
		parts = append(parts, i18n.T("Resume"))
	}
	if capabilities.Models {
		parts = append(parts, i18n.T("Models"))
	}
	if len(capabilities.Modes) > 0 {
		parts = append(parts, i18n.T("Modes"))
	}
	if capabilities.Images {
		parts = append(parts, i18n.T("Images"))
	}
	if capabilities.Audio {
		parts = append(parts, i18n.T("Audio"))
	}
	if capabilities.Context {
		parts = append(parts, i18n.T("Context"))
	}
	if len(parts) == 0 {
		return i18n.T("Basic ACP")
	}
	return strings.Join(parts, ", ")
}

func environmentText(environment map[string]string) string {
	keys := make([]string, 0, len(environment))
	for key := range environment {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+environment[key])
	}
	return strings.Join(lines, "\n")
}

func parseEnvironment(text string) (map[string]string, error) {
	result := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, errors.New(i18n.T("Environment entries must use KEY=value."))
		}
		result[key] = value
	}
	return result, nil
}

func splitCommandLine(text string) ([]string, error) {
	var result []string
	var word strings.Builder
	quote := rune(0)
	escaped := false
	flush := func() {
		if word.Len() > 0 {
			result = append(result, word.String())
			word.Reset()
		}
	}
	for _, char := range strings.TrimSpace(text) {
		if escaped {
			word.WriteRune(char)
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
		case ' ', '\t', '\n':
			flush()
		default:
			word.WriteRune(char)
		}
	}
	if escaped || quote != 0 {
		return nil, errors.New(i18n.T("The command has an unfinished quote or escape."))
	}
	flush()
	return result, nil
}
