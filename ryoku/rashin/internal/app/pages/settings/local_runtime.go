package settings

import (
	"strconv"
	"strings"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type localRuntimePanel struct {
	install, stop, saveBinary kit.Plaque
	binary                    kit.TextField
	binaryFrom                string
	backend, device, light    look.Select
	backendChoice             int
	idle                      kit.Stepper
	remote                    kit.Toggle
	busy                      bool
	stopBusy                  bool
	problem                   string
}

func (r *localRuntimePanel) layout(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	runtime := snapshot.Local.Runtime
	progress, installing := snapshot.LocalProgress[protocol.LocalRuntimeID]
	r.install.Kind, r.install.Small, r.install.Label = kit.PlaqueTonal, true, i18n.T("Get llama.cpp")
	if runtime.State == protocol.RuntimeTooOld {
		r.install.Label = i18n.T("Update llama.cpp")
	}
	r.install.Busy = r.busy || installing && progress.Phase == protocol.LocalPhaseDownload
	if r.install.Clicked(gtx) && !r.busy && !installing {
		backend := protocol.BackendVulkan
		if len(runtime.Offer) > 0 {
			backend = runtime.Offer[min(max(r.backendChoice, 0), len(runtime.Offer)-1)]
		}
		r.busy = true
		e.LocalInstallRuntime(backend, func(err error) {
			r.busy = false
			if err != nil {
				r.problem = err.Error()
			} else {
				r.problem = ""
			}
		})
	}
	stopping := r.stopBusy || runtime.State == protocol.RuntimeStopping
	r.stop.Kind, r.stop.Small, r.stop.Busy = kit.PlaqueTonal, true, stopping
	r.stop.Label = i18n.T("Stop the model")
	if stopping {
		r.stop.Label = i18n.T("Stopping")
	}
	if r.stop.Clicked(gtx) && !stopping {
		r.stopBusy = true
		e.LocalStop(func(err error) {
			r.stopBusy = false
			if err != nil {
				r.problem = err.Error()
			} else {
				r.problem = ""
			}
		})
	}
	runtimeText := runtimeWords(runtime)
	children := []layout.FlexChild{layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg, runtimeText))}
	if runtime.Path != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout), layout.Rigid(kit.RawText(th.Theme, th.T.MonoSm, th.R.Fg2, runtime.Path)))
	}
	if runtime.Error != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, runtime.Error)))
	}
	if env.OnNixOS() && (runtime.State == protocol.RuntimeMissing || runtime.State == protocol.RuntimeTooOld) {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("On NixOS, enable the llama.cpp option in your Rashin package and rebuild your system."))))
	}
	if len(runtime.Offer) > 1 && (runtime.State == protocol.RuntimeMissing || runtime.State == protocol.RuntimeTooOld) {
		choices := make([]string, 0, len(runtime.Offer))
		for _, backend := range runtime.Offer {
			choices = append(choices, backendWords(backend))
		}
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			r.backend.Label = i18n.T("Engine backend")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			d := r.backend.Layout(gtx, th, e.Overlays, choices, r.backendChoice)
			if selected, changed := r.backend.Changed(); changed {
				r.backendChoice = selected
			}
			return d
		}))
	}
	if (runtime.State == protocol.RuntimeMissing || runtime.State == protocol.RuntimeTooOld) && !env.OnNixOS() {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.install.Layout(gtx, th.Theme) }))
	}
	if installing || progress.Phase == protocol.LocalPhaseError {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, progressWords(progress))), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			fraction := -1.0
			if progress.Total > 0 {
				fraction = float64(progress.Done) / float64(progress.Total)
			}
			return kit.BladeProgress{Fraction: fraction, Active: progress.Phase != protocol.LocalPhaseError}.Layout(gtx, th.Theme)
		}))
	}
	if runtime.State == protocol.RuntimeReady || runtime.State == protocol.RuntimeStarting || stopping {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return r.stopPlaque(gtx, e, snapshot)
		}))
	}
	children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return look.SettingsSlab(gtx, th, []look.Setting{r.deviceSetting(e, snapshot)})
	}), layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout), layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return r.binaryField(gtx, e, snapshot.Settings.Providers.Models.Binary)
	}))
	if r.problem != "" {
		children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout), layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Jasper, r.problem)))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (r *localRuntimePanel) stopPlaque(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	runtime := snapshot.Local.Runtime
	name := i18n.T("The local model")
	id := runtime.ActiveModel
	if len(runtime.Loaded) > 0 {
		id = runtime.Loaded[0]
	}
	for _, model := range snapshot.Local.Models {
		if model.ID == id {
			name = model.Name
			break
		}
	}
	detail := i18n.Tf("%s is starting on this computer.", name)
	if runtime.State == protocol.RuntimeReady {
		detail = i18n.Tf("%s is running on this computer.", name)
	}
	if runtime.State == protocol.RuntimeStopping || r.stopBusy {
		detail = i18n.Tf("Stopping %s and releasing its memory.", name)
	}
	if runtime.LoadedMiB > 0 && runtime.State == protocol.RuntimeReady {
		detail = i18n.Tf("%s · %d MB in memory", detail, runtime.LoadedMiB)
	}
	return kit.OutlinedSlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("Model on this computer"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, detail)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.stop.Layout(gtx, th.Theme) }),
		}
		if runtime.State == protocol.RuntimeStopping || r.stopBusy {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kit.Working{Active: true}.Layout(gtx, th.Theme) }))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func runtimeWords(runtime protocol.LocalRuntime) string {
	if runtime.State == protocol.RuntimeMissing {
		return i18n.T("llama.cpp is not installed yet")
	}
	if runtime.State == protocol.RuntimeTooOld {
		return i18n.Tf("llama.cpp build %d is too old. Build %d or newer is needed.", runtime.Build, runtime.MinBuild)
	}
	kind := i18n.T("Your llama.cpp")
	switch runtime.Source {
	case protocol.RuntimeManaged:
		kind = i18n.T("Rashin's llama.cpp")
	case protocol.RuntimeSystem:
		kind = i18n.T("System llama.cpp")
	}
	version := runtime.Version
	if version == "" && runtime.Build != 0 {
		version = "b" + strconv.Itoa(runtime.Build)
	}
	if version != "" {
		kind += " · " + version
	}
	if runtime.Backend != "" {
		kind += " · " + backendWords(runtime.Backend)
	}
	switch runtime.State {
	case protocol.RuntimeReady:
		return kind + " · " + i18n.T("Running")
	case protocol.RuntimeStarting:
		return kind + " · " + i18n.T("Starting")
	case protocol.RuntimeStopping:
		return kind + " · " + i18n.T("Stopping")
	case protocol.RuntimeFailed:
		return kind + " · " + i18n.T("Could not start")
	default:
		return kind + " · " + i18n.T("Starts when needed")
	}
}

func backendWords(backend string) string {
	switch backend {
	case protocol.BackendCPU:
		return i18n.T("Processor")
	case protocol.BackendVulkan:
		return i18n.T("Graphics card (Vulkan)")
	case protocol.BackendCUDA:
		return i18n.T("Graphics card (CUDA)")
	default:
		return backend
	}
}

func progressWords(progress protocol.LocalProgress) string {
	switch progress.Phase {
	case protocol.LocalPhaseError:
		return i18n.Tf("Download failed: %s", progress.Error)
	case protocol.LocalPhaseVerify:
		return i18n.T("Checking the downloaded files")
	}
	if progress.Total > 0 {
		return i18n.Tf("Downloading %s of %s", env.LocalSize(progress.Done), env.LocalSize(progress.Total))
	}
	return i18n.T("Preparing the download")
}

func (r *localRuntimePanel) binaryField(gtx layout.Context, e *env.Env, binary string) layout.Dimensions {
	th := e.Theme
	if binary != r.binaryFrom {
		r.binary.SetText(binary)
		r.binaryFrom = binary
	}
	r.binary.Label, r.binary.Placeholder, r.binary.Mono = i18n.T("Your llama-server binary"), i18n.T("Managed runtime, then llama-server on PATH"), true
	r.binary.Editor.SingleLine, r.binary.Editor.Submit = true, true
	r.saveBinary.Kind, r.saveBinary.Small, r.saveBinary.Label = kit.PlaqueTonal, true, i18n.T("Use this binary")
	submitted := false
	for {
		event, ok := r.binary.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	changed := strings.TrimSpace(r.binary.Text()) != binary
	if (r.saveBinary.Clicked(gtx) || submitted) && changed && !r.saveBinary.Busy {
		r.saveBinary.Busy = true
		value := strings.TrimSpace(r.binary.Text())
		setLocalOption(e, "binary", value, func(err error) {
			r.saveBinary.Busy = false
			if err != nil {
				r.binary.Error = err.Error()
			} else {
				r.binary.Error = ""
				r.binaryFrom = ""
			}
		})
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return r.binary.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !changed {
				gtx = gtx.Disabled()
			}
			return r.saveBinary.Layout(gtx, th.Theme)
		}),
	)
}

// where every model runs unless its own Settings say otherwise; it belongs to the engine, not to routing
func (r *localRuntimePanel) deviceSetting(e *env.Env, snapshot state.Snapshot) look.Setting {
	th := e.Theme
	return look.Setting{
		Title:  i18n.T("Default device for all models"),
		Detail: i18n.T("Automatic picks the device each model fits on. Choose the processor or one device to keep every model there. A model's own Settings win."),
		Control: func(gtx layout.Context) layout.Dimensions {
			choices, ids, chosen := deviceChoices(snapshot.Local.Runtime, snapshot.Settings.Providers.Models.Device)
			r.device.Label = i18n.T("Device")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			d := r.device.LayoutOptions(gtx, th, e.Overlays, choices, chosen)
			if at, changed := r.device.Changed(); changed && at < len(ids) {
				setLocalOption(e, "device", ids[at], func(err error) {
					e.Report(err)
					if err == nil {
						// every model's plan follows the device, and a settings change announces none
						e.LocalRefresh(e.Report)
					}
				})
			}
			return d
		},
	}
}

func (r *localRuntimePanel) routing(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	settings := snapshot.Settings.Providers.Models
	r.idle.Min, r.idle.Max, r.idle.Step, r.idle.Unit, r.idle.Editable = 1, protocol.LocalIdleMinutesMax, 1, i18n.T("minutes"), true
	r.idle.Value = settings.IdleMinutes
	r.remote.Value = settings.RemoteFirst
	return look.SettingsSlab(gtx, th, []look.Setting{
		{Title: i18n.T("Light work model"), Detail: i18n.T("Used for titles, memory and quick internal work. Automatic picks the smallest local model that can use tools."), Control: func(gtx layout.Context) layout.Dimensions {
			choices, ids, chosen := env.LightChoices(e, settings.Light)
			r.light.Label = i18n.T("Light work model")
			gtx.Constraints.Min.X = gtx.Dp(selectWidth)
			d := r.light.LayoutOptions(gtx, th, e.Overlays, choices, chosen)
			if at, changed := r.light.Changed(); changed && at < len(ids) {
				setLocalOption(e, "light", ids[at], e.Report)
			}
			return d
		}},
		{Title: i18n.T("Stop after idle"), Detail: i18n.T("The model leaves memory when it has not answered for this many minutes."), Control: func(gtx layout.Context) layout.Dimensions {
			d := look.Named(gtx, i18n.T("Stop after idle"), func(gtx layout.Context) layout.Dimensions { return r.idle.Layout(gtx, th.Theme) })
			if r.idle.Changed() {
				setLocalOption(e, "idleMinutes", r.idle.Value, e.Report)
			}
			return d
		}},
	})
}

func deviceChoices(runtime protocol.LocalRuntime, current string) (choices []look.SelectOption, ids []string, selected int) {
	choices = []look.SelectOption{{Label: i18n.T("Automatic")}, {Label: i18n.T("Processor only")}}
	ids = []string{"", protocol.BackendCPU}
	for _, device := range runtime.Devices {
		label := device.Name
		if label == "" {
			label = device.ID
		}
		option := look.SelectOption{Label: label, Detail: deviceKindWords(device.Kind)}
		if device.TotalMiB > 0 {
			option.Detail = look.Clauses(option.Detail, i18n.Tf("%d MB free of %d MB", device.FreeMiB, device.TotalMiB))
		}
		choices, ids = append(choices, option), append(ids, device.ID)
	}
	found := current == ""
	for at, id := range ids {
		if id == current {
			found, selected = true, at
			break
		}
	}
	if !found {
		selected = len(ids)
		ids = append(ids, current)
		choices = append(choices, look.SelectOption{Label: current, Detail: i18n.T("Device not available now"), Disabled: true})
	}
	return
}

func deviceKindWords(kind string) string {
	switch kind {
	case protocol.LocalDeviceDiscrete:
		return i18n.T("Graphics card")
	case protocol.LocalDeviceIntegrated:
		return i18n.T("Integrated graphics, shares the system's memory")
	}
	return ""
}

func setLocalOption(e *env.Env, name string, value any, done func(error)) {
	e.SetSettings(map[string]any{"providers": map[string]any{"models": map[string]any{name: value}}}, done)
}
