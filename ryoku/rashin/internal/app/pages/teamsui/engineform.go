package teamsui

import (
	"maps"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the option category the harness names how hard it thinks with; it stands first
const thoughtLevel = "thought_level"

// the engine a member, the teams defaults or one task runs on: harness, model, the harness's own options and a check that they start
type EngineForm struct {
	// Agents makes the first harness option the one Agents starts, as the teams defaults resolve, instead of the teams default chain
	Agents bool
	// Keep is the member one task is changed for: the harness stays the member's and each first choice keeps what it runs
	Keep *protocol.Member

	harnessID, modelID string
	values             map[string]string
	// text is the model field as last seen, so an edit is noticed once
	text string

	harness, model look.Select
	options        map[string]*look.Select
	modelText      kit.TextField
	// typed is a model typed although a list is known
	typed       bool
	read, check kit.Plaque
	busy        bool
	// what the last probe read, until the snapshot carries a list as new
	probed  protocol.HarnessInfo
	result  string
	failed  bool
	changed bool
}

func (f *EngineForm) Set(harness, model string, options map[string]string) {
	f.harnessID, f.modelID, f.values = harness, model, maps.Clone(options)
	f.text = model
	f.modelText.SetText(model)
	f.typed, f.result, f.failed = false, "", false
}

// harness "" follows the first option, model "" takes the default, options hold only what was picked
func (f *EngineForm) Value() (harness, model string, options map[string]string) {
	if len(f.values) > 0 {
		options = maps.Clone(f.values)
	}
	return f.harnessID, f.modelID, options
}

func (f *EngineForm) Changed() bool {
	changed := f.changed
	f.changed = false
	return changed
}

// what a task would start on: for one task the member's engine with the choices over it, else the chain a member's task takes
func (f *EngineForm) Effective(snapshot state.Snapshot) (harness, model string, options map[string]string) {
	fallback := registryDefault(snapshot.Harnesses)
	switch {
	case f.Keep != nil:
		harness, model, options = protocol.ResolveEngine(*f.Keep, snapshot.Settings, fallback)
		if f.modelID != "" {
			model = f.modelID
		}
		if len(f.values) > 0 {
			// a task's options replace the member's, so the member's stay unless one was picked over them
			merged := maps.Clone(options)
			if merged == nil {
				merged = make(map[string]string, len(f.values))
			}
			maps.Copy(merged, f.values)
			options = merged
		}
		return harness, model, options
	case f.Agents:
		harness = f.harnessID
		if harness == "" {
			harness = agentsHarness(snapshot)
		}
		return harness, f.modelID, maps.Clone(f.values)
	}
	return protocol.ResolveEngine(protocol.Member{Harness: f.harnessID, Model: f.modelID, Options: f.values}, snapshot.Settings, fallback)
}

// the harness Agents starts, by name; the teams defaults fall back to it
func AgentsHarnessName(snapshot state.Snapshot) string {
	return harnessName(snapshot.Harnesses, agentsHarness(snapshot))
}

// the harness a member with no harness of its own runs on, by name
func DefaultHarnessName(snapshot state.Snapshot) string {
	harness, _, _ := protocol.ResolveEngine(protocol.Member{}, snapshot.Settings, registryDefault(snapshot.Harnesses))
	return harnessName(snapshot.Harnesses, harness)
}

func agentsHarness(snapshot state.Snapshot) string {
	if snapshot.Settings.Agent.Harness != "" {
		return snapshot.Settings.Agent.Harness
	}
	return registryDefault(snapshot.Harnesses)
}

func harnessName(harnesses []protocol.HarnessInfo, id string) string {
	if id == "" {
		return i18n.T("no harness installed")
	}
	if info, ok := harnessByID(harnesses, id); ok {
		return info.Name
	}
	return id
}

// the harness the first choices run on, the model and options they stand for, and where those come from; source is empty for the harness's own
func (f *EngineForm) inherited(snapshot state.Snapshot) (harness, model string, options map[string]string, source string) {
	fallback := registryDefault(snapshot.Harnesses)
	switch {
	case f.Keep != nil:
		harness, model, options = protocol.ResolveEngine(*f.Keep, snapshot.Settings, fallback)
		return harness, model, options, i18n.Tf("Same as %s", f.Keep.Name)
	case f.harnessID != "":
		return f.harnessID, "", nil, ""
	case f.Agents:
		return agentsHarness(snapshot), "", nil, ""
	}
	harness, model, options = protocol.ResolveEngine(protocol.Member{}, snapshot.Settings, fallback)
	return harness, model, options, i18n.T("Teams default")
}

// the harness as the snapshot has it, with what a probe just read while the snapshot is older
func (f *EngineForm) info(snapshot state.Snapshot, id string) protocol.HarnessInfo {
	info, _ := harnessByID(snapshot.Harnesses, id)
	probed := f.probed.Capabilities
	if f.probed.ID == id && probed != nil && (info.Capabilities == nil || info.Capabilities.Learned.Before(probed.Learned)) {
		info.Capabilities = probed
	}
	return info
}

// rows read the form when they are drawn, so a harness picked above already shapes the rows under it
func (f *EngineForm) Layout(gtx layout.Context, e *env.Env, defaultLabel string) layout.Dimensions {
	th := e.Theme
	if f.options == nil {
		f.options = make(map[string]*look.Select)
	}
	var rows []layout.Widget
	if f.Keep == nil {
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return f.harnessField(gtx, e, defaultLabel) })
	}
	rows = append(rows,
		func(gtx layout.Context) layout.Dimensions { return f.modelField(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return f.optionFields(gtx, e) },
		func(gtx layout.Context) layout.Dimensions { return f.checkRow(gtx, e) },
	)
	children := make([]layout.FlexChild, 0, 2*len(rows))
	for _, row := range rows {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			dimensions := row(gtx)
			if dimensions.Size.Y > 0 {
				dimensions.Size.Y += gtx.Dp(th.Sp.S3)
			}
			return dimensions
		}))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (f *EngineForm) harnessField(gtx layout.Context, e *env.Env, defaultLabel string) layout.Dimensions {
	snapshot := e.Snapshot()
	ids := []string{""}
	choices := []look.SelectOption{{Label: defaultLabel, Detail: i18n.T("Follows the default, so it changes when the default does.")}}
	found := f.harnessID == ""
	// a harness that is not installed is listed so it can be seen, but a team cannot start it, so it cannot be picked
	for _, harness := range snapshot.Harnesses {
		ids = append(ids, harness.ID)
		choices = append(choices, look.SelectOption{Label: harness.Name, Detail: harnessState(harness), Disabled: !harness.Installed})
		found = found || harness.ID == f.harnessID
	}
	if !found {
		ids = append(ids, f.harnessID)
		choices = append(choices, look.SelectOption{Label: f.harnessID, Detail: i18n.T("not installed"), Disabled: true})
	}
	if index, changed := f.harness.Changed(); changed && index >= 0 && index < len(ids) && ids[index] != f.harnessID {
		// model ids and options belong to one harness, so another harness starts from its own defaults
		f.harnessID, f.modelID, f.values, f.text, f.typed = ids[index], "", nil, "", false
		f.modelText.SetText("")
		f.result, f.failed, f.changed = "", false, true
	}
	f.harness.Label = i18n.T("Harness")
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return f.harness.LayoutOptions(gtx, e.Theme, e.Overlays, choices, max(0, slices.Index(ids, f.harnessID)))
}

func harnessState(harness protocol.HarnessInfo) string {
	switch {
	case !harness.Installed:
		return i18n.T("not installed")
	case harness.Custom:
		return i18n.T("your own harness, installed")
	case harness.Version != "":
		return i18n.Tf("%s, installed", harness.Version)
	}
	return i18n.T("installed")
}

// the first choice names what an empty model runs: the member's or the teams default when they name one, else the harness's own
func firstModel(info protocol.HarnessInfo, model, source string) string {
	if model != "" && source != "" {
		return i18n.Tf("%s (%s)", source, modelName(info.Capabilities, model))
	}
	if model := defaultModel(info); model != "" {
		return i18n.Tf("Harness default (%s)", modelName(info.Capabilities, model))
	}
	return i18n.T("Harness default")
}

func (f *EngineForm) modelField(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	harness, model, _, source := f.inherited(snapshot)
	info := f.info(snapshot, harness)
	first := firstModel(info, model, source)
	var list []protocol.Choice
	if info.Capabilities != nil {
		list = info.Capabilities.ModelList
	}
	listed := len(list) > 0
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	if listed && !f.typed {
		return f.modelSelect(gtx, e, list, first)
	}
	if text := f.modelText.Text(); text != f.text {
		f.text, f.modelID = text, strings.TrimSpace(text)
		f.result, f.failed, f.changed = "", false, true
	}
	f.modelText.Label, f.modelText.Mono = i18n.T("Model"), true
	f.modelText.Placeholder = i18n.Tf("Model id, empty for %s", first)
	f.modelText.Error = ""
	if !protocol.ValidEngineModel(f.modelID) {
		f.modelText.Error = i18n.T("A model id has no spaces.")
	}
	f.read.Kind, f.read.Small = kit.PlaqueTonal, true
	f.read.Label, f.read.Busy = i18n.T("Read the list"), f.busy
	if listed {
		f.read.Label, f.read.Busy = i18n.T("Pick from the list"), false
	}
	if f.read.Clicked(gtx) {
		switch {
		case listed:
			f.typed = false
		case !f.busy && info.Installed:
			f.probe(e, protocol.HarnessProbeParams{ID: info.ID}, info.Name, false)
		}
	}
	children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return f.modelText.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !listed && (f.busy || !info.Installed) {
					gtx = gtx.Disabled()
				}
				return f.read.Layout(gtx, th.Theme)
			}),
		)
	})}
	if !listed {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Rashin learns a harness's models the first time it starts. Read the list starts it once, without a prompt."))),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// a model the list no longer shows stays as itself, and the last choice turns the list into a field for one it never showed
func (f *EngineForm) modelSelect(gtx layout.Context, e *env.Env, list []protocol.Choice, first string) layout.Dimensions {
	ids := []string{""}
	choices := []look.SelectOption{{Label: first}}
	found := f.modelID == ""
	for _, choice := range list {
		label := choice.Name
		if label == "" {
			label = choice.ID
		}
		ids = append(ids, choice.ID)
		choices = append(choices, look.SelectOption{Label: label, Detail: choice.Description})
		found = found || choice.ID == f.modelID
	}
	if !found {
		ids = append(ids, f.modelID)
		choices = append(choices, look.SelectOption{Label: f.modelID})
	}
	typeAt := len(ids)
	choices = append(choices, look.SelectOption{Label: i18n.T("Type a model id"), Detail: i18n.T("For a model this list does not show.")})
	if index, changed := f.model.Changed(); changed {
		switch {
		case index == typeAt:
			f.typed = true
		case index >= 0 && index < len(ids) && ids[index] != f.modelID:
			f.modelID, f.text = ids[index], ids[index]
			f.modelText.SetText(ids[index])
			f.result, f.failed, f.changed = "", false, true
		}
	}
	f.model.Label = i18n.T("Model")
	return f.model.LayoutOptions(gtx, e.Theme, e.Overlays, choices, max(0, slices.Index(ids, f.modelID)))
}

// the options a harness offers with choices: how hard it thinks first, then the rest in the harness's order
func catalogOptions(capabilities *protocol.HarnessCapabilities) []protocol.SessionOption {
	if capabilities == nil {
		return nil
	}
	options := make([]protocol.SessionOption, 0, len(capabilities.Options))
	for _, option := range capabilities.Options {
		if len(option.Choices) > 0 {
			options = append(options, option)
		}
	}
	slices.SortStableFunc(options, func(a, b protocol.SessionOption) int {
		return thinking(b) - thinking(a)
	})
	return options
}

func thinking(option protocol.SessionOption) int {
	if option.Category == thoughtLevel {
		return 1
	}
	return 0
}

// a choice's name as the harness gives it, else its id
func ChoiceName(choices []protocol.Choice, id string) string {
	for _, choice := range choices {
		if choice.ID == id && choice.Name != "" {
			return choice.Name
		}
	}
	return id
}

func (f *EngineForm) optionFields(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	harness, _, inherited, source := f.inherited(snapshot)
	options := catalogOptions(f.info(snapshot, harness).Capabilities)
	children := make([]layout.FlexChild, 0, 2*len(options))
	for index, option := range options {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return f.optionField(gtx, e, option, inherited, source)
		}))
	}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (f *EngineForm) optionField(gtx layout.Context, e *env.Env, option protocol.SessionOption, inherited map[string]string, source string) layout.Dimensions {
	control := f.options[option.ID]
	if control == nil {
		control = new(look.Select)
		f.options[option.ID] = control
	}
	word, value := i18n.T("Harness default"), option.Current
	if kept, ok := inherited[option.ID]; ok && source != "" {
		word, value = source, kept
	}
	first := word
	if name := ChoiceName(option.Choices, value); name != "" {
		first = i18n.Tf("%s (%s)", word, name)
	}
	ids := []string{""}
	choices := []look.SelectOption{{Label: first}}
	current := f.values[option.ID]
	found := current == ""
	for _, choice := range option.Choices {
		ids = append(ids, choice.ID)
		choices = append(choices, look.SelectOption{Label: ChoiceName(option.Choices, choice.ID), Detail: choice.Description})
		found = found || choice.ID == current
	}
	if !found {
		ids = append(ids, current)
		choices = append(choices, look.SelectOption{Label: current})
	}
	if index, changed := control.Changed(); changed && index >= 0 && index < len(ids) && ids[index] != current {
		if f.values == nil {
			f.values = make(map[string]string)
		}
		if ids[index] == "" {
			delete(f.values, option.ID)
		} else {
			f.values[option.ID] = ids[index]
		}
		current = ids[index]
		f.result, f.failed, f.changed = "", false, true
	}
	control.Label = option.Name
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return control.LayoutOptions(gtx, e.Theme, e.Overlays, choices, max(0, slices.Index(ids, current)))
}

// Check starts the harness once in the sandbox with what a task would ask of it, so a refusal shows here and not on a task
func (f *EngineForm) checkRow(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	harness, model, options := f.Effective(snapshot)
	info, known := harnessByID(snapshot.Harnesses, harness)
	ready := known && info.Installed && !f.busy
	f.check.Kind, f.check.Small, f.check.Label, f.check.Busy = kit.PlaqueTonal, true, i18n.T("Check"), f.busy
	if f.check.Clicked(gtx) && ready {
		f.probe(e, protocol.HarnessProbeParams{ID: harness, Model: model, Options: options}, info.Name, true)
	}
	line, style, ink := i18n.T("Starts the harness once in a sandbox, as a task would, without a prompt."), th.T.Caption, th.R.Fg3
	switch {
	case f.result != "" && f.failed:
		line, style, ink = f.result, th.T.Detail, th.R.Jasper
	case f.result != "":
		line, style, ink = f.result, th.T.Detail, th.R.Fg2
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return f.check.Layout(gtx, th.Theme)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Flexed(1, look.Paragraph(th, style, ink, line)),
	)
}

// one probe at a time; checking says what the harness took, reading only says when it offered no list
func (f *EngineForm) probe(e *env.Env, params protocol.HarnessProbeParams, name string, checking bool) {
	f.busy, f.result, f.failed = true, "", false
	var result protocol.HarnessProbeResult
	e.Call(protocol.MHarnessesProbe, params, &result, func(err error) {
		f.busy = false
		switch {
		case err != nil:
			f.result, f.failed = err.Error(), true
			return
		case !result.OK:
			f.result, f.failed = result.Message, true
			if f.result == "" {
				f.result = i18n.Tf("%s did not start.", name)
			}
		}
		f.probed = result.Harness
		if !result.OK {
			return
		}
		capabilities := result.Harness.Capabilities
		switch {
		case checking:
			model := params.Model
			if model == "" {
				model = defaultModel(result.Harness)
			}
			if model == "" {
				f.result = i18n.Tf("%s started in the sandbox and took its default model.", name)
			} else {
				f.result = i18n.Tf("%s started in the sandbox and took %s.", name, modelName(capabilities, model))
			}
		case capabilities == nil || len(capabilities.ModelList) == 0:
			f.result = i18n.Tf("%s started but offered no list of models. Type the model id.", name)
		}
	})
}
