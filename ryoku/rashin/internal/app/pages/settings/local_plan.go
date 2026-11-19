package settings

import (
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the least each override select takes, so three stand in one line of a wide pane and stack in a narrow one
const tuningSelectWidth = 220

// the device a plan runs on, by the name the engine gave it
func planDevice(plan protocol.LocalPlan) string {
	switch {
	case plan.Kind == protocol.LocalDeviceCPU || plan.Device == protocol.BackendCPU:
		return i18n.T("Processor")
	case plan.DeviceName != "":
		return plan.DeviceName
	case plan.Device != "":
		return plan.Device
	case plan.Kind == protocol.LocalDeviceIntegrated:
		return i18n.T("Integrated graphics")
	}
	return i18n.T("Graphics card")
}

// how much of the model sits on the graphics device; nothing to say on the processor or before the layers are counted
func planLayers(plan protocol.LocalPlan) string {
	if plan.Kind == protocol.LocalDeviceCPU || plan.Device == protocol.BackendCPU || plan.Layers <= 0 {
		return ""
	}
	integrated := plan.Kind == protocol.LocalDeviceIntegrated
	switch {
	case plan.GPULayers >= plan.Layers && integrated:
		return i18n.Tf("all %d layers on the graphics", plan.Layers)
	case plan.GPULayers >= plan.Layers:
		return i18n.Tf("all %d layers on the card", plan.Layers)
	case integrated:
		return i18n.Tf("%d of %d layers on the graphics", max(plan.GPULayers, 0), plan.Layers)
	}
	return i18n.Tf("%d of %d layers on the card", max(plan.GPULayers, 0), plan.Layers)
}

// NVIDIA GeForce RTX 4070 · 32K context · all 25 layers on the card · about 2.1 GB
func planWords(plan protocol.LocalPlan) string {
	words := []string{planDevice(plan)}
	if context := env.LocalPlanContext(plan); context > 0 {
		words = append(words, i18n.Tf("%s context", env.ShortContext(context)))
	}
	if layers := planLayers(plan); layers != "" {
		words = append(words, layers)
	}
	if plan.EstimatedMiB > 0 {
		words = append(words, i18n.Tf("about %s", env.LocalMemory(plan.EstimatedMiB)))
	}
	return strings.Join(words, " · ")
}

// what the engine did differently from the plan, when no retry already says why
func planDrift(plan protocol.LocalPlan) string {
	if plan.Retry != "" || plan.ActualContext <= 0 || plan.Context <= 0 || plan.ActualContext == plan.Context {
		return ""
	}
	return i18n.Tf("llama.cpp loaded it with %s context, where Rashin planned %s.", env.ShortContext(plan.ActualContext), env.ShortContext(plan.Context))
}

// the line every installed model carries, with why Rashin chose it and what a load changed
func planBlock(th *look.Theme, plan protocol.LocalPlan) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		lead := look.Paragraph(th, th.T.Detail, th.R.Fg, i18n.Tf("Runs with: %s", planWords(plan)))
		if plan.Verdict == protocol.LocalVerdictTooBig {
			// a plan that cannot hold the model is not something it runs with
			lead = look.Paragraph(th, th.T.Detail, th.R.Jasper, i18n.T("Does not fit this computer's free memory now"))
		}
		children := []layout.FlexChild{layout.Rigid(lead)}
		if plan.Reason != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, plan.Reason)))
		}
		if plan.Retry != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, plan.Retry)))
		}
		if drift := planDrift(plan); drift != "" {
			children = append(children, layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, drift)))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	}
}

// one glance at what a file would do on this computer, the fit first and what stops Rashin using it after
type verdictChip struct {
	Word string
	Tone palette.Tone
}

func verdictWords(verdict string) (string, palette.Tone) {
	switch verdict {
	case protocol.LocalVerdictCard:
		return i18n.T("Fits on the graphics card"), palette.Alive
	case protocol.LocalVerdictShared:
		return i18n.T("Fits in shared graphics memory"), palette.Alive
	case protocol.LocalVerdictSplit:
		return i18n.T("Split between graphics card and memory; slower"), palette.Becoming
	case protocol.LocalVerdictCPU:
		return i18n.T("Runs on the processor, slowly"), palette.Attention
	case protocol.LocalVerdictUnsuitable:
		return i18n.T("Unsuitable for Rashin's everyday tasks"), palette.Failed
	case protocol.LocalVerdictTooBig:
		return i18n.T("Too big for this computer"), palette.Failed
	}
	return "", palette.Idle
}

func fileVerdict(file protocol.LocalFile) string {
	if file.Verdict != "" {
		return file.Verdict
	}
	return file.Plan.Verdict
}

func fileChips(file protocol.LocalFile, recommended bool) []verdictChip {
	chips := make([]verdictChip, 0, 4)
	if fileVerdict(file) == protocol.LocalVerdictUnsuitable {
		word, tone := verdictWords(fileVerdict(file))
		chips = append(chips, verdictChip{word, tone})
	}
	if recommended {
		chips = append(chips, verdictChip{i18n.T("Recommended"), palette.Alive})
	}
	if fileVerdict(file) != protocol.LocalVerdictUnsuitable {
		if word, tone := verdictWords(fileVerdict(file)); word != "" {
			chips = append(chips, verdictChip{word, tone})
		}
	}
	if !file.Tools {
		chips = append(chips, verdictChip{i18n.T("No tool calling"), palette.Attention})
	}
	if file.Context > 0 && file.Context < protocol.LocalMinContext {
		chips = append(chips, verdictChip{i18n.T("Context too small"), palette.Failed})
	}
	return chips
}

// Automatic, then each context the plan says fits; one stored that no longer fits stays, unpickable, so the
// setting never reads as something else
func contextChoices(plan protocol.LocalPlan, current int) (options []look.SelectOption, values []int, selected int) {
	options, values = []look.SelectOption{{Label: i18n.T("Automatic")}}, []int{0}
	found := current == 0
	// the planner lists them as it found them, largest first and one twice when a compact cache reached it
	fits := slices.Clone(plan.ContextOptions)
	slices.Sort(fits)
	for _, context := range slices.Compact(fits) {
		if context <= 0 {
			continue
		}
		if context == current {
			found, selected = true, len(options)
		}
		options, values = append(options, look.SelectOption{Label: env.ShortContext(context)}), append(values, context)
	}
	if !found {
		selected = len(options)
		options = append(options, look.SelectOption{Label: i18n.Tf("%s, does not fit now", env.ShortContext(current)), Disabled: true})
		values = append(values, current)
	}
	return options, values, selected
}

func cacheChoices(current string) (options []look.SelectOption, values []string, selected int) {
	options = []look.SelectOption{
		{Label: i18n.T("Automatic")},
		{Label: i18n.T("Full precision (f16)")},
		{Label: i18n.T("Compact (q8_0)")},
	}
	values = []string{"", protocol.LocalKVF16, protocol.LocalKVQ8}
	for at, value := range values {
		if value == current {
			selected = at
		}
	}
	return options, values, selected
}

// what the user set for one model, as the line under Settings says it
func overrideWords(override protocol.LocalModelOverride, runtime protocol.LocalRuntime) string {
	if override == (protocol.LocalModelOverride{}) {
		return i18n.T("Automatic")
	}
	set := make([]string, 0, 3)
	switch override.Device {
	case "":
	case protocol.BackendCPU:
		set = append(set, i18n.T("Processor only"))
	default:
		name := override.Device
		for _, device := range runtime.Devices {
			if device.ID == override.Device && device.Name != "" {
				name = device.Name
			}
		}
		set = append(set, name)
	}
	if override.Context > 0 {
		set = append(set, i18n.Tf("%s context", env.ShortContext(override.Context)))
	}
	switch override.KVType {
	case protocol.LocalKVF16:
		set = append(set, i18n.T("full precision cache"))
	case protocol.LocalKVQ8:
		set = append(set, i18n.T("compact cache"))
	}
	return i18n.Tf("Set by you: %s", look.Clauses(set...))
}

// the fold under an installed model: Automatic unless the user set a device, a context or a cache type
type localTuning struct {
	fold                foldRow
	open                bool
	device, context, kv look.Select
	reset               kit.Plaque
	busy                bool
	problem             string
}

func (t *localTuning) save(e *env.Env, id string, override protocol.LocalModelOverride) {
	t.busy, t.problem = true, ""
	e.LocalOverride(id, override, func(err error) {
		t.busy = false
		if err != nil {
			t.problem = err.Error()
		}
	})
}

func (t *localTuning) layout(gtx layout.Context, e *env.Env, model protocol.LocalModel, plan protocol.LocalPlan, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	override := snapshot.Settings.Providers.Models.Overrides[model.ID]
	if t.fold.toggled(gtx) {
		t.open = !t.open
	}
	head := foldHead{Title: i18n.T("Settings"), Detail: overrideWords(override, snapshot.Local.Runtime)}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// every model has a Settings fold, so each is named after its model
			return look.Named(gtx, i18n.Tf("%s settings", model.Name), func(gtx layout.Context) layout.Dimensions {
				return t.fold.layoutHead(gtx, th, t.open, head)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			t.fold.unfold.Open = t.open
			return t.fold.unfold.Layout(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: th.Sp.S3, Right: th.Sp.S3, Bottom: th.Sp.S2, Left: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return t.body(gtx, e, model, plan, override, snapshot)
				})
			})
		}),
	)
}

func (t *localTuning) body(gtx layout.Context, e *env.Env, model protocol.LocalModel, plan protocol.LocalPlan, override protocol.LocalModelOverride, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	automatic := override == (protocol.LocalModelOverride{})
	t.reset.Kind, t.reset.Small, t.reset.Label, t.reset.Icon = kit.PlaqueTonal, true, i18n.T("Back to automatic"), look.Icon.Undo
	if t.reset.Clicked(gtx) && !automatic && !t.busy {
		t.save(e, model.ID, protocol.LocalModelOverride{})
	}
	sized := func(gtx layout.Context) layout.Context {
		width := min(gtx.Dp(tuningSelectWidth), gtx.Constraints.Max.X)
		gtx.Constraints.Min.X, gtx.Constraints.Max.X = width, width
		return gtx
	}
	selects := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			choices, ids, chosen := deviceChoices(snapshot.Local.Runtime, override.Device)
			if snapshot.Settings.Providers.Models.Device != "" {
				// with a device set for every model, automatic here means that one
				choices[0].Detail = i18n.T("Uses the default device for all models")
			}
			t.device.Label = i18n.T("Run on")
			d := t.device.LayoutOptions(sized(gtx), th, e.Overlays, choices, chosen)
			if at, changed := t.device.Changed(); changed && at < len(ids) && ids[at] != override.Device {
				// what fits is counted per device, so a context set for the old one goes back to automatic
				t.save(e, model.ID, protocol.LocalModelOverride{Device: ids[at], KVType: override.KVType})
			}
			return d
		},
		func(gtx layout.Context) layout.Dimensions {
			choices, values, chosen := contextChoices(plan, override.Context)
			t.context.Label = i18n.T("Context")
			d := t.context.LayoutOptions(sized(gtx), th, e.Overlays, choices, chosen)
			if at, changed := t.context.Changed(); changed && at < len(values) && values[at] != override.Context {
				next := override
				next.Context = values[at]
				t.save(e, model.ID, next)
			}
			return d
		},
		func(gtx layout.Context) layout.Dimensions {
			choices, values, chosen := cacheChoices(override.KVType)
			t.kv.Label = i18n.T("Context cache")
			d := t.kv.LayoutOptions(sized(gtx), th, e.Overlays, choices, chosen)
			if at, changed := t.kv.Changed(); changed && at < len(values) && values[at] != override.KVType {
				next := override
				next.KVType = values[at]
				t.save(e, model.ID, next)
			}
			return d
		},
	}
	children := []layout.FlexChild{
		layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("Automatic reads this computer's free memory each time the model starts. Context offers only the sizes that fit."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return (look.Flow{Gap: th.Sp.S3, LineGap: th.Sp.S3}).Layout(gtx, len(selects), func(gtx layout.Context, at int) layout.Dimensions {
				return selects[at](gtx)
			})
		}),
	}
	if slicesContains(snapshot.Local.Runtime.Loaded, model.ID) {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("This model is in memory. A change applies the next time it loads."))))
	}
	if t.problem != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, t.problem)))
	}
	children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				// the row stays when nothing is set, so the fold never changes height under the pointer
				if automatic {
					gtx = gtx.Disabled()
				}
				return t.reset.Layout(gtx, th.Theme)
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !t.busy {
					return layout.Dimensions{}
				}
				return kit.Working{Active: true, Size: 16}.Layout(gtx, th.Theme)
			}),
		)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
