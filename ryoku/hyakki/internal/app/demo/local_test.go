package demo

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// every pretended computer has to place every model the way the daemon's planner would: whole on a card,
// in shared graphics memory, partly on a card, on the processor, or not at all
func TestDemoHardwarePlansEveryModel(t *testing.T) {
	type model struct {
		name          string
		bytes, params int64
		trained       int
	}
	models := []model{
		{"2B", 1_392_000_000, 2_000_000_000, 32768},
		{"7B", 4_700_000_000, 7_000_000_000, 131072},
		{"14B", 8_900_000_000, 14_000_000_000, 131072},
		{"32B", 19_800_000_000, 32_000_000_000, 131072},
		{"tiny", 130_000_000, 65_000_000, 512},
	}
	card, shared, split, cpu, big, unsuitable := protocol.LocalVerdictCard, protocol.LocalVerdictShared, protocol.LocalVerdictSplit, protocol.LocalVerdictCPU, protocol.LocalVerdictTooBig, protocol.LocalVerdictUnsuitable
	want := map[string][]string{
		"":     {card, shared, split, split, unsuitable},
		"igpu": {shared, shared, cpu, big, unsuitable},
		"cpu":  {cpu, big, big, big, unsuitable},
		"big":  {card, card, card, split, unsuitable},
		"arc":  {card, card, card, split, unsuitable},
	}
	for hardware, verdicts := range want {
		t.Setenv(demoHardwareEnv, hardware)
		machine := demoMachineNow()
		for at, m := range models {
			plan := demoPlan(machine, m.bytes, m.params, m.trained, "", protocol.LocalModelOverride{})
			if plan.Verdict != verdicts[at] {
				t.Errorf("%q hardware, %s model: verdict %q, want %q (%+v)", hardware, m.name, plan.Verdict, verdicts[at], plan)
				continue
			}
			if plan.Reason == "" || plan.EstimatedMiB <= 0 || plan.Layers <= 0 || plan.GPULayers > plan.Layers {
				t.Errorf("%q hardware, %s model: plan says too little: %+v", hardware, m.name, plan)
			}
			if plan.Verdict == big {
				continue
			}
			if plan.Context > m.trained || !slices.Contains(plan.ContextOptions, plan.Context) {
				t.Errorf("%q hardware, %s model: context %d is not one it offers %v", hardware, m.name, plan.Context, plan.ContextOptions)
			}
			switch plan.Verdict {
			case cpu:
				if plan.Kind != protocol.LocalDeviceCPU || plan.GPULayers != 0 {
					t.Errorf("%q hardware, %s model: a processor plan names a card: %+v", hardware, m.name, plan)
				}
			case split:
				if plan.Kind != protocol.LocalDeviceDiscrete || plan.GPULayers < 1 || plan.GPULayers >= plan.Layers {
					t.Errorf("%q hardware, %s model: a partial plan is not partial: %+v", hardware, m.name, plan)
				}
			case shared:
				if plan.Kind != protocol.LocalDeviceIntegrated || plan.GPULayers != plan.Layers {
					t.Errorf("%q hardware, %s model: a shared plan is not whole on integrated graphics: %+v", hardware, m.name, plan)
				}
			case card:
				if plan.Kind != protocol.LocalDeviceDiscrete || plan.GPULayers != plan.Layers || plan.DeviceName == "" {
					t.Errorf("%q hardware, %s model: a card plan is not whole on a card: %+v", hardware, m.name, plan)
				}
			}
		}
	}
	t.Setenv(demoHardwareEnv, "")
	if plan := demoPlan(demoMachineNow(), 1_392_000_000, 2_000_000_000, 32768, "", protocol.LocalModelOverride{}); plan.Context != 32768 || plan.KVType != protocol.LocalKVF16 || plan.DeviceName != "NVIDIA GeForce RTX 4070" {
		t.Fatalf("the stock model on the hybrid laptop: %+v", plan)
	}
	if plan := demoPlan(demoMachineNow(), 130_000_000, 65_000_000, 512, "", protocol.LocalModelOverride{}); plan.Context != 512 {
		t.Fatalf("a model trained on 512 tokens was planned at %d", plan.Context)
	}
}

// the setting for every model and a model's own both move the plan, and the model's own wins
func TestDemoPlanFollowsTheDeviceSettingAndTheOverride(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	machine := demoMachineNow()
	plan := func(device string, override protocol.LocalModelOverride) protocol.LocalPlan {
		return demoPlan(machine, 1_392_000_000, 2_000_000_000, 32768, device, override)
	}
	if got := plan(protocol.BackendCPU, protocol.LocalModelOverride{}); got.Kind != protocol.LocalDeviceCPU || got.Verdict != protocol.LocalVerdictCPU {
		t.Fatalf("Processor only for every model was not followed: %+v", got)
	}
	if got := plan(protocol.BackendCPU, protocol.LocalModelOverride{Device: "Vulkan1"}); got.Kind != protocol.LocalDeviceIntegrated || got.DeviceName != "AMD Radeon 780M" {
		t.Fatalf("the model's own device lost to the setting for all: %+v", got)
	}
	got := plan("", protocol.LocalModelOverride{Context: 16384, KVType: protocol.LocalKVQ8})
	if got.Context != 16384 || got.KVType != protocol.LocalKVQ8 || got.Kind != protocol.LocalDeviceDiscrete {
		t.Fatalf("a chosen context and cache were not used: %+v", got)
	}
	if got := plan("", protocol.LocalModelOverride{Context: 65536}); got.Context != 32768 {
		t.Fatalf("a context past what the model was trained on was planned: %+v", got)
	}
}

func TestDemoSettingsOverrideReplansOneModelAndBackToAutomatic(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "")
	s := &server{data: makeFixtures()}
	planned := func() protocol.LocalPlan {
		t.Helper()
		result := call(t, s, protocol.MLocalModels, protocol.Empty{}).(protocol.LocalResult)
		return result.Models[0].Plan
	}
	if plan := planned(); plan.Verdict != protocol.LocalVerdictCard {
		t.Fatalf("automatic plan = %+v", plan)
	}
	patch := func(entry any) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"providers": map[string]any{"models": map[string]any{"overrides": map[string]any{"lfm3-omni-2b": entry}}}})
		if err != nil {
			t.Fatal(err)
		}
		call(t, s, protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw})
	}
	patch(map[string]any{"device": protocol.BackendCPU, "context": 16384, "kvType": nil})
	if plan := planned(); plan.Kind != protocol.LocalDeviceCPU || plan.Context != 16384 {
		t.Fatalf("the override did not move the plan: %+v", plan)
	}
	patch(nil)
	if _, kept := s.data.snapshot.Settings.Providers.Models.Overrides["lfm3-omni-2b"]; kept {
		t.Fatal("Back to automatic left the override in the settings")
	}
	if plan := planned(); plan.Verdict != protocol.LocalVerdictCard || plan.Context != 32768 {
		t.Fatalf("the plan did not go back to automatic: %+v", plan)
	}
}

// the model in memory keeps what it was loaded with beside the fresh estimate, and one whose first load
// failed says so
func TestDemoLoadedModelRecordsWhatItTookAndWhyItRetried(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "1")
	s := &server{data: makeFixtures()}
	runtime := call(t, s, protocol.MLocalStart, protocol.LocalModelParams{ID: demoRetryID}).(protocol.LocalRuntime)
	loaded, ok := runtime.Plans[demoRetryID]
	if !ok || loaded.Retry == "" || loaded.ActualContext == 0 || loaded.ActualContext >= loaded.Context {
		t.Fatalf("the retried load is not recorded: %+v", runtime.Plans)
	}
	if runtime.LoadedMiB != loaded.EstimatedMiB {
		t.Fatalf("memory in use %d, plan %d", runtime.LoadedMiB, loaded.EstimatedMiB)
	}
	result := call(t, s, protocol.MLocalModels, protocol.Empty{}).(protocol.LocalResult)
	if again := result.Runtime.Plans[demoRetryID]; again.Retry != loaded.Retry || again.ActualContext != loaded.ActualContext {
		t.Fatalf("a later reading rewrote what the load took: %+v", again)
	}
	if override := s.data.snapshot.Settings.Providers.Models.Overrides[demoPartialID]; override.Device != protocol.BackendCPU {
		t.Fatalf("the states demo has no model set by hand: %+v", s.data.snapshot.Settings.Providers.Models.Overrides)
	}
	stopped := call(t, s, protocol.MLocalStop, protocol.LocalModelParams{}).(protocol.LocalRuntime)
	if len(stopped.Plans) != 0 {
		t.Fatalf("a stopped engine still reports a loaded plan: %+v", stopped.Plans)
	}
}

func TestDemoSearchRanksFilesForThisComputer(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	s := &server{data: makeFixtures()}
	result := call(t, s, protocol.MLocalSearch, protocol.LocalSearchParams{Repo: "owner/model-GGUF"}).(protocol.LocalSearchResult)
	if result.Recommended != "model-Q6_K.gguf" || result.Files[0].Name != result.Recommended || !result.Files[0].Recommended || result.Mmproj != "mmproj-model.gguf" {
		t.Fatalf("recommended %q, first %+v, projector %q", result.Recommended, result.Files[0], result.Mmproj)
	}
	verdicts := make(map[string]string)
	for _, file := range result.Files {
		verdicts[file.Name] = file.Verdict
	}
	if verdicts["model-Q4_K_M.gguf"] != protocol.LocalVerdictCard || verdicts["model-Q8_0.gguf"] != protocol.LocalVerdictCard || verdicts["model-F16-00001-of-00002.gguf"] != protocol.LocalVerdictShared {
		t.Fatalf("verdicts on the hybrid laptop = %v", verdicts)
	}
	if verdicts["model-F16-00002-of-00002.gguf"] != "" || verdicts["mmproj-model.gguf"] != "" {
		t.Fatalf("a later part or the projector was judged as a model: %v", verdicts)
	}
	// each part keeps its own size and the count of parts, as the daemon lists them
	parts, weight := 0, int64(0)
	for _, file := range result.Files {
		if file.Parts == 2 {
			parts++
			weight += file.Size
		}
	}
	if parts != 2 || weight != 8_000_000_000 {
		t.Fatalf("the split set is %d parts of %d bytes: %+v", parts, weight, result.Files)
	}

	t.Setenv(demoHardwareEnv, "cpu")
	result = call(t, s, protocol.MLocalSearch, protocol.LocalSearchParams{Repo: "owner/model-GGUF"}).(protocol.LocalSearchResult)
	if result.Recommended != "model-Q4_K_M.gguf" || result.Files[0].Verdict != protocol.LocalVerdictCPU {
		t.Fatalf("without a graphics card the smallest file is the pick: %q, %+v", result.Recommended, result.Files[0])
	}
	for _, file := range result.Files {
		if file.Name == "model-Q8_0.gguf" && file.Verdict != protocol.LocalVerdictTooBig {
			t.Fatalf("8 GB of memory holds Q8_0: %+v", file)
		}
	}
}

func TestDemoSearchTakesALinkAndAFile(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	s := &server{data: makeFixtures()}
	search := func(source string) protocol.LocalSearchResult {
		t.Helper()
		return call(t, s, protocol.MLocalSearch, protocol.LocalSearchParams{Repo: source}).(protocol.LocalSearchResult)
	}
	if got := search("https://huggingface.co/owner/model-GGUF"); got.Repo != "owner/model-GGUF" || got.Path != "" || len(got.Files) < 5 {
		t.Fatalf("a repository link = %+v", got)
	}
	one := search("https://huggingface.co/owner/model-GGUF/blob/main/model-Q4_K_M.gguf")
	if one.Repo != "owner/model-GGUF" || one.Path != "model-Q4_K_M.gguf" || one.Recommended != one.Path || len(one.Files) != 2 {
		t.Fatalf("a link to one file = %+v", one)
	}
	later := search("https://huggingface.co/owner/model-GGUF/resolve/main/model-F16-00002-of-00002.gguf?download=true")
	if later.Path != "model-F16-00001-of-00002.gguf" || len(later.Files) != 3 || later.Files[0].Parts != 2 {
		t.Fatalf("a link to a later part = %+v", later)
	}
	file := search("/home/nero/Models/Tiny Stories.F16.gguf")
	if file.Repo != "" || file.Path != "/home/nero/Models/Tiny Stories.F16.gguf" || len(file.Files) != 1 || file.Files[0].Tools || file.Files[0].Context != 512 || file.Files[0].Warning == "" {
		t.Fatalf("a file on this computer = %+v", file)
	}
	for _, refused := range []string{"", "model", "notes.txt", "/home/nero/notes.txt", "https://huggingface.co/owner"} {
		raw, _ := json.Marshal(protocol.LocalSearchParams{Repo: refused})
		if _, err := s.call(protocol.MLocalSearch, raw); err == nil {
			t.Errorf("%q was searched", refused)
		}
	}
}

func TestDemoAddDownloadsASplitSetAndInstallsAFileAtOnce(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "1")
	s := &server{data: makeFixtures()}
	set := call(t, s, protocol.MLocalAdd, protocol.LocalAddParams{Repo: "owner/model-GGUF", File: "model-F16-00001-of-00002.gguf", Mmproj: "mmproj-model.gguf"}).(protocol.LocalModel)
	if set.ID != "model-f16" || len(set.Files) != 3 || set.Files[0].Size+set.Files[1].Size != 8_000_000_000 || !set.Vision || set.State != protocol.LocalDownloading {
		t.Fatalf("the split set = %+v", set)
	}
	// a file of the user's is copied before the answer: it comes back installed, named by its content
	path := "/home/nero/Models/Tiny Stories.F16.gguf"
	file := call(t, s, protocol.MLocalAdd, protocol.LocalAddParams{Repo: path}).(protocol.LocalModel)
	if !strings.HasPrefix(file.ID, "local-") || file.Repo != "local/"+strings.TrimPrefix(file.ID, "local-") || file.Name != "Tiny Stories.F16.gguf" {
		t.Fatalf("the file on this computer is named %q from %q as %q", file.ID, file.Repo, file.Name)
	}
	if file.State != protocol.LocalInstalled || file.Catalogue || file.Warning == "" || file.Plan.Context != 512 || file.Files[0].Done != file.Files[0].Size {
		t.Fatalf("the file on this computer = %+v", file)
	}
	before := len(s.data.snapshot.Local.Models)
	again := call(t, s, protocol.MLocalAdd, protocol.LocalAddParams{Repo: path}).(protocol.LocalModel)
	if again.ID != file.ID || len(s.data.snapshot.Local.Models) != before {
		t.Fatalf("the same file again made %q beside %q (%d models, %d before)", again.ID, file.ID, len(s.data.snapshot.Local.Models), before)
	}
	// the same file once more is how a copy that went bad is repaired
	if broken := s.data.snapshot.Local.Models[s.localIndex(demoBrokenID)]; broken.State != protocol.LocalBroken {
		t.Fatalf("the states demo has no broken file model: %+v", broken)
	}
	repaired := call(t, s, protocol.MLocalAdd, protocol.LocalAddParams{Repo: "/home/nero/Models/my-finetune.Q4_K_M.gguf"}).(protocol.LocalModel)
	if repaired.ID != demoBrokenID || repaired.State != protocol.LocalInstalled || repaired.Plan.Context == 0 {
		t.Fatalf("adding the file again did not repair it: %+v", repaired)
	}
}

// a build found outside the usual places runs only once the settings name it, and clearing the setting
// goes back to Hyakki's own
func TestDemoEngineFollowsTheBuildTheSettingsName(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "")
	t.Setenv(demoEngineEnv, "")
	s := &server{data: makeFixtures()}
	runtime := func() protocol.LocalRuntime {
		t.Helper()
		return call(t, s, protocol.MLocalModels, protocol.Empty{}).(protocol.LocalResult).Runtime
	}
	binary := func(path string) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"providers": map[string]any{"models": map[string]any{"binary": path}}})
		if err != nil {
			t.Fatal(err)
		}
		call(t, s, protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw})
	}
	stock := runtime()
	if stock.Source != protocol.RuntimeManaged || stock.State != protocol.RuntimeStopped || len(stock.Binaries) != 3 || stock.Binaries[0].Path != stock.Path {
		t.Fatalf("the stock engine = %+v", stock)
	}
	built := stock.Binaries[2]
	if built.Source == protocol.RuntimeManaged || built.Source == protocol.RuntimeSystem || built.Backend != protocol.BackendCUDA {
		t.Fatalf("the demo has no build of the user's: %+v", stock.Binaries)
	}
	call(t, s, protocol.MLocalStart, protocol.LocalModelParams{ID: "lfm3-omni-2b"})
	binary(built.Path)
	chosen := runtime()
	if chosen.Path != built.Path || chosen.Source != protocol.RuntimeConfigured || chosen.Backend != protocol.BackendCUDA || chosen.State != protocol.RuntimeStopped || len(chosen.Loaded) != 0 {
		t.Fatalf("the chosen build does not run, or kept the old server's model: %+v", chosen)
	}
	picked := "/home/nero/src/llama.cpp/build/bin/llama-server"
	binary(picked)
	if got := runtime(); got.Path != picked || len(got.Binaries) != 4 || got.Binaries[3].Path != picked {
		t.Fatalf("a file picked in the chooser did not join the builds: %+v", got)
	}
	binary("")
	if got := runtime(); got.Path != stock.Path || got.Source != protocol.RuntimeManaged {
		t.Fatalf("clearing the setting did not go back to Hyakki's own: %+v", got)
	}
}

// with only a build nothing uses, no engine runs until one is chosen, and Hyakki's own download joins the list
func TestDemoEngineWaitsForAChoiceWhenOnlyFoundBuildsExist(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "")
	t.Setenv(demoEngineEnv, "found")
	s := &server{data: makeFixtures()}
	waiting := call(t, s, protocol.MLocalModels, protocol.Empty{}).(protocol.LocalResult).Runtime
	if waiting.State != protocol.RuntimeMissing || waiting.Path != "" || len(waiting.Binaries) != 1 {
		t.Fatalf("a found build ran without being chosen: %+v", waiting)
	}
	call(t, s, protocol.MLocalRuntimeInstall, protocol.LocalRuntimeParams{Backend: protocol.BackendCPU})
	got := call(t, s, protocol.MLocalModels, protocol.Empty{}).(protocol.LocalResult).Runtime
	if got.Source != protocol.RuntimeManaged || got.Backend != protocol.BackendCPU || got.State != protocol.RuntimeStopped || len(got.Binaries) != 2 {
		t.Fatalf("Hyakki's own download is not the engine: %+v", got)
	}
	t.Setenv(demoEngineEnv, "none")
	if none := demoLocalResult().Runtime; none.State != protocol.RuntimeMissing || len(none.Binaries) != 0 {
		t.Fatalf("a computer with no llama.cpp = %+v", none)
	}
}

// a found file is judged for this computer, used where it lies and forgotten again with nothing deleted
func TestDemoFoundModelIsUsedInPlaceAndForgotten(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "")
	t.Setenv(demoFoundEnv, "")
	s := &server{data: makeFixtures()}
	find := func() []protocol.LocalFoundModel {
		t.Helper()
		return call(t, s, protocol.MLocalFound, protocol.Empty{}).(protocol.LocalFoundResult).Models
	}
	found := find()
	if len(found) != 4 {
		t.Fatalf("found %d models, want the four other programs keep", len(found))
	}
	sources, verdicts := make(map[string]bool), make(map[string]string)
	for _, model := range found {
		sources[model.Source] = true
		verdicts[model.Source] = model.Plan.Verdict
		if model.Path == "" || model.Name == "" || model.Size == 0 || model.Plan.Verdict == "" || model.Model != "" {
			t.Errorf("a found model says too little: %+v", model)
		}
	}
	if !sources[protocol.LocalFoundOllama] || !sources[protocol.LocalFoundLMStudio] || !sources[protocol.LocalFoundHuggingFace] || !sources[protocol.LocalFoundLlamaCpp] {
		t.Fatalf("sources found = %v", sources)
	}
	// the small one fits the card whole, the 8B only shared graphics memory, and 42 GB fits nothing here
	if verdicts[protocol.LocalFoundLlamaCpp] != protocol.LocalVerdictCard || verdicts[protocol.LocalFoundOllama] != protocol.LocalVerdictShared || verdicts[protocol.LocalFoundHuggingFace] != protocol.LocalVerdictTooBig {
		t.Fatalf("verdicts on the hybrid laptop = %v", verdicts)
	}
	before, disk := len(s.data.snapshot.Local.Models), s.data.snapshot.Local.DiskBytes
	used := call(t, s, protocol.MLocalUseFound, protocol.LocalUseFoundParams{Path: found[0].Path}).(protocol.LocalModel)
	if used.Path != found[0].Path || used.State != protocol.LocalInstalled || used.Name != "qwen3:8b" || !used.Tools || used.Plan.Context == 0 {
		t.Fatalf("the model used in place = %+v", used)
	}
	if s.data.snapshot.Local.DiskBytes != disk {
		t.Fatalf("a file used where it lies was counted as Hyakki's disk: %d, was %d", s.data.snapshot.Local.DiskBytes, disk)
	}
	again := call(t, s, protocol.MLocalUseFound, protocol.LocalUseFoundParams{Path: found[0].Path}).(protocol.LocalModel)
	if again.ID != used.ID || len(s.data.snapshot.Local.Models) != before+1 {
		t.Fatalf("the same file twice made %q beside %q", again.ID, used.ID)
	}
	if got := find()[0].Model; got != used.ID {
		t.Fatalf("the scan does not name the model that uses the file: %q", got)
	}
	routed := false
	for _, model := range s.data.models.Models {
		routed = routed || model.ID == protocol.ProviderRashinModels+"/"+used.ID
	}
	if !routed {
		t.Fatal("the model used in place cannot be routed to")
	}
	call(t, s, protocol.MLocalRemove, protocol.LocalModelParams{ID: used.ID})
	if len(s.data.snapshot.Local.Models) != before || find()[0].Model != "" {
		t.Fatal("removing the model did not forget it, or the file is no longer found")
	}
	raw, _ := json.Marshal(protocol.LocalUseFoundParams{Path: "/home/nero/Models/gone.gguf"})
	if _, err := s.call(protocol.MLocalUseFound, raw); err == nil {
		t.Fatal("a file that was never found was used")
	}
}

// a folder the user added is looked in with the rest, and only for as long as it is in the settings
func TestDemoFoundLooksInTheFoldersTheUserAdded(t *testing.T) {
	t.Setenv(demoHardwareEnv, "")
	t.Setenv(demoStatesEnv, "")
	t.Setenv(demoFoundEnv, "none")
	s := &server{data: makeFixtures()}
	folders := func(list []string) {
		t.Helper()
		raw, err := json.Marshal(map[string]any{"providers": map[string]any{"models": map[string]any{"folders": list}}})
		if err != nil {
			t.Fatal(err)
		}
		call(t, s, protocol.MSettingsSet, protocol.SettingsSetParams{Patch: raw})
	}
	find := func() []protocol.LocalFoundModel {
		t.Helper()
		return call(t, s, protocol.MLocalFound, protocol.Empty{}).(protocol.LocalFoundResult).Models
	}
	if got := find(); len(got) != 0 {
		t.Fatalf("a computer where no program keeps a model found %+v", got)
	}
	folders([]string{"/home/nero/Models"})
	got := find()
	if len(got) != 2 || got[0].Source != protocol.LocalFoundFolder || !strings.HasPrefix(got[0].Path, "/home/nero/Models/") {
		t.Fatalf("the added folder was not looked in: %+v", got)
	}
	if tiny := got[1]; tiny.Plan.Verdict != protocol.LocalVerdictUnsuitable || tiny.Warning == "" || tiny.Tools {
		t.Fatalf("a model Hyakki cannot use is not judged so: %+v", tiny)
	}
	folders([]string{})
	if got := find(); len(got) != 0 {
		t.Fatalf("a folder taken out of the settings is still looked in: %+v", got)
	}
}
