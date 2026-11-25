package env

import (
	"maps"
	"slices"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type ModelParts uint8

const (
	ModelOptions ModelParts = 1 << iota
	ModelPlan
	ModelAsking
	ModelCheck
)

type ModelInherit struct {
	Label   string
	Model   string
	Options map[string]string
}

type ModelChangeKind uint8

const (
	ModelPicked ModelChangeKind = iota
	OptionPicked
	PlanPicked
	ApprovalsPicked
)

type ModelChange struct {
	Kind          ModelChangeKind
	Option, Value string
}

// a form is owned by its page on the window goroutine; the model catalogue belongs to the window
type ModelForm struct {
	Parts   ModelParts
	Agents  bool
	Inherit *ModelInherit
	Label   string
	Offered []protocol.SessionOption

	model          string
	options        map[string]string
	plan           bool
	approvals      string
	changes        []ModelChange
	selectModel    look.Select
	selectApproval look.Select
	selectOptions  map[string]*look.Select
	planSwitch     kit.Toggle
	selectPlan     look.Select
	check          kit.Plaque
	stop           kit.Plaque
	stopping       bool
	checking       bool
	checkText      string
	checkFailed    bool
	checkedModel   string
	checkedOptions []protocol.SessionOption

	followed        bool
	remoteModel     string
	remoteOptions   map[string]string
	remotePlan      bool
	remoteApprovals string
}

func (f *ModelForm) Set(model string, options map[string]string) {
	clean := maps.Clone(options)
	delete(clean, protocol.OptionPlan)
	if f.model == model && maps.Equal(f.options, clean) {
		return
	}
	f.model, f.options = model, clean
	f.checkedModel, f.checkedOptions, f.checkText = "", nil, ""
	f.checkFailed = false
	f.changes = nil
}

func (f *ModelForm) SetPlan(on bool) { f.plan = on }

func (f *ModelForm) SetApprovals(value string) {
	if !protocol.KnownApprovals(value) {
		value = protocol.ApprovalsReadOnly
	}
	f.approvals = value
}

func (f *ModelForm) Value() (string, map[string]string) {
	return f.model, maps.Clone(f.options)
}

func (f *ModelForm) Effective() (string, map[string]string) {
	model, options := f.Value()
	if f.Inherit != nil {
		if model == "" {
			model = f.Inherit.Model
		}
		merged := maps.Clone(f.Inherit.Options)
		if merged == nil && len(options) > 0 {
			merged = make(map[string]string, len(options))
		}
		maps.Copy(merged, options)
		options = merged
	}
	if model == "" {
		model = protocol.RouteAuto
	}
	delete(options, protocol.OptionPlan)
	return model, options
}

func (f *ModelForm) PlanFirst() bool { return f.plan }

func (f *ModelForm) Approvals() string {
	if protocol.KnownApprovals(f.approvals) {
		return f.approvals
	}
	return protocol.ApprovalsReadOnly
}

func (f *ModelForm) SessionOptions() map[string]string {
	_, options := f.Effective()
	if f.Parts&ModelPlan != 0 {
		if options == nil {
			options = make(map[string]string, 1)
		}
		options[protocol.OptionPlan] = protocol.OptionOff
		if f.plan {
			options[protocol.OptionPlan] = protocol.OptionOn
		}
	}
	return options
}

func (f *ModelForm) Changed() (ModelChange, bool) {
	if len(f.changes) == 0 {
		return ModelChange{}, false
	}
	change := f.changes[0]
	f.changes = f.changes[1:]
	return change, true
}

// the form's owner reads picks before layout, so a newly made pick needs another frame
func (f *ModelForm) queue(gtx layout.Context, change ModelChange) {
	f.changes = append(f.changes, change)
	gtx.Execute(op.InvalidateCmd{})
}

func (f *ModelForm) Ready(e *Env) bool {
	menu := Models(e)
	if !menu.loaded {
		return true
	}
	model, _ := f.Effective()
	if protocol.IsRoute(model) {
		if f.Agents {
			return menu.agentReady()
		}
		return true
	}
	info, ok := menu.Info(model)
	return ok && info.Available && (!f.Agents || info.Tools) &&
		(info.Provider != protocol.ProviderRashinModels || info.Tools && info.Context >= 8192 && info.Params >= 500_000_000)
}

func (f *ModelForm) modelChoices(e *Env) ([]look.SelectOption, []string, int) {
	menu := Models(e)
	model, _ := f.Effective()
	options, ids := menu.list(model, f.Agents)
	if f.Inherit != nil {
		label := f.Inherit.Label
		if label == "" {
			label = i18n.T("Default")
		}
		label = i18n.Tf("%s (%s)", label, menu.Words(f.Inherit.Model))
		options = append([]look.SelectOption{{Label: label, Detail: i18n.T("Follows the default")}}, options...)
		ids = append([]string{""}, ids...)
	}
	selected := max(0, slices.Index(ids, f.model))
	if f.Inherit == nil && f.model == "" {
		selected = max(0, slices.Index(ids, protocol.RouteAuto))
	}
	return options, ids, selected
}

func (f *ModelForm) modelField(gtx layout.Context, e *Env, quiet bool, home any) layout.Dimensions {
	options, ids, selected := f.modelChoices(e)
	label := f.Label
	if label == "" {
		label = i18n.T("Model")
	}
	f.selectModel.Form, f.selectModel.Label, f.selectModel.Return = look.SelectField, label, home
	if quiet {
		f.selectModel.Form = look.SelectQuiet
	}
	f.selectModel.Searched = true
	dim := f.selectModel.LayoutOptions(gtx, e.Theme, e.Overlays, options, selected)
	if index, changed := f.selectModel.Changed(); changed && index >= 0 && index < len(ids) && ids[index] != f.model && !options[index].Disabled {
		f.model = ids[index]
		f.options = nil
		f.checkedModel, f.checkedOptions, f.checkText = "", nil, ""
		f.queue(gtx, ModelChange{Kind: ModelPicked, Value: f.model})
	}
	return dim
}

func (f *ModelForm) availableOptions(e *Env) []protocol.SessionOption {
	if f.Offered != nil {
		return f.Offered
	}
	model, _ := f.Effective()
	if f.checkedModel == model && f.checkedOptions != nil {
		return f.checkedOptions
	}
	info, ok := Models(e).Info(model)
	if !ok || !info.Reasoning {
		return nil
	}
	return []protocol.SessionOption{{ID: protocol.OptionEffort, Name: i18n.T("Thinking"), Category: "thought_level", Choices: []protocol.Choice{
		{ID: "", Name: i18n.T("Default")}, {ID: "off", Name: i18n.T("Off")},
		{ID: "low", Name: i18n.T("Low")}, {ID: "medium", Name: i18n.T("Medium")},
		{ID: "high", Name: i18n.T("High")},
	}}}
}

func (f *ModelForm) optionFields(e *Env, quiet bool, home any) []layout.Widget {
	if f.Parts&ModelOptions == 0 {
		return nil
	}
	available := f.availableOptions(e)
	controls := make([]layout.Widget, 0, len(available))
	for _, offered := range available {
		if offered.ID == protocol.OptionPlan || len(offered.Choices) == 0 {
			continue
		}
		if f.selectOptions == nil {
			f.selectOptions = make(map[string]*look.Select)
		}
		selection := f.selectOptions[offered.ID]
		if selection == nil {
			selection = new(look.Select)
			f.selectOptions[offered.ID] = selection
		}
		id := offered.ID
		choices := offered.Choices
		name := offered.Name
		if name == "" {
			name = id
		}
		controls = append(controls, func(gtx layout.Context) layout.Dimensions {
			current := f.options[id]
			if current == "" && f.Inherit != nil {
				current = f.Inherit.Options[id]
			}
			list := ListOf(choices, current)
			if list.Selected < 0 {
				list.Selected = max(0, slices.Index(list.IDs, ""))
			}
			selection.Form, selection.Label, selection.Return = look.SelectField, name, home
			if quiet {
				selection.Form = look.SelectQuiet
			}
			if id == protocol.OptionEffort {
				selection.Icon = look.Icon.Thought
			}
			dim := selection.LayoutOptions(gtx, e.Theme, e.Overlays, list.Options, list.Selected)
			if index, changed := selection.Changed(); changed && index >= 0 && index < len(list.IDs) && list.IDs[index] != current {
				if f.options == nil {
					f.options = make(map[string]string)
				}
				value := list.IDs[index]
				if value == "" {
					delete(f.options, id)
				} else {
					f.options[id] = value
				}
				f.checkText = ""
				f.queue(gtx, ModelChange{Kind: OptionPicked, Option: id, Value: value})
			}
			return dim
		})
	}
	return controls
}

func (f *ModelForm) approvalField(gtx layout.Context, e *Env, quiet bool, home any) layout.Dimensions {
	value := f.Approvals()
	selected := slices.Index(approvalValues[:], value)
	control := &f.selectApproval
	control.Form, control.Label, control.Return = look.SelectField, i18n.T("Asking"), home
	if quiet {
		control.Form = look.SelectQuiet
	}
	control.Icon, control.Tone = look.Icon.Lock, palette.Idle
	if value == protocol.ApprovalsAuto {
		control.Icon, control.Tone = look.Icon.Unlock, palette.Attention
	}
	dim := control.LayoutOptions(gtx, e.Theme, e.Overlays, approvalOptions(AgentApprovals), selected)
	if index, changed := control.Changed(); changed && index >= 0 && index < len(approvalValues) && approvalValues[index] != value {
		f.approvals = approvalValues[index]
		f.queue(gtx, ModelChange{Kind: ApprovalsPicked, Value: f.approvals})
	}
	return dim
}

func (f *ModelForm) planField(gtx layout.Context, e *Env) layout.Dimensions {
	f.planSwitch.Label, f.planSwitch.Value = i18n.T("Plan first"), f.plan
	dim := f.planSwitch.Layout(gtx, e.Theme.Theme)
	if f.planSwitch.Changed() {
		f.plan = f.planSwitch.Value
		value := protocol.OptionOff
		if f.plan {
			value = protocol.OptionOn
		}
		f.queue(gtx, ModelChange{Kind: PlanPicked, Value: value})
	}
	return dim
}
func (f *ModelForm) planControl(gtx layout.Context, e *Env, home any) layout.Dimensions {
	f.selectPlan.Form, f.selectPlan.Label, f.selectPlan.Return = look.SelectQuiet, i18n.T("Plan first"), home
	selected := 0
	if f.plan {
		selected = 1
	}
	dim := f.selectPlan.LayoutOptions(gtx, e.Theme, e.Overlays,
		[]look.SelectOption{{Label: i18n.T("Off")}, {Label: i18n.T("On")}}, selected)
	if index, changed := f.selectPlan.Changed(); changed && index != selected {
		f.plan = index == 1
		value := protocol.OptionOff
		if f.plan {
			value = protocol.OptionOn
		}
		f.queue(gtx, ModelChange{Kind: PlanPicked, Value: value})
	}
	return dim
}

func (f *ModelForm) checkField(gtx layout.Context, e *Env) layout.Dimensions {
	f.check.Kind, f.check.Small, f.check.Label, f.check.Busy = kit.PlaqueTonal, true, i18n.T("Check"), f.checking
	if f.check.Clicked(gtx) && !f.checking && f.Ready(e) {
		model, options := f.Effective()
		f.checking, f.checkText = true, ""
		var result protocol.ModelProbeResult
		e.Call(protocol.MModelsProbe, protocol.ModelProbeParams{Model: model, Options: options}, &result, func(err error) {
			f.checking = false
			current, chosen := f.Effective()
			if model != current || !maps.Equal(options, chosen) {
				return
			}
			if err != nil {
				f.checkText, f.checkFailed = err.Error(), true
				return
			}
			f.checkFailed = !result.OK
			f.checkText = result.Message
			if result.OK {
				f.checkedModel, f.checkedOptions = model, result.Options
				if f.checkText == "" {
					f.checkText = i18n.T("The model answered and called a tool.")
				}
			} else if f.checkText == "" {
				f.checkText = i18n.T("The model did not answer or could not call a tool.")
			}
		})
	}
	if f.checking || !f.Ready(e) {
		gtx = gtx.Disabled()
	}
	return f.check.Layout(gtx, e.Theme.Theme)
}

func (f *ModelForm) stopControl(gtx layout.Context, e *Env) layout.Dimensions {
	runtime := e.Snapshot().Local.Runtime
	if runtime.State != protocol.RuntimeReady && runtime.State != protocol.RuntimeStarting &&
		runtime.State != protocol.RuntimeStopping && !f.stopping {
		return layout.Dimensions{}
	}
	id := runtime.ActiveModel
	if len(runtime.Loaded) > 0 {
		id = runtime.Loaded[0]
	}
	name := i18n.T("Local model")
	for _, model := range e.Snapshot().Local.Models {
		if model.ID == id {
			name = model.Name
			break
		}
	}
	busy := f.stopping || runtime.State == protocol.RuntimeStopping
	f.stop.Kind, f.stop.Small, f.stop.Busy = kit.PlaqueTonal, true, busy
	f.stop.Label = i18n.Tf("Stop the model · %s", name)
	if busy {
		f.stop.Label = i18n.T("Stopping")
	}
	if f.stop.Clicked(gtx) && !busy {
		f.stopping = true
		e.LocalStop(func(err error) { f.stopping = false; e.Report(err) })
	}
	if !busy {
		return f.stop.Layout(gtx, e.Theme.Theme)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.stop.Layout(gtx, e.Theme.Theme) }),
		layout.Rigid(layout.Spacer{Width: e.Theme.Sp.S1}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return kit.Working{Active: true}.Layout(gtx, e.Theme.Theme)
		}),
	)
}

func (f *ModelForm) Layout(gtx layout.Context, e *Env) layout.Dimensions {
	th := e.Theme
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return f.modelField(gtx, e, false, nil)
	})}
	model, _ := f.Effective()
	children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return ModelLine(gtx, e, model)
	}))
	if runtime := e.Snapshot().Local.Runtime; runtime.State == protocol.RuntimeReady || runtime.State == protocol.RuntimeStarting || runtime.State == protocol.RuntimeStopping || f.stopping {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.stopControl(gtx, e) }))
	}
	for _, control := range f.optionFields(e, false, nil) {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(control))
	}
	if f.Parts&ModelAsking != 0 {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.approvalField(gtx, e, false, nil) }))
	}
	if f.Parts&ModelPlan != 0 {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.planField(gtx, e) }))
	}
	if f.Parts&ModelCheck != 0 {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.checkField(gtx, e) }))
		if f.checkText != "" {
			ink := th.R.Fg2
			if f.checkFailed {
				ink = th.R.Stone(palette.Failed)
			}
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, ink, f.checkText)))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (f *ModelForm) Controls(e *Env, home any) []layout.Widget {
	controls := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return f.modelField(gtx, e, true, home) }}
	if runtime := e.Snapshot().Local.Runtime; runtime.State == protocol.RuntimeReady || runtime.State == protocol.RuntimeStarting || runtime.State == protocol.RuntimeStopping || f.stopping {
		controls = append(controls, func(gtx layout.Context) layout.Dimensions { return f.stopControl(gtx, e) })
	}
	controls = append(controls, f.optionFields(e, true, home)...)
	if f.Parts&ModelAsking != 0 {
		controls = append(controls, func(gtx layout.Context) layout.Dimensions { return f.approvalField(gtx, e, true, home) })
	}
	if f.Parts&ModelPlan != 0 {
		controls = append(controls, func(gtx layout.Context) layout.Dimensions { return f.planControl(gtx, e, home) })
	}
	return controls
}

func (f *ModelForm) FollowAgent(e *Env) {
	settings := e.Snapshot().Settings.Agent
	if !f.followed {
		f.Set(settings.Model, settings.Options)
		f.SetPlan(settings.Plan)
		f.SetApprovals(settings.Approvals)
		f.followed = true
		f.remoteModel, f.remoteOptions = settings.Model, maps.Clone(settings.Options)
		f.remotePlan, f.remoteApprovals = settings.Plan, settings.Approvals
	} else {
		if f.remoteModel != settings.Model || !maps.Equal(f.remoteOptions, settings.Options) {
			f.Set(settings.Model, settings.Options)
			f.remoteModel, f.remoteOptions = settings.Model, maps.Clone(settings.Options)
		}
		if f.remotePlan != settings.Plan {
			f.SetPlan(settings.Plan)
			f.remotePlan = settings.Plan
		}
		if f.remoteApprovals != settings.Approvals {
			f.SetApprovals(settings.Approvals)
			f.remoteApprovals = settings.Approvals
		}
	}
	for {
		change, ok := f.Changed()
		if !ok {
			break
		}
		patch := make(map[string]any, 1)
		switch change.Kind {
		case ModelPicked:
			patch["model"] = change.Value
			if len(settings.Options) != 0 {
				cleared := make(map[string]any, len(settings.Options))
				for key := range settings.Options {
					cleared[key] = nil
				}
				patch["options"] = cleared
			}
		case OptionPicked:
			var value any = change.Value
			if change.Value == "" {
				value = nil
			}
			patch["options"] = map[string]any{change.Option: value}
		case PlanPicked:
			patch["plan"] = change.Value == protocol.OptionOn
		case ApprovalsPicked:
			patch["approvals"] = change.Value
		}
		e.SetSettings(map[string]any{"agent": patch}, e.Report)
	}
}
