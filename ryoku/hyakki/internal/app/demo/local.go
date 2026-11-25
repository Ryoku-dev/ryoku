package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

const (
	demoLocalID = "rashin-models/lfm3-omni-2b"
	demoFreeID  = "openrouter/qwen/qwen3-coder:free"
	demoPaidID  = "anthropic/claude-sonnet-5-5"

	// HYAKKI_DEMO_LOCAL_HARDWARE names the computer the demo pretends to be: unset is the hybrid laptop,
	// igpu, cpu, big and arc are the others
	demoHardwareEnv = "HYAKKI_DEMO_LOCAL_HARDWARE"

	// HYAKKI_DEMO_LOCAL_STATES=1 installs a model for every way a plan can turn out
	demoStatesEnv = "HYAKKI_DEMO_LOCAL_STATES"

	// HYAKKI_DEMO_LOCAL_ENGINE names the llama.cpp the computer holds: unset is Hyakki's own beside the
	// system's and one the user built, found leaves only builds nothing uses until one is chosen, none leaves none
	demoEngineEnv = "HYAKKI_DEMO_LOCAL_ENGINE"

	// HYAKKI_DEMO_LOCAL_FOUND=none is a computer where no other program keeps a model file
	demoFoundEnv = "HYAKKI_DEMO_LOCAL_FOUND"

	demoPartialID  = "qwen3-coder-14b"
	demoRetryID    = "qwq-32b"
	demoImportedID = "local-7c1d0f52a9b3e604"
	demoBrokenID   = "local-19e4b07cd2a85f36"
)

// the computer the demo pretends to be: what llama.cpp would list and the memory beside it
type demoMachine struct {
	devices []protocol.LocalDevice
	ramMiB  int
}

func demoMachineNow() demoMachine {
	radeon := protocol.LocalDevice{ID: "Vulkan0", Name: "AMD Radeon 780M", TotalMiB: 15700, FreeMiB: 9800, Kind: protocol.LocalDeviceIntegrated}
	switch os.Getenv(demoHardwareEnv) {
	case "igpu":
		return demoMachine{devices: []protocol.LocalDevice{radeon}, ramMiB: 15700}
	case "cpu":
		return demoMachine{ramMiB: 7800}
	case "big":
		return demoMachine{ramMiB: 64000, devices: []protocol.LocalDevice{
			{ID: "Vulkan0", Name: "NVIDIA GeForce RTX 4090", TotalMiB: 24564, FreeMiB: 23100, Kind: protocol.LocalDeviceDiscrete},
		}}
	case "arc":
		return demoMachine{ramMiB: 31800, devices: []protocol.LocalDevice{
			{ID: "Vulkan0", Name: "Intel Arc A770", TotalMiB: 16288, FreeMiB: 15200, Kind: protocol.LocalDeviceDiscrete},
		}}
	}
	radeon.ID = "Vulkan1"
	return demoMachine{ramMiB: 31800, devices: []protocol.LocalDevice{
		{ID: "Vulkan0", Name: "NVIDIA GeForce RTX 4070", TotalMiB: 8188, FreeMiB: 6100, Kind: protocol.LocalDeviceDiscrete},
		radeon,
	}}
}

// demoPlan stands in for the daemon's planner and keeps its order of preference: a card that holds the
// whole model, shared graphics memory that leaves the system room, part of the model on a card, then
// the processor. device is the setting for every model, override the one for this model.
func demoPlan(machine demoMachine, bytes, params int64, trained int, device string, override protocol.LocalModelOverride) protocol.LocalPlan {
	layers := 24
	switch {
	case params >= 30_000_000_000:
		layers = 64
	case params >= 12_000_000_000:
		layers = 40
	case params >= 5_000_000_000:
		layers = 32
	}
	weights := max(int(bytes>>20), 1)
	// MiB of full precision cache per 1024 tokens
	perK := layers * 4
	cache := func(tokens int, kind string) int {
		size := perK * tokens / 1024
		if kind == protocol.LocalKVQ8 {
			size = (size*34 + 63) / 64
		}
		return size
	}
	if trained <= 0 {
		trained = protocol.LocalMinContext
	}
	floor := min(protocol.LocalMinContext, trained)
	want := override.Device
	if want == "" {
		want = device
	}
	system := max(machine.ramMiB-max(4096, machine.ramMiB/4), 0)
	room := func(d protocol.LocalDevice) int {
		free := d.FreeMiB - max(768, d.FreeMiB/10)
		if d.Kind == protocol.LocalDeviceIntegrated {
			free = min(free, system)
		}
		return max(free, 0)
	}
	allowed := func(d protocol.LocalDevice) bool {
		return want != protocol.BackendCPU && (want == "" || want == d.ID)
	}
	need := weights + cache(floor, protocol.LocalKVF16) + 256
	plan := protocol.LocalPlan{Device: protocol.BackendCPU, Kind: protocol.LocalDeviceCPU, Layers: layers, KVType: protocol.LocalKVF16}
	budget := system
	for _, kind := range []string{protocol.LocalDeviceDiscrete, protocol.LocalDeviceIntegrated} {
		for _, d := range machine.devices {
			if d.Kind != kind || !allowed(d) || room(d) < need {
				continue
			}
			if plan.Kind == protocol.LocalDeviceCPU || room(d) > budget {
				plan.Device, plan.DeviceName, plan.Kind, plan.GPULayers, budget = d.ID, d.Name, kind, layers, room(d)
			}
		}
		if plan.Kind != protocol.LocalDeviceCPU {
			break
		}
	}
	if plan.Kind == protocol.LocalDeviceCPU && system >= need {
		for _, d := range machine.devices {
			spare := room(d) - cache(floor, protocol.LocalKVF16) - 256
			if d.Kind != protocol.LocalDeviceDiscrete || !allowed(d) || spare < 1024 {
				continue
			}
			plan.Device, plan.DeviceName, plan.Kind = d.ID, d.Name, protocol.LocalDeviceDiscrete
			plan.GPULayers = max(1, min(layers-1, layers*spare/weights))
			break
		}
	}
	if override.KVType != "" {
		plan.KVType = override.KVType
	}
	spare := budget - weights - 256
	sizes := []int{8192, 16384, 32768, 65536}
	if trained < protocol.LocalMinContext {
		sizes = []int{trained}
	}
	for _, tokens := range sizes {
		if tokens <= trained && cache(tokens, plan.KVType) <= spare {
			plan.ContextOptions = append(plan.ContextOptions, tokens)
			plan.Context = tokens
		}
	}
	compact := false
	if override.KVType == "" {
		for _, tokens := range sizes {
			if tokens <= trained && tokens > plan.Context && cache(tokens, protocol.LocalKVQ8) <= spare {
				plan.KVType, plan.Context, compact = protocol.LocalKVQ8, tokens, true
				plan.ContextOptions = append(plan.ContextOptions, tokens)
				break
			}
		}
	}
	if override.Context > 0 && slices.Contains(plan.ContextOptions, override.Context) {
		plan.Context = override.Context
	}
	switch {
	case plan.Context == 0:
		plan.Verdict, plan.Context, plan.GPULayers = protocol.LocalVerdictTooBig, floor, 0
		plan.Reason = i18n.T("There is not enough free memory for this model and a usable context.")
	case plan.Kind == protocol.LocalDeviceCPU:
		plan.Verdict = protocol.LocalVerdictCPU
		switch {
		case want == protocol.BackendCPU:
			plan.Reason = i18n.T("Set to run on the processor.")
		case len(machine.devices) == 0:
			plan.Reason = i18n.T("This computer has no graphics card llama.cpp can use, so the processor runs it.")
		default:
			plan.Reason = i18n.T("No graphics card has enough free memory, so the processor runs it.")
		}
	case plan.GPULayers < layers:
		plan.Verdict = protocol.LocalVerdictSplit
		plan.Reason = i18n.T("The graphics card holds part of the model and its context; the remaining layers run from system memory, which is slower.")
	case plan.Kind == protocol.LocalDeviceIntegrated:
		plan.Verdict = protocol.LocalVerdictShared
		plan.Reason = i18n.T("Shared graphics memory holds the model and leaves the system enough.")
	default:
		plan.Verdict = protocol.LocalVerdictCard
		plan.Reason = i18n.T("The graphics card holds the whole model and its context.")
	}
	if plan.Verdict != protocol.LocalVerdictTooBig && (trained < protocol.LocalMinContext || params < 500_000_000) {
		plan.Verdict = protocol.LocalVerdictUnsuitable
		plan.Reason = i18n.T("This model has too little capacity or context for Hyakki's everyday tasks; plain chat is still possible.")
	}
	if plan.Verdict != protocol.LocalVerdictTooBig {
		if compact {
			plan.Reason += " " + i18n.T("An 8-bit cache makes the longer context fit.")
		}
		if override != (protocol.LocalModelOverride{}) {
			plan.Reason = i18n.T("Using your settings within the memory that is free.")
		}
	}
	plan.EstimatedMiB = weights + cache(plan.Context, plan.KVType) + 256
	return plan
}

func demoModelBytes(model protocol.LocalModel) (total int64) {
	for _, file := range model.Files {
		total += file.Size
	}
	return total
}

// every installed model gets a fresh plan; one in memory keeps the plan it was loaded with beside it
func demoPlanLocal(local *protocol.LocalResult, settings protocol.LocalModelsSettings) {
	machine := demoMachineNow()
	loaded := make(map[string]protocol.LocalPlan, len(local.Runtime.Loaded))
	for index := range local.Models {
		model := &local.Models[index]
		if model.State != protocol.LocalInstalled {
			model.Plan = protocol.LocalPlan{}
			continue
		}
		model.Plan = demoPlan(machine, demoModelBytes(*model), model.Params, model.Context, settings.Device, settings.Overrides[model.ID])
		if !slices.Contains(local.Runtime.Loaded, model.ID) {
			continue
		}
		running, kept := local.Runtime.Plans[model.ID]
		if !kept {
			running = model.Plan
			running.ActualContext = running.Context
			if model.ID == demoRetryID && running.Context > protocol.LocalMinContext {
				// the one fixture whose first load fails, so the retry line has something to say
				running.ActualContext = running.Context / 2
				running.Retry = i18n.Tf("The first load ran out of memory at %dK context, so Hyakki loaded it again at %dK.",
					running.Context/1000, running.ActualContext/1000)
			}
		}
		loaded[model.ID] = running
	}
	local.Runtime.Plans = nil
	if len(loaded) > 0 {
		local.Runtime.Plans = loaded
	}
}

func demoLocalResult() protocol.LocalResult {
	installed := protocol.LocalModel{
		ID: "lfm3-omni-2b", Name: "LFM3 Omni 2B", Repo: "LiquidAI/LFM3-Omni-2B-GGUF",
		Revision: "d508b88f08a5b4286f2d2805a6a8d69a0ff44c72", Class: protocol.LocalClassLight,
		Params: 2_000_000_000, Context: 32768, Tools: true, Vision: true, Audio: true,
		Catalogue: true, State: protocol.LocalInstalled,
		Files: []protocol.LocalFile{
			{Name: "LFM3-Omni-2B-Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 1_100_000_000, Done: 1_100_000_000, Quant: "Q4_K_M"},
			{Name: "mmproj-LFM3-Omni-2B.gguf", Role: protocol.LocalFileMmproj, Size: 292_000_000, Done: 292_000_000},
		},
		License: protocol.LocalLicense{Name: "LFM Open License v1.0", URL: "https://www.liquid.ai/lfm-license", Note: i18n.T("Commercial use only below US$10M annual revenue.")},
	}
	available := protocol.LocalModel{
		ID: "qwen3-coder-7b", Name: "Qwen3 Coder 7B", Repo: "Qwen/Qwen3-Coder-7B-GGUF", Revision: "demo-qwen-revision",
		Class: protocol.LocalClassMain, Params: 7_000_000_000, Context: 131072, Tools: true, Catalogue: true,
		State: protocol.LocalAvailable, Files: []protocol.LocalFile{{Name: "Qwen3-Coder-7B-Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 4_700_000_000, Quant: "Q4_K_M"}},
		License: protocol.LocalLicense{Name: "Apache 2.0", URL: "https://www.apache.org/licenses/LICENSE-2.0"},
	}
	partial := protocol.LocalModel{
		ID: "gemma-3-4b-it", Name: "Gemma 3 4B", Repo: "ggml-org/gemma-3-4b-it-GGUF", Revision: "demo-gemma-revision",
		Class: protocol.LocalClassLight, Params: 4_000_000_000, Context: 32768, Tools: true, Vision: true,
		State: protocol.LocalPaused, Files: []protocol.LocalFile{{Name: "gemma-3-4b-it-Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 2_700_000_000, Done: 1_080_000_000, Quant: "Q4_K_M"}},
		License: protocol.LocalLicense{Name: "Gemma Terms of Use", URL: "https://ai.google.dev/gemma/terms"},
	}
	machine := demoMachineNow()
	result := protocol.LocalResult{
		Models: []protocol.LocalModel{installed, available, partial},
		Runtime: protocol.LocalRuntime{
			State:    protocol.RuntimeMissing,
			Devices:  machine.devices,
			Offer:    []string{protocol.BackendVulkan, protocol.BackendCPU},
			Binaries: demoBuilds(len(machine.devices) > 0),
		},
	}
	demoEngine(&result.Runtime, "")
	if os.Getenv(demoStatesEnv) == "1" {
		result.Models = append(result.Models, demoLocalStates()...)
	}
	for _, model := range result.Models {
		for _, file := range model.Files {
			result.DiskBytes += file.Done
		}
	}
	if os.Getenv("HYAKKI_DEMO_LOCAL_RUNNING") == "1" {
		result.Runtime.State, result.Runtime.Address = protocol.RuntimeReady, "http://127.0.0.1:41873/v1"
		result.Runtime.Loaded, result.Runtime.ActiveModel = []string{"lfm3-omni-2b"}, "lfm3-omni-2b"
	}
	demoPlanLocal(&result, protocol.LocalModelsSettings{})
	if plan, running := result.Runtime.Plans["lfm3-omni-2b"]; running {
		result.Runtime.LoadedMiB = plan.EstimatedMiB
	}
	return result
}

func demoManagedBuild(backend string) protocol.LocalRuntimeBinary {
	return protocol.LocalRuntimeBinary{
		Path:   filepath.Join(paths.LlamaRuntimeDir(), "llama-b12521", "llama-server"),
		Source: protocol.RuntimeManaged, Version: "b12521", Build: 12521, Backend: backend, Router: true,
	}
}

// the llama.cpp builds on the demo's computer: Hyakki's own, the distribution's and one the user built,
// which names the folder it lies in as its source the way the daemon does
func demoBuilds(graphics bool) []protocol.LocalRuntimeBinary {
	backend := protocol.BackendVulkan
	if !graphics {
		backend = protocol.BackendCPU
	}
	if os.Getenv(demoEngineEnv) == "none" {
		return nil
	}
	builds := []protocol.LocalRuntimeBinary{{
		Path:   filepath.Join(paths.Home(), "llama.cpp", "build", "bin", "llama-server"),
		Source: "~/llama.cpp/build/bin", Version: "b12610", Build: 12610, Backend: protocol.BackendCUDA, Router: true,
	}}
	if os.Getenv(demoStatesEnv) == "1" {
		// an old distribution build that knows no router, and one that would not answer the probe
		builds = append(builds,
			protocol.LocalRuntimeBinary{Path: "/opt/llama.cpp/bin/llama-server", Source: "/opt/llama.cpp/bin", Version: "0 (unknown)", Backend: protocol.BackendCPU},
			protocol.LocalRuntimeBinary{
				Path: filepath.Join(paths.Home(), ".local", "bin", "llama-server"), Source: protocol.RuntimeLogin,
				Broken: i18n.T("It did not answer when Hyakki asked for its version."),
			},
		)
	}
	if os.Getenv(demoEngineEnv) == "found" {
		return builds
	}
	return append([]protocol.LocalRuntimeBinary{
		demoManagedBuild(backend),
		{Path: "/usr/bin/llama-server", Source: protocol.RuntimeSystem, Version: "b12480", Build: 12480, Backend: backend, Router: true},
	}, builds...)
}

// demoEngine settles which build runs the way the daemon does: the one the settings name, else Hyakki's
// own, else the system's. A build found elsewhere runs only once the user chose it. It reports a change.
func demoEngine(runtime *protocol.LocalRuntime, binary string) bool {
	pick := -1
	if binary != "" {
		pick = slices.IndexFunc(runtime.Binaries, func(build protocol.LocalRuntimeBinary) bool { return build.Path == binary })
		if pick < 0 {
			// a file picked in the chooser is probed and joins the builds
			runtime.Binaries = append(runtime.Binaries, protocol.LocalRuntimeBinary{
				Path: binary, Source: protocol.RuntimeConfigured, Version: "b12700", Build: 12700, Backend: protocol.BackendVulkan, Router: true,
			})
			pick = len(runtime.Binaries) - 1
		}
	} else {
		for _, source := range []string{protocol.RuntimeManaged, protocol.RuntimeSystem, protocol.RuntimeLogin} {
			pick = slices.IndexFunc(runtime.Binaries, func(build protocol.LocalRuntimeBinary) bool {
				return build.Source == source && build.Broken == ""
			})
			if pick >= 0 {
				break
			}
		}
	}
	path := ""
	if pick >= 0 {
		path = runtime.Binaries[pick].Path
	}
	if path == runtime.Path {
		return false
	}
	// another build is another server: what the old one held in memory is gone with it
	*runtime = protocol.LocalRuntime{State: protocol.RuntimeMissing, Devices: runtime.Devices, Offer: runtime.Offer, Binaries: runtime.Binaries}
	if pick < 0 {
		return true
	}
	build := runtime.Binaries[pick]
	runtime.Path, runtime.Source, runtime.Version, runtime.Build, runtime.Backend = build.Path, build.Source, build.Version, build.Build, build.Backend
	if binary != "" {
		runtime.Source = protocol.RuntimeConfigured
	}
	// an old build still serves one model plainly, so only a build the probe refused cannot start
	runtime.State = protocol.RuntimeStopped
	if build.Broken != "" {
		runtime.State, runtime.Error = protocol.RuntimeFailed, build.Broken
	}
	return true
}

// what local.found turns up on the demo's computer: files other programs keep, and two more in every
// folder the user added, each judged for this computer the way a file is before it is downloaded
func (s *server) demoFound() []protocol.LocalFoundModel {
	home := paths.Home()
	settings := s.data.snapshot.Settings.Providers.Models
	var found []protocol.LocalFoundModel
	if os.Getenv(demoFoundEnv) != "none" {
		found = []protocol.LocalFoundModel{
			{
				Path: filepath.Join(home, ".ollama", "models", "blobs", "sha256-6e9f90f02bb3b39b59e81916e5cfce9deb45aeaeb9a54a5be4414486b907dc1e"),
				Name: "qwen3:8b", Source: protocol.LocalFoundOllama, Size: 5_200_000_000, Quant: "Q4_K_M", Params: 8_200_000_000, Context: 40960, Tools: true,
			},
			{
				Path: filepath.Join(home, ".lmstudio", "models", "lmstudio-community", "gemma-3-12b-it-GGUF", "gemma-3-12b-it-Q4_K_M.gguf"),
				Name: "Gemma 3 12B", Source: protocol.LocalFoundLMStudio, Size: 7_300_000_000, Quant: "Q4_K_M", Params: 12_200_000_000, Context: 131072, Tools: true, Vision: true,
			},
			{
				Path: filepath.Join(home, ".cache", "huggingface", "hub", "models--bartowski--Llama-3.3-70B-Instruct-GGUF", "snapshots", "4c1f2a9e", "Llama-3.3-70B-Instruct-Q4_K_M.gguf"),
				Name: "Llama 3.3 70B Instruct", Source: protocol.LocalFoundHuggingFace, Size: 42_500_000_000, Quant: "Q4_K_M", Params: 70_600_000_000, Context: 131072, Tools: true,
			},
			{
				Path: filepath.Join(home, ".cache", "llama.cpp", "ggml-org_gemma-3-1b-it-GGUF_gemma-3-1b-it-Q4_K_M.gguf"),
				Name: "Gemma 3 1B", Source: protocol.LocalFoundLlamaCpp, Size: 806_000_000, Quant: "Q4_K_M", Params: 1_000_000_000, Context: 32768,
			},
		}
	}
	for _, folder := range settings.Folders {
		found = append(found,
			protocol.LocalFoundModel{
				Path: filepath.Join(folder, "Mistral-Nemo-Instruct-2407-Q5_K_M.gguf"),
				Name: "Mistral Nemo Instruct 2407", Source: protocol.LocalFoundFolder, Size: 8_700_000_000, Quant: "Q5_K_M", Params: 12_200_000_000, Context: 131072, Tools: true,
			},
			protocol.LocalFoundModel{
				Path: filepath.Join(folder, "tinystories-65m.F16.gguf"),
				Name: "TinyStories 65M", Source: protocol.LocalFoundFolder, Size: 130_000_000, Quant: "F16", Params: 65_000_000, Context: 512, Warning: demoTinyWarning(),
			},
		)
	}
	machine := demoMachineNow()
	for at := range found {
		model := &found[at]
		model.Plan = demoPlan(machine, model.Size, model.Params, model.Context, settings.Device, protocol.LocalModelOverride{})
		for _, known := range s.data.snapshot.Local.Models {
			if known.Path == model.Path {
				model.Model = known.ID
			}
		}
	}
	return found
}

// a found file becomes a model at once: nothing is copied, so there is nothing to wait for, and its path
// is all that ties the model to the file
func (s *server) demoUseFound(path string) (protocol.LocalModel, error) {
	local := &s.data.snapshot.Local
	for _, known := range local.Models {
		if known.Path == path {
			return known, nil
		}
	}
	for _, found := range s.demoFound() {
		if found.Path != path {
			continue
		}
		hash := fnv.New64a()
		hash.Write([]byte(path))
		sum := fmt.Sprintf("%016x", hash.Sum64())
		model := protocol.LocalModel{
			ID: "local-" + sum, Name: found.Name, Repo: "local/" + sum, Revision: sum, Path: path, State: protocol.LocalInstalled,
			Class: protocol.LocalClassLight, Params: found.Params, Context: found.Context, Tools: found.Tools, Vision: found.Vision, Warning: found.Warning,
			Files: []protocol.LocalFile{{Name: filepath.Base(path), Role: protocol.LocalFileModel, Size: found.Size, Done: found.Size, Quant: found.Quant}},
		}
		if model.Params > protocol.LightParams {
			model.Class = protocol.LocalClassMain
		}
		local.Models = append(local.Models, model)
		s.syncLocalModels()
		s.planLocal()
		return local.Models[len(local.Models)-1], nil
	}
	return protocol.LocalModel{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That file is no longer where it was found."))
}

// installed models the stock list has no room for: one too large for most cards, one whose first load
// fails, one that came from a file and cannot answer, and one from a file whose copy went bad
func demoLocalStates() []protocol.LocalModel {
	whole := func(name, quant string, size int64) []protocol.LocalFile {
		return []protocol.LocalFile{{Name: name, Role: protocol.LocalFileModel, Size: size, Done: size, Quant: quant}}
	}
	return []protocol.LocalModel{
		{
			ID: demoPartialID, Name: "Qwen3 Coder 14B", Repo: "Qwen/Qwen3-Coder-14B-GGUF", Revision: "demo-qwen-14b-revision",
			Class: protocol.LocalClassMain, Params: 14_000_000_000, Context: 131072, Tools: true, Reasoning: true,
			State: protocol.LocalInstalled, Files: whole("Qwen3-Coder-14B-Q4_K_M.gguf", "Q4_K_M", 8_900_000_000),
			License: protocol.LocalLicense{Name: "Apache 2.0", URL: "https://www.apache.org/licenses/LICENSE-2.0"},
		},
		{
			ID: demoRetryID, Name: "QwQ 32B", Repo: "Qwen/QwQ-32B-GGUF", Revision: "demo-qwq-revision",
			Class: protocol.LocalClassMain, Params: 32_000_000_000, Context: 131072, Tools: true, Reasoning: true,
			State: protocol.LocalInstalled, Files: whole("qwq-32b-Q4_K_M.gguf", "Q4_K_M", 19_800_000_000),
			License: protocol.LocalLicense{Name: "Apache 2.0", URL: "https://www.apache.org/licenses/LICENSE-2.0"},
		},
		{
			ID: demoImportedID, Name: "tinystories-65m.F16.gguf", Repo: "local/7c1d0f52a9b3e604", Revision: "7c1d0f52a9b3e604",
			Class: protocol.LocalClassLight, Params: 65_000_000, Context: 512,
			State: protocol.LocalInstalled, Files: whole("tinystories-65m.F16.gguf", "F16", 130_000_000),
			Warning: demoTinyWarning(),
		},
		{
			ID: demoBrokenID, Name: "my-finetune.Q4_K_M.gguf", Repo: "local/19e4b07cd2a85f36", Revision: "19e4b07cd2a85f36",
			Class: protocol.LocalClassMain, Params: 7_000_000_000, Context: 32768, Tools: true,
			State: protocol.LocalBroken, Files: []protocol.LocalFile{{Name: "my-finetune.Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 4_700_000_000, Quant: "Q4_K_M"}},
			Error: i18n.T("The copy Hyakki kept of this file no longer matches it."),
		},
	}
}

func demoTinyWarning() string {
	return i18n.T("This model was trained on 512 tokens of context and has no tool calling. Hyakki can't use it to answer or act. Pick a model with tool calling and at least 8K context.")
}

func (s *server) localIndex(id string) int {
	for i := range s.data.snapshot.Local.Models {
		if s.data.snapshot.Local.Models[i].ID == id || protocol.ProviderRashinModels+"/"+s.data.snapshot.Local.Models[i].ID == id {
			return i
		}
	}
	return -1
}

func (s *server) localChanged() {
	s.queueEvent(protocol.EvtLocalChanged, s.data.snapshot.Local)
}

// planLocal plans again from the settings as they stand, as the daemon does when it is asked for the models
func (s *server) planLocal() {
	models := &s.data.snapshot.Settings.Providers.Models
	if models.Overrides == nil && os.Getenv(demoStatesEnv) == "1" {
		// one model the user set by hand, so the fold has a state other than Automatic to show, and one
		// folder the user added, so the list of folders has a row
		models.Overrides = map[string]protocol.LocalModelOverride{demoPartialID: {Device: protocol.BackendCPU, Context: 16384}}
		models.Folders = []string{filepath.Join(paths.Home(), "Models")}
		s.queueEvent(protocol.EvtSettingsChanged, s.data.snapshot.Settings)
	}
	demoPlanLocal(&s.data.snapshot.Local, *models)
}

func (s *server) refreshLocalProvider() {
	for i := range s.data.snapshot.Providers {
		p := &s.data.snapshot.Providers[i]
		if p.ID != protocol.ProviderRashinModels {
			continue
		}
		p.Running = s.data.snapshot.Local.Runtime.State == protocol.RuntimeReady
		p.Models, p.Available = 0, 0
		for _, model := range s.data.snapshot.Local.Models {
			if model.State == protocol.LocalInstalled {
				p.Models++
				p.Available++
			}
		}
		s.queueEvent(protocol.EvtProvidersChanged, protocol.ProvidersResult{Providers: s.data.snapshot.Providers})
		break
	}
}

func (s *server) syncLocalModels() {
	models := make([]protocol.ModelInfo, 0, len(s.data.models.Models))
	for _, local := range s.data.snapshot.Local.Models {
		if local.State != protocol.LocalInstalled {
			continue
		}
		models = append(models, protocol.ModelInfo{
			ID: protocol.ProviderRashinModels + "/" + local.ID, Provider: protocol.ProviderRashinModels,
			Name: local.Name, Tier: protocol.TierLocal, Context: local.Context, Params: local.Params,
			Tools: local.Tools, ToolsKnown: true, Vision: local.Vision, Audio: local.Audio,
			Reasoning: local.Reasoning, Light: local.Class == protocol.LocalClassLight, Available: true,
		})
	}
	for _, model := range s.data.models.Models {
		if model.Provider != protocol.ProviderRashinModels {
			models = append(models, model)
		}
	}
	s.data.models.Models = models
}

// what the add field was given: a file on this computer, or a repository with the one file a link named
func demoSource(source string) (repo, file string, local, ok bool) {
	source = strings.TrimSpace(source)
	if filepath.IsAbs(source) {
		return "", source, true, strings.HasSuffix(strings.ToLower(source), ".gguf")
	}
	rest, link := strings.CutPrefix(source, "https://huggingface.co/")
	if !link {
		return source, "", false, validDemoRepo(source)
	}
	if cut := strings.IndexAny(rest, "?#"); cut >= 0 {
		rest = rest[:cut]
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 {
		return "", "", false, false
	}
	repo = parts[0] + "/" + parts[1]
	// owner/model/blob/<revision>/<file> and owner/model/resolve/<revision>/<file>
	if len(parts) >= 5 && (parts[2] == "blob" || parts[2] == "resolve") {
		file = parts[len(parts)-1]
	}
	return repo, file, false, validDemoRepo(repo)
}

func demoQuant(name string) string {
	upper := strings.ToUpper(name)
	for _, quant := range []string{"Q4_K_M", "Q5_K_M", "Q6_K", "Q8_0", "F16"} {
		if strings.Contains(upper, quant) {
			return quant
		}
	}
	return ""
}

// a file as local.search lists it: read, planned for this computer and given its verdict
func (s *server) demoFile(name string, size, params int64, trained int, tools bool) protocol.LocalFile {
	file := protocol.LocalFile{
		Name: name, Role: protocol.LocalFileModel, Size: size, Params: params, Context: trained, Tools: tools, Quant: demoQuant(name),
	}
	file.Plan = demoPlan(demoMachineNow(), size, params, trained, s.data.snapshot.Settings.Providers.Models.Device, protocol.LocalModelOverride{})
	file.Verdict = file.Plan.Verdict
	return file
}

func (s *server) demoTinyFile(name string) protocol.LocalFile {
	file := s.demoFile(name, 130_000_000, 65_000_000, 512, false)
	file.Warning = demoTinyWarning()
	return file
}

// the quantisations of a 4B model, ranked as the daemon ranks them: what runs whole on graphics with the
// target context first and the best quality of those on top, the rest by how well they fit
func (s *server) demoRepoFiles() (files []protocol.LocalFile, recommended string) {
	const params, trained = 4_000_000_000, 32768
	files = []protocol.LocalFile{
		s.demoFile("model-Q4_K_M.gguf", 2_500_000_000, params, trained, true),
		s.demoFile("model-Q5_K_M.gguf", 2_900_000_000, params, trained, true),
		s.demoFile("model-Q6_K.gguf", 3_300_000_000, params, trained, true),
		s.demoFile("model-Q8_0.gguf", 4_300_000_000, params, trained, true),
		s.demoFile("model-F16-00001-of-00002.gguf", 8_000_000_000, params, trained, true),
	}
	files[4].Parts = 2
	rank := func(file protocol.LocalFile) int {
		short := 0
		if file.Plan.Context < trained {
			short = 2
		}
		switch file.Verdict {
		case protocol.LocalVerdictCard:
			return short
		case protocol.LocalVerdictShared:
			return short + 1
		case protocol.LocalVerdictSplit:
			return 4
		case protocol.LocalVerdictCPU:
			return 5
		}
		return 6
	}
	sort.SliceStable(files, func(i, j int) bool {
		left, right := rank(files[i]), rank(files[j])
		if left != right {
			return left < right
		}
		// of what runs whole on graphics the larger file is the better copy of the model
		if left < 4 {
			return files[i].Size > files[j].Size
		}
		return files[i].Size < files[j].Size
	})
	if files[0].Verdict != protocol.LocalVerdictTooBig {
		files[0].Recommended = true
		recommended = files[0].Name
	}
	// every part of a split set carries its own size and the count of parts; only the first is judged,
	// from the weight of them all
	for index := range files {
		if files[index].Parts > 1 {
			files[index].Size = 4_000_000_000
		}
	}
	files = append(files,
		protocol.LocalFile{Name: "model-F16-00002-of-00002.gguf", Role: protocol.LocalFileModel, Size: 4_000_000_000, Parts: 2},
		protocol.LocalFile{Name: "mmproj-model.gguf", Role: protocol.LocalFileMmproj, Size: 320_000_000},
	)
	return files, recommended
}

func (s *server) demoSearch(source string) (protocol.LocalSearchResult, error) {
	repo, named, local, ok := demoSource(source)
	if !ok {
		return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Enter a Hugging Face repository or link, or the full path of a .gguf file."))
	}
	if local {
		name := filepath.Base(named)
		file := s.demoFile(name, 4_700_000_000, 7_000_000_000, 32768, true)
		if lower := strings.ToLower(name); strings.Contains(lower, "tiny") || strings.Contains(lower, "supra") {
			file = s.demoTinyFile(name)
		}
		file.Recommended = file.Verdict != protocol.LocalVerdictTooBig
		result := protocol.LocalSearchResult{Path: named, Files: []protocol.LocalFile{file}}
		if file.Recommended {
			result.Recommended = name
		}
		return result, nil
	}
	result := protocol.LocalSearchResult{Repo: repo, Revision: "demo-huggingface-revision", License: "Apache 2.0"}
	lower := strings.ToLower(repo)
	if strings.Contains(lower, "gated") {
		result.Gated = true
		return result, nil
	}
	if strings.Contains(lower, "supra") {
		result.Files = []protocol.LocalFile{s.demoTinyFile("Supra2.5-65M.F16.gguf")}
		return result, nil
	}
	result.Files, result.Recommended = s.demoRepoFiles()
	result.Mmproj = "mmproj-model.gguf"
	if named == "" {
		return result, nil
	}
	// a link to one file offers that file alone, from its first part when it named a later one
	set := strings.TrimSuffix(named, ".gguf")
	if cut := strings.Index(named, "-0000"); cut >= 0 {
		set = named[:cut]
		named = set + "-00001-of-00002.gguf"
	}
	kept := make([]protocol.LocalFile, 0, 3)
	result.Recommended = ""
	for _, file := range result.Files {
		part := file.Role == protocol.LocalFileModel && strings.HasPrefix(file.Name, set+"-0000")
		if file.Role != protocol.LocalFileMmproj && file.Name != named && !part {
			continue
		}
		file.Recommended = file.Name == named && file.Verdict != protocol.LocalVerdictTooBig
		if file.Recommended {
			result.Recommended = named
		}
		kept = append(kept, file)
	}
	if !slices.ContainsFunc(kept, func(file protocol.LocalFile) bool { return file.Name == named }) {
		return protocol.LocalSearchResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That file is not in the repository."))
	}
	result.Files, result.Path = kept, named
	return result, nil
}

// a file of the user's is copied and checked before local.add answers, so it comes back installed and
// named by its content; the same file again repairs a broken copy and changes nothing in a sound one
func (s *server) demoImport(found protocol.LocalSearchResult) protocol.LocalModel {
	local := &s.data.snapshot.Local
	file := found.Files[0]
	file.Plan, file.Verdict, file.Recommended, file.Done = protocol.LocalPlan{}, "", false, file.Size
	hash := fnv.New64a()
	hash.Write([]byte(found.Path))
	sum := fmt.Sprintf("%016x", hash.Sum64())
	for _, known := range local.Models {
		// in the demo the same file name stands for the same content
		if strings.HasPrefix(known.Repo, "local/") && known.Name == file.Name {
			sum = strings.TrimPrefix(known.Repo, "local/")
		}
	}
	model := protocol.LocalModel{
		ID: "local-" + sum, Name: file.Name, Repo: "local/" + sum, Revision: sum, State: protocol.LocalInstalled,
		Class: protocol.LocalClassLight, Params: file.Params, Context: file.Context, Tools: file.Tools, Warning: file.Warning,
		Files: []protocol.LocalFile{file},
	}
	if model.Params > protocol.LightParams {
		model.Class = protocol.LocalClassMain
	}
	index := s.localIndex(model.ID)
	if index < 0 {
		index = len(local.Models)
		local.Models = append(local.Models, model)
	} else {
		local.Models[index] = model
	}
	s.localDisk()
	s.syncLocalModels()
	s.planLocal()
	return local.Models[index]
}

// the model local.add answers with: a file of the user's installed, or a pick from a repository with
// nothing fetched yet
func (s *server) demoAdd(p protocol.LocalAddParams) (protocol.LocalModel, error) {
	found, err := s.demoSearch(p.Repo)
	if err != nil {
		return protocol.LocalModel{}, err
	}
	if found.Repo == "" {
		return s.demoImport(found), nil
	}
	picked := p.File
	if !strings.HasSuffix(strings.ToLower(picked), ".gguf") || strings.ContainsAny(picked, "/\\") {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a repository and a GGUF model file."))
	}
	var files []protocol.LocalFile
	set := strings.TrimSuffix(picked, "-00001-of-00002.gguf")
	for _, file := range found.Files {
		if file.Name == picked || set != picked && file.Role == protocol.LocalFileModel && strings.HasPrefix(file.Name, set+"-0000") {
			file.Plan, file.Verdict, file.Recommended = protocol.LocalPlan{}, "", false
			files = append(files, file)
		}
	}
	if len(files) == 0 || files[0].Name != picked {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a repository and a GGUF model file."))
	}
	name := strings.TrimSuffix(set, ".gguf")
	// the id is what settings and routes name the model by: lower case, no blanks
	id := strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, strings.ToLower(name))
	if s.localIndex(id) >= 0 {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("That model is already in the list."))
	}
	first := files[0]
	model := protocol.LocalModel{
		ID: id, Name: name, Repo: found.Repo, Revision: found.Revision, State: protocol.LocalDownloading,
		Class: protocol.LocalClassMain, Params: first.Params, Context: first.Context, Tools: first.Tools, Warning: first.Warning,
		License: protocol.LocalLicense{Name: found.License}, Files: files,
	}
	if first.Params <= protocol.LightParams {
		model.Class = protocol.LocalClassLight
	}
	if p.Mmproj != "" {
		model.Files = append(model.Files, protocol.LocalFile{Name: p.Mmproj, Role: protocol.LocalFileMmproj, Size: 320_000_000})
		model.Vision = true
	}
	s.data.snapshot.Local.Models = append(s.data.snapshot.Local.Models, model)
	return model, nil
}

func (s *server) callLocal(method string, raw json.RawMessage) (any, error) {
	local := &s.data.snapshot.Local
	switch method {
	case protocol.MLocalModels:
		if demoEngine(&local.Runtime, s.data.snapshot.Settings.Providers.Models.Binary) {
			s.refreshLocalProvider()
		}
		s.planLocal()
		return *local, nil
	case protocol.MLocalFound:
		return protocol.LocalFoundResult{Models: s.demoFound()}, nil
	case protocol.MLocalUseFound:
		var p protocol.LocalUseFoundParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		model, err := s.demoUseFound(p.Path)
		if err != nil {
			return nil, err
		}
		s.localChanged()
		s.refreshLocalProvider()
		return model, nil
	case protocol.MLocalDevices:
		return protocol.LocalDevicesResult{Devices: local.Runtime.Devices}, nil
	case protocol.MLocalSearch:
		var p protocol.LocalSearchParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		return s.demoSearch(p.Repo)
	case protocol.MLocalDownload, protocol.MLocalCancel, protocol.MLocalRemove, protocol.MLocalStart:
		var p protocol.LocalModelParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		index := s.localIndex(p.ID)
		if method == protocol.MLocalStart && p.ID == "" {
			index = s.localIndex("lfm3-omni-2b")
		}
		if index < 0 {
			return nil, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That model is not available."))
		}
		model := &local.Models[index]
		switch method {
		case protocol.MLocalDownload:
			if model.State != protocol.LocalInstalled {
				model.State = protocol.LocalDownloading
				s.localChanged()
			}
			return protocol.Empty{}, nil
		case protocol.MLocalCancel:
			if model.State == protocol.LocalDownloading {
				model.State = protocol.LocalPaused
				s.localChanged()
			}
			return protocol.Empty{}, nil
		case protocol.MLocalRemove:
			if slices.Contains(local.Runtime.Loaded, model.ID) {
				local.Runtime.State, local.Runtime.Address, local.Runtime.Loaded = protocol.RuntimeStopped, "", nil
				local.Runtime.ActiveModel, local.Runtime.LoadedMiB = "", 0
			}
			if model.Catalogue {
				model.State = protocol.LocalAvailable
				for i := range model.Files {
					model.Files[i].Done = 0
				}
			} else {
				local.Models = append(local.Models[:index:index], local.Models[index+1:]...)
			}
			s.localDisk()
			s.syncLocalModels()
			s.planLocal()
			s.localChanged()
			s.refreshLocalProvider()
			return *local, nil
		default:
			if model.State != protocol.LocalInstalled {
				return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Download the model before starting it."))
			}
			id := model.ID
			local.Runtime.State, local.Runtime.Address, local.Runtime.Loaded = protocol.RuntimeReady, "http://127.0.0.1:41873/v1", []string{id}
			local.Runtime.ActiveModel, local.Runtime.Plans = id, nil
			s.planLocal()
			local.Runtime.LoadedMiB = local.Runtime.Plans[id].EstimatedMiB
			s.localChanged()
			s.refreshLocalProvider()
			return local.Runtime, nil
		}
	case protocol.MLocalAdd:
		var p protocol.LocalAddParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		model, err := s.demoAdd(p)
		if err != nil {
			return nil, err
		}
		s.localChanged()
		if model.State == protocol.LocalInstalled {
			s.refreshLocalProvider()
		}
		return model, nil
	case protocol.MLocalStop:
		local.Runtime.State = protocol.RuntimeStopping
		s.localChanged()
		local.Runtime.State, local.Runtime.Address, local.Runtime.Loaded = protocol.RuntimeStopped, "", nil
		local.Runtime.ActiveModel, local.Runtime.LoadedMiB, local.Runtime.Plans = "", 0, nil
		s.localChanged()
		s.refreshLocalProvider()
		return local.Runtime, nil
	case protocol.MLocalRuntimeInstall:
		var p protocol.LocalRuntimeParams
		if err := decode(raw, &p); err != nil {
			return nil, err
		}
		if p.Backend == "" {
			p.Backend = protocol.BackendVulkan
		}
		if p.Backend != protocol.BackendCPU && p.Backend != protocol.BackendVulkan {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose an available runtime backend."))
		}
		s.queueEvent(protocol.EvtLocalProgress, protocol.LocalProgress{ID: protocol.LocalRuntimeID, Done: 0, Total: 150_000_000, Phase: protocol.LocalPhaseDownload})
		s.queueEvent(protocol.EvtLocalProgress, protocol.LocalProgress{ID: protocol.LocalRuntimeID, Done: 150_000_000, Total: 150_000_000, Phase: protocol.LocalPhaseVerify})
		s.queueEvent(protocol.EvtLocalProgress, protocol.LocalProgress{ID: protocol.LocalRuntimeID, Done: 150_000_000, Total: 150_000_000, Phase: protocol.LocalPhaseDone})
		managed := demoManagedBuild(p.Backend)
		if at := slices.IndexFunc(local.Runtime.Binaries, func(build protocol.LocalRuntimeBinary) bool { return build.Source == protocol.RuntimeManaged }); at >= 0 {
			local.Runtime.Binaries[at] = managed
		} else {
			local.Runtime.Binaries = append([]protocol.LocalRuntimeBinary{managed}, local.Runtime.Binaries...)
		}
		// the fresh build is another server: which one runs is settled again from nothing
		local.Runtime.Path = ""
		demoEngine(&local.Runtime, s.data.snapshot.Settings.Providers.Models.Binary)
		s.refreshLocalProvider()
		s.localChanged()
		return protocol.Empty{}, nil
	}
	return nil, protocol.Errorf(protocol.CodeUnsupported, "demo method %s is not implemented", method)
}

func validDemoRepo(repo string) bool {
	owner, name, ok := strings.Cut(repo, "/")
	return ok && owner != "" && name != "" && !strings.ContainsAny(repo, " \\?#") && !strings.Contains(name, "/")
}

func (s *server) localDisk() {
	var total int64
	for _, model := range s.data.snapshot.Local.Models {
		// a file used where it lies takes none of Hyakki's own disk
		if model.Path != "" {
			continue
		}
		for _, file := range model.Files {
			total += file.Done
		}
	}
	s.data.snapshot.Local.DiskBytes = total
}

func (s *server) playLocal(ctx context.Context, out *outbox, id string) {
	for tick := 1; tick <= 5; tick++ {
		select {
		case <-ctx.Done():
			return
		case <-time.After(8 * time.Millisecond):
		}
		s.mu.Lock()
		index := s.localIndex(id)
		if index < 0 || s.data.snapshot.Local.Models[index].State != protocol.LocalDownloading {
			s.mu.Unlock()
			return
		}
		model := &s.data.snapshot.Local.Models[index]
		var total, done int64
		for i := range model.Files {
			file := &model.Files[i]
			file.Done = max(file.Done, file.Size*int64(tick)/5)
			total += file.Size
			done += file.Done
		}
		phase := protocol.LocalPhaseDownload
		if tick == 4 {
			phase = protocol.LocalPhaseVerify
		} else if tick == 5 {
			phase = protocol.LocalPhaseDone
			model.State = protocol.LocalInstalled
			s.planLocal()
		}
		s.localDisk()
		progress := protocol.LocalProgress{ID: id, File: model.Files[0].Name, Done: done, Total: total, Phase: phase}
		state, stateErr := json.Marshal(s.data.snapshot.Local)
		s.mu.Unlock()
		if raw, err := json.Marshal(progress); err == nil {
			_ = out.send(protocol.Event{Event: protocol.EvtLocalProgress, Data: raw})
		}
		if stateErr == nil {
			_ = out.send(protocol.Event{Event: protocol.EvtLocalChanged, Data: state})
		}
		if tick == 5 {
			s.mu.Lock()
			s.syncLocalModels()
			s.refreshLocalProvider()
			events := s.takeEvents()
			s.mu.Unlock()
			for _, event := range events {
				_ = out.send(event)
			}
		}
	}
}

func (s *server) probeModel(raw json.RawMessage) (protocol.ModelProbeResult, error) {
	var p protocol.ModelProbeParams
	if err := decode(raw, &p); err != nil {
		return protocol.ModelProbeResult{}, err
	}
	id := p.Model
	if id == "" {
		id = protocol.RouteAuto
	}
	if protocol.IsRoute(id) {
		id = demoLocalID
	}
	for _, model := range s.data.models.Models {
		if model.ID != id {
			continue
		}
		answer := protocol.ModelProbeResult{OK: model.Available && model.Tools, Model: protocol.Choice{ID: model.ID, Name: model.Name}}
		if answer.OK {
			answer.Message = i18n.T("Ready")
			answer.Options = demoOptions(p.Options, model.Reasoning)
		} else {
			answer.Message = i18n.T("This model cannot run an agent here.")
		}
		return answer, nil
	}
	return protocol.ModelProbeResult{Message: i18n.T("This model is not available.")}, nil
}
