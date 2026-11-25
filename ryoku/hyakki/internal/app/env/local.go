package env

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// what stands between a question and a model Hyakki runs itself, worst first so pages can offer the one step that fixes it
type LocalStanding uint8

const (
	// no model is installed: offer a download
	LocalNothing LocalStanding = iota
	// a model is installed and no llama.cpp can run it
	LocalNoEngine
	// it starts when a question needs it, which is rest and not a fault
	LocalResting
	LocalStarting
	LocalStopping
	LocalReady
	LocalFailed
)

// the model is the one in memory, else the first installed; the zero model with LocalNothing
func LocalStand(local protocol.LocalResult) (LocalStanding, protocol.LocalModel) {
	var model protocol.LocalModel
	found := false
	for _, candidate := range local.Models {
		if candidate.State != protocol.LocalInstalled {
			continue
		}
		loaded := false
		for _, id := range local.Runtime.Loaded {
			loaded = loaded || id == candidate.ID
		}
		if !found || loaded {
			model, found = candidate, true
		}
		if loaded {
			break
		}
	}
	if !found {
		return LocalNothing, model
	}
	switch local.Runtime.State {
	case protocol.RuntimeMissing:
		return LocalNoEngine, model
	case protocol.RuntimeStarting:
		return LocalStarting, model
	case protocol.RuntimeStopping:
		return LocalStopping, model
	case protocol.RuntimeReady:
		return LocalReady, model
	case protocol.RuntimeFailed:
		return LocalFailed, model
	}
	return LocalResting, model
}

// what a local model is routed as
func LocalRoute(id string) string { return protocol.ProviderRashinModels + "/" + id }

// how much of a model is on disk and how much it is in all, over its files
func LocalBytes(model protocol.LocalModel) (done, total int64) {
	for _, file := range model.Files {
		done += min(file.Done, file.Size)
		total += file.Size
	}
	return done, total
}

// decimal units, as download pages and file managers count: 1.39 GB, 292 MB
func LocalSize(bytes int64) string {
	switch {
	case bytes >= 1_000_000_000:
		return i18n.Tf("%s GB", strconv.FormatFloat(float64(bytes)/1e9, 'f', 2, 64))
	case bytes >= 1_000_000:
		return i18n.Tf("%d MB", (bytes+500_000)/1_000_000)
	case bytes >= 1_000:
		return i18n.Tf("%d KB", (bytes+500)/1_000)
	}
	return i18n.Tf("%d bytes", bytes)
}

// 32768 reads as 32K, 131072 as 131K, 1048576 as 1M; a tiny model's 512 stays 512, never 0K
func ShortContext(tokens int) string {
	if tokens >= 1_000_000 {
		return i18n.Tf("%sM", strconv.FormatFloat(float64((tokens+50_000)/100_000)/10, 'f', -1, 64))
	}
	if tokens < 1000 {
		return strconv.Itoa(tokens)
	}
	return i18n.Tf("%dK", tokens/1000)
}

// memory is counted the way a graphics card states it: 2.1 GB, 780 MB
func LocalMemory(mib int) string {
	if mib >= 1024 {
		return i18n.Tf("%s GB", strconv.FormatFloat(float64(mib)/1024, 'f', 1, 64))
	}
	return i18n.Tf("%d MB", mib)
}

// how a model runs here: what it was loaded with while it is in memory, else the fresh estimate;
// false before the hardware was read
func LocalPlanFor(local protocol.LocalResult, model protocol.LocalModel) (protocol.LocalPlan, bool) {
	planned := func(plan protocol.LocalPlan) bool { return plan.Context > 0 || plan.Kind != "" }
	plan := model.Plan
	if loaded := local.Runtime.Plans[model.ID]; planned(loaded) && (slices.Contains(local.Runtime.Loaded, model.ID) || !planned(plan)) {
		// the fresh estimate knows which contexts fit now, the load only what it took
		if len(plan.ContextOptions) > 0 {
			loaded.ContextOptions = plan.ContextOptions
		}
		plan = loaded
	}
	return plan, planned(plan)
}

// the context a model answers with: the one llama.cpp reported once loaded, else the planned one
func LocalPlanContext(plan protocol.LocalPlan) int {
	if plan.ActualContext > 0 {
		return plan.ActualContext
	}
	return plan.Context
}

// what the add field holds, made ready for the daemon: a file address, a quoted path or one under ~
// becomes the full path, and file says so; a repository or a Hugging Face address goes as typed
func LocalSource(typed string) (source string, file bool) {
	source = strings.Trim(strings.TrimSpace(typed), `"'`)
	if strings.HasPrefix(source, "file://") {
		if parsed, err := url.Parse(source); err == nil && parsed.Path != "" {
			source = parsed.Path
		}
	}
	if source == "~" || strings.HasPrefix(source, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			source = filepath.Join(home, source[1:])
		}
	}
	return source, filepath.IsAbs(source)
}

// a path under the home folder reads as ~/Models, the way a file manager says it
func LocalPlace(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || home == "/" {
		return path
	}
	if path == home {
		return "~"
	}
	if rest, under := strings.CutPrefix(path, home+"/"); under {
		return "~/" + rest
	}
	return path
}

// one model file from the system chooser; unavailable when none answers, so the path can be typed
func (e *Env) ChooseModelFile(chosen func(path string), unavailable func()) {
	e.choose(desktop.ChooseRequest{Title: i18n.T("Choose a GGUF model file")}, func(paths []string) { chosen(paths[0]) }, unavailable)
}

// the user's own llama-server from the system chooser, opened in start when that is a folder
func (e *Env) ChooseLlamaServer(start string, chosen func(path string), unavailable func()) {
	e.choose(desktop.ChooseRequest{Title: i18n.T("Choose llama-server"), Start: start}, func(paths []string) { chosen(paths[0]) }, unavailable)
}

// done nil tells a refusal at the foot of the pane
func (e *Env) localCall(method string, params, result any, done func(error)) {
	if done == nil {
		done = e.Report
	}
	e.Call(method, params, result, done)
}

// LocalDevices probes the engine's devices only when the model manager opens
func (e *Env) LocalDevices(done func([]protocol.LocalDevice, error)) {
	var result protocol.LocalDevicesResult
	e.Call(protocol.MLocalDevices, protocol.Empty{}, &result, func(err error) {
		done(result.Devices, err)
	})
}

// LocalRefresh reads the models, the runtime and every plan again
func (e *Env) LocalRefresh(done func(error)) {
	var result protocol.LocalResult
	e.Call(protocol.MLocalModels, protocol.Empty{}, &result, func(err error) {
		if err == nil {
			e.Store.SetLocal(result)
		}
		done(err)
	})
}

// LocalOverride stores what the user set for one model, and the zero override goes back to automatic;
// a settings change announces no new plan, so the plans are read again before done hears of it
func (e *Env) LocalOverride(id string, override protocol.LocalModelOverride, done func(error)) {
	var entry any
	if override != (protocol.LocalModelOverride{}) {
		// null drops a field the user handed back, where a merge would keep its old value
		fields := map[string]any{"device": nil, "context": nil, "kvType": nil}
		if override.Device != "" {
			fields["device"] = override.Device
		}
		if override.Context > 0 {
			fields["context"] = override.Context
		}
		if override.KVType != "" {
			fields["kvType"] = override.KVType
		}
		entry = fields
	}
	patch := map[string]any{"providers": map[string]any{"models": map[string]any{"overrides": map[string]any{id: entry}}}}
	e.SetSettings(patch, func(err error) {
		if err != nil {
			done(err)
			return
		}
		e.LocalRefresh(done)
	})
}

// id may be empty: the server starts and loads a model when the first question names one
func (e *Env) LocalStart(id string, done func(error)) {
	var runtime protocol.LocalRuntime
	e.localCall(protocol.MLocalStart, protocol.LocalModelParams{ID: id}, &runtime, e.keepRuntime(&runtime, done))
}

func (e *Env) LocalStop(done func(error)) {
	var runtime protocol.LocalRuntime
	e.localCall(protocol.MLocalStop, protocol.LocalModelParams{}, &runtime, e.keepRuntime(&runtime, func(err error) {
		if err == nil && e.Notify != nil {
			e.Notify(i18n.T("Stopped. No model runs on this computer now."))
		}
		if done != nil {
			done(err)
		} else {
			e.Report(err)
		}
	}))
}

// the answer is shown at once, the local.changed that follows says the same
func (e *Env) keepRuntime(runtime *protocol.LocalRuntime, done func(error)) func(error) {
	return func(err error) {
		if err == nil {
			e.Store.SetLocalRuntime(*runtime)
		}
		if done != nil {
			done(err)
		} else {
			e.Report(err)
		}
	}
}

// starts a download or picks a paused one up where it stopped
func (e *Env) LocalDownload(id string, done func(error)) {
	e.localCall(protocol.MLocalDownload, protocol.LocalModelParams{ID: id}, new(protocol.Empty), done)
}

// keeps what was fetched, so Resume goes on from there
func (e *Env) LocalPause(id string, done func(error)) {
	e.localCall(protocol.MLocalCancel, protocol.LocalModelParams{ID: id}, new(protocol.Empty), done)
}

func (e *Env) LocalRemove(id string, done func(error)) {
	var result protocol.LocalResult
	e.localCall(protocol.MLocalRemove, protocol.LocalModelParams{ID: id}, &result, func(err error) {
		if err == nil {
			e.Store.SetLocal(result)
		}
		if done != nil {
			done(err)
		} else {
			e.Report(err)
		}
	})
}

func (e *Env) LocalInstallRuntime(backend string, done func(error)) {
	e.localCall(protocol.MLocalRuntimeInstall, protocol.LocalRuntimeParams{Backend: backend}, new(protocol.Empty), done)
}

// a refusal belongs beside the field that asked, so done always hears it
func (e *Env) LocalSearch(repo string, done func(protocol.LocalSearchResult, error)) {
	var result protocol.LocalSearchResult
	e.Call(protocol.MLocalSearch, protocol.LocalSearchParams{Repo: repo}, &result, func(err error) { done(result, err) })
}

// how long the app waits while the daemon copies and checks a file of the user's; a 20 GB model on a slow
// disk takes minutes, and the daemon answers only once the whole file is in
const localImportWait = 30 * time.Minute

// a model the user added from a file of their own, which nothing can download again
func LocalFromFile(model protocol.LocalModel) bool { return strings.HasPrefix(model.Repo, "local/") }

// Repo is a repository with File picked from it, or the full path of a .gguf on this computer; done hears
// of the file only when it is copied, checked and installed, or refused
func (e *Env) LocalAdd(params protocol.LocalAddParams, done func(protocol.LocalModel, error)) {
	if !filepath.IsAbs(params.Repo) {
		var model protocol.LocalModel
		e.Call(protocol.MLocalAdd, params, &model, func(err error) { done(model, err) })
		return
	}
	e.localPatient(protocol.MLocalAdd, params, done)
}

// every other call is given up after 45 seconds, long before a large file is copied or read whole
func (e *Env) localPatient(method string, params any, done func(protocol.LocalModel, error)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), localImportWait)
		defer cancel()
		var model protocol.LocalModel
		err := e.Client.Call(ctx, method, params, &model)
		if e.Post != nil {
			e.Post(func() { done(model, err) })
		} else {
			done(model, err)
		}
	}()
}

// a model Hyakki runs from a file that stays where its owner keeps it: removing it deletes nothing
func LocalInPlace(model protocol.LocalModel) bool { return model.Path != "" }

// LocalFound reads which model files other programs and the user's folders already hold; the daemon
// looks only when asked, and only the model manager asks
func (e *Env) LocalFound(done func([]protocol.LocalFoundModel, error)) {
	var result protocol.LocalFoundResult
	e.Call(protocol.MLocalFound, protocol.Empty{}, &result, func(err error) { done(result.Models, err) })
}

// LocalUseFound takes a found file where it lies: nothing is copied, and the file stays its owner's
func (e *Env) LocalUseFound(path string, done func(protocol.LocalModel, error)) {
	e.localPatient(protocol.MLocalUseFound, protocol.LocalUseFoundParams{Path: path}, done)
}

// the rows of the Light work select: Automatic, then every model on this computer that can call tools;
// a stored one nothing serves now is still shown, unpickable, so the setting never reads as something else
func LightChoices(e *Env, current string) (options []look.SelectOption, ids []string, selected int) {
	options = []look.SelectOption{{
		Label:  i18n.T("Automatic"),
		Detail: i18n.T("The smallest model on this computer that can use tools"),
	}}
	ids = []string{""}
	names := make(map[string]string)
	for _, provider := range e.Snapshot().Providers {
		names[provider.ID] = provider.Name
	}
	found := current == ""
	for _, model := range Models(e).Local() {
		if !model.Tools || !model.Available {
			continue
		}
		option := look.SelectOption{
			Label: model.Name, Detail: names[model.Provider],
			Mark: look.Mark{ID: model.Provider, Name: names[model.Provider]},
		}
		if model.Context > 0 {
			option.Detail = look.Clauses(option.Detail, i18n.Tf("%s context", ShortContext(model.Context)))
		}
		if model.Light {
			option.Chips = []string{i18n.T("Light")}
		}
		if model.ID == current {
			found, selected = true, len(options)
		}
		options, ids = append(options, option), append(ids, model.ID)
	}
	if !found {
		selected = len(options)
		options = append(options, look.SelectOption{
			Label: strings.TrimPrefix(current, protocol.ProviderRashinModels+"/"), Detail: i18n.T("Not on this computer now"), Disabled: true,
		})
		ids = append(ids, current)
	}
	return options, ids, selected
}

var nixos struct {
	once sync.Once
	is   bool
}

// Hyakki cannot put a llama-server on NixOS itself, so the manager points at the package option there
func OnNixOS() bool {
	nixos.once.Do(func() {
		data, err := os.ReadFile("/etc/os-release")
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) == "ID=nixos" {
				nixos.is = true
			}
		}
	})
	return nixos.is
}
