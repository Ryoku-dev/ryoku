package settings

import (
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestLocalModelCardNamesWholeDownloadAndCapabilities(t *testing.T) {
	model := protocol.LocalModel{
		ID: "lfm3", Name: "LFM3 Omni 2B", Class: protocol.LocalClassMain,
		State: protocol.LocalInstalled, Tools: true, Vision: true, Audio: true,
		Files: []protocol.LocalFile{
			{Name: "lfm3.gguf", Role: protocol.LocalFileModel, Size: 1_100_000_000},
			{Name: "mmproj.gguf", Role: protocol.LocalFileMmproj, Size: 292_000_000},
		},
	}
	if got := modelDetail(model); !strings.Contains(got, "1.39 GB") || !strings.Contains(got, "1.10 GB") || !strings.Contains(got, "292 MB") {
		t.Fatalf("size does not account for the picture reader: %q", got)
	}
	if got := modelChips(model); !reflect.DeepEqual(got, []string{"Main", "Tools", "Pictures", "Sound"}) {
		t.Fatalf("model chips = %q", got)
	}
	if got := modelStateWord(model); got != "On this computer" {
		t.Fatalf("installed model reads %q", got)
	}
}

func TestManagedRuntimeAtRestIsNotReportedAsFailed(t *testing.T) {
	for _, runtime := range []protocol.LocalRuntime{
		{State: protocol.RuntimeStopped, Source: protocol.RuntimeManaged, Version: "b12521", Backend: protocol.BackendVulkan},
		{State: protocol.RuntimeStopped, Source: protocol.RuntimeSystem, Path: "/usr/bin/llama-server"},
	} {
		if got := runtimeWords(runtime); !strings.Contains(got, "Starts when needed") || strings.Contains(got, "Not running") {
			t.Fatalf("runtime at rest reads %q", got)
		}
	}
}

func TestDevicePickerKeepsUnpluggedChoiceVisible(t *testing.T) {
	runtime := protocol.LocalRuntime{Devices: []protocol.LocalDevice{{ID: "Vulkan0", Name: "Graphics card", FreeMiB: 7000, TotalMiB: 8000}}}
	options, ids, selected := deviceChoices(runtime, "Vulkan1")
	if selected != len(ids)-1 || !options[selected].Disabled || ids[selected] != "Vulkan1" {
		t.Fatalf("missing device changed saved choice: options %v, ids %v, chosen %d", options, ids, selected)
	}
	options, ids, selected = deviceChoices(runtime, "Vulkan0")
	if selected != 2 || ids[selected] != "Vulkan0" || !strings.Contains(options[selected].Detail, "7000 MB free") {
		t.Fatalf("device capacity missing: options %v, ids %v, chosen %d", options, ids, selected)
	}
}

func TestDevicePickerSaysWhichGraphicsShareTheSystemsMemory(t *testing.T) {
	runtime := protocol.LocalRuntime{Devices: []protocol.LocalDevice{
		{ID: "Vulkan0", Name: "NVIDIA GeForce RTX 4070", Kind: protocol.LocalDeviceDiscrete, FreeMiB: 6100, TotalMiB: 8188},
		{ID: "Vulkan1", Name: "AMD Radeon 780M", Kind: protocol.LocalDeviceIntegrated, FreeMiB: 9800, TotalMiB: 15700},
	}}
	options, _, _ := deviceChoices(runtime, "")
	if got := options[2].Detail; got != "Graphics card, 6100 MB free of 8188 MB" {
		t.Fatalf("discrete card reads %q", got)
	}
	if got := options[3].Detail; !strings.HasPrefix(got, "Integrated graphics") || !strings.Contains(got, "9800 MB free") {
		t.Fatalf("integrated graphics read %q", got)
	}
}

// the line under an installed model: device, context, layers on the card and memory, in that order
func TestPlanWordsNameDeviceContextLayersAndMemory(t *testing.T) {
	cases := []struct {
		plan protocol.LocalPlan
		want string
	}{
		{protocol.LocalPlan{Device: "Vulkan0", DeviceName: "NVIDIA GeForce RTX 4070", Kind: protocol.LocalDeviceDiscrete, Context: 32768, GPULayers: 24, Layers: 24, EstimatedMiB: 4655},
			"NVIDIA GeForce RTX 4070 · 32K context · all 24 layers on the card · about 4.5 GB"},
		{protocol.LocalPlan{Device: "Vulkan0", DeviceName: "NVIDIA GeForce RTX 4070", Kind: protocol.LocalDeviceDiscrete, Context: 65536, GPULayers: 17, Layers: 40, EstimatedMiB: 18983},
			"NVIDIA GeForce RTX 4070 · 65K context · 17 of 40 layers on the card · about 18.5 GB"},
		{protocol.LocalPlan{Device: "Vulkan1", DeviceName: "AMD Radeon 780M", Kind: protocol.LocalDeviceIntegrated, Context: 32768, GPULayers: 32, Layers: 32, EstimatedMiB: 6915},
			"AMD Radeon 780M · 32K context · all 32 layers on the graphics · about 6.8 GB"},
		{protocol.LocalPlan{Device: "none", DeviceName: "Processor", Kind: protocol.LocalDeviceCPU, Context: 16384, Layers: 24, EstimatedMiB: 900},
			"Processor · 16K context · about 900 MB"},
		{protocol.LocalPlan{Device: protocol.BackendCPU, Context: 512, ActualContext: 512},
			"Processor · 512 context"},
	}
	for _, test := range cases {
		if got := planWords(test.plan); got != test.want {
			t.Errorf("planWords = %q, want %q", got, test.want)
		}
	}
}

// what llama.cpp reported after the load is the truth the line shows, and a difference is explained once
func TestPlanShowsTheContextTheLoadReallyTook(t *testing.T) {
	plan := protocol.LocalPlan{DeviceName: "NVIDIA GeForce RTX 4070", Kind: protocol.LocalDeviceDiscrete, Context: 32768, ActualContext: 16384}
	if got := planWords(plan); !strings.Contains(got, "16K context") || strings.Contains(got, "32K") {
		t.Fatalf("the loaded context is not shown: %q", got)
	}
	if got := planDrift(plan); !strings.Contains(got, "16K") || !strings.Contains(got, "32K") {
		t.Fatalf("the difference is not explained: %q", got)
	}
	plan.Retry = "The first load ran out of memory."
	if got := planDrift(plan); got != "" {
		t.Fatalf("a retry already says why, yet the drift line reads %q", got)
	}
	plan.Retry, plan.ActualContext = "", plan.Context
	if got := planDrift(plan); got != "" {
		t.Fatalf("a load that took the plan reads %q", got)
	}
}

func TestContextOverrideOffersOnlyWhatFits(t *testing.T) {
	// the planner lists largest first, and a context a compact cache reached comes twice
	plan := protocol.LocalPlan{Context: 32768, ContextOptions: []int{32768, 32768, 16384, 8192}}
	options, values, selected := contextChoices(plan, 16384)
	labels := make([]string, len(options))
	for at, option := range options {
		labels[at] = option.Label
	}
	if !reflect.DeepEqual(labels, []string{"Automatic", "8K", "16K", "32K"}) || !reflect.DeepEqual(values, []int{0, 8192, 16384, 32768}) || selected != 2 {
		t.Fatalf("context choices = %v %v, chosen %d", labels, values, selected)
	}
	if _, _, selected = contextChoices(plan, 0); selected != 0 {
		t.Fatalf("no override reads as option %d, want Automatic", selected)
	}
	options, values, selected = contextChoices(plan, 65536)
	if selected != len(options)-1 || !options[selected].Disabled || values[selected] != 65536 || !strings.Contains(options[selected].Label, "does not fit") {
		t.Fatalf("a stored context that no longer fits changed: %v %v, chosen %d", options, values, selected)
	}
}

func TestOverrideWordsSayAutomaticOrWhatWasSet(t *testing.T) {
	runtime := protocol.LocalRuntime{Devices: []protocol.LocalDevice{{ID: "Vulkan1", Name: "AMD Radeon 780M"}}}
	cases := []struct {
		override protocol.LocalModelOverride
		want     string
	}{
		{protocol.LocalModelOverride{}, "Automatic"},
		{protocol.LocalModelOverride{Device: protocol.BackendCPU}, "Set by you: Processor only"},
		{protocol.LocalModelOverride{Device: "Vulkan1", Context: 16384}, "Set by you: AMD Radeon 780M, 16K context"},
		{protocol.LocalModelOverride{Device: "Vulkan7", KVType: protocol.LocalKVQ8}, "Set by you: Vulkan7, compact cache"},
	}
	for _, test := range cases {
		if got := overrideWords(test.override, runtime); got != test.want {
			t.Errorf("overrideWords(%+v) = %q, want %q", test.override, got, test.want)
		}
	}
	if _, values, selected := cacheChoices(protocol.LocalKVQ8); values[selected] != protocol.LocalKVQ8 || values[0] != "" {
		t.Fatalf("cache choices = %v, chosen %d", values, selected)
	}
}

// the chips a file carries before download: the pick, how it fits, then what stops Rashin using it
func TestFileChipsGiveTheVerdictBeforeDownload(t *testing.T) {
	words := func(file protocol.LocalFile, recommended bool) []string {
		var got []string
		for _, chip := range fileChips(file, recommended) {
			got = append(got, chip.Word)
		}
		return got
	}
	fits := protocol.LocalFile{Name: "model-Q4_K_M.gguf", Tools: true, Context: 32768, Verdict: protocol.LocalVerdictCard}
	if got := words(fits, true); !reflect.DeepEqual(got, []string{"Recommended", "Fits on the graphics card"}) {
		t.Fatalf("recommended file chips = %q", got)
	}
	slow := protocol.LocalFile{Name: "model-Q8_0.gguf", Tools: true, Context: 32768, Plan: protocol.LocalPlan{Verdict: protocol.LocalVerdictCPU}}
	if got := words(slow, false); !reflect.DeepEqual(got, []string{"Runs on the processor, slowly"}) {
		t.Fatalf("a verdict carried only by the plan is lost: %q", got)
	}
	tiny := protocol.LocalFile{Name: "tiny.gguf", Context: 512, Verdict: protocol.LocalVerdictUnsuitable}
	if got := words(tiny, false); !reflect.DeepEqual(got, []string{"Unsuitable for Rashin's everyday tasks", "No tool calling", "Context too small"}) {
		t.Fatalf("a model Rashin cannot use reads %q", got)
	}
	if _, tone := verdictWords(tiny.Verdict); tone != palette.Failed {
		t.Fatalf("unsuitable model needs a red warning, got %v", tone)
	}
	for verdict, want := range map[string]string{
		protocol.LocalVerdictShared:     "Fits in shared graphics memory",
		protocol.LocalVerdictSplit:      "Split between graphics card and memory; slower",
		protocol.LocalVerdictTooBig:     "Too big for this computer",
		protocol.LocalVerdictUnsuitable: "Unsuitable for Rashin's everyday tasks",
	} {
		if got, _ := verdictWords(verdict); got != want {
			t.Errorf("verdict %q reads %q, want %q", verdict, got, want)
		}
	}
}

// the add list shows a split set once, keeps the daemon's ranking and starts on its pick
func TestAddListTakesTheRankingHidesLaterPartsAndPicksTheRecommended(t *testing.T) {
	result := protocol.LocalSearchResult{
		Repo: "owner/model-GGUF", Recommended: "model-Q5_K_M.gguf", Mmproj: "mmproj-model.gguf",
		Files: []protocol.LocalFile{
			{Name: "model-Q4_K_M.gguf", Role: protocol.LocalFileModel, Quant: "Q4_K_M", Verdict: protocol.LocalVerdictCard},
			{Name: "model-Q5_K_M.gguf", Role: protocol.LocalFileModel, Quant: "Q5_K_M", Verdict: protocol.LocalVerdictCard},
			{Name: "model-F16-00001-of-00003.gguf", Role: protocol.LocalFileModel, Parts: 3, Size: 4_000_000_000, Verdict: protocol.LocalVerdictTooBig},
			{Name: "model-F16-00002-of-00003.gguf", Role: protocol.LocalFileModel, Parts: 3, Size: 4_000_000_000},
			{Name: "model-F16-00003-of-00003.gguf", Role: protocol.LocalFileModel, Parts: 3, Size: 1_000_000_000},
			{Name: "model-F16-00002-of-00002.gguf", Role: protocol.LocalFileModel, Parts: 2, Size: 7},
			{Name: "notes.txt", Role: protocol.LocalFileModel},
			{Name: "mmproj-other.gguf", Role: protocol.LocalFileMmproj},
			{Name: "mmproj-model.gguf", Role: protocol.LocalFileMmproj},
		},
	}
	var add localAddPanel
	add.take(result, false)
	names := make([]string, len(add.modelFiles))
	for at, file := range add.modelFiles {
		names[at] = file.Name
	}
	if !reflect.DeepEqual(names, []string{"model-Q4_K_M.gguf", "model-Q5_K_M.gguf", "model-F16-00001-of-00003.gguf"}) {
		t.Fatalf("model files listed = %q", names)
	}
	if chosen, picked := add.selected(); !picked || chosen.Name != "model-Q5_K_M.gguf" || add.pictureIndex != 2 || len(add.rows) != 3 {
		t.Fatalf("picked %q (%t), picture reader %d, rows %d", chosen.Name, picked, add.pictureIndex, len(add.rows))
	}
	set := add.modelFiles[2]
	if fileTitle(set) != "model-F16.gguf" || fileNote(set) != "3 parts" {
		t.Fatalf("split set reads %q, %q", fileTitle(set), fileNote(set))
	}
	// every part comes with its own size; the one row stands for the whole set and for no other set
	if set.Size != 9_000_000_000 {
		t.Fatalf("split set weighs %d, want the sum of its three parts", set.Size)
	}
	if first := add.modelFiles[0]; fileTitle(first) != "Q4_K_M" || fileNote(first) != "model-Q4_K_M.gguf" {
		t.Fatalf("quantisation row reads %q, %q", fileTitle(first), fileNote(first))
	}

	// nothing recommended and several to choose from: the choice is left open
	result.Recommended = ""
	add.take(result, false)
	if _, picked := add.selected(); picked {
		t.Fatal("a file was picked that the daemon did not recommend")
	}
	// a link to one file starts on that file, and a file that cannot run here is never preselected
	result.Path = "model-Q4_K_M.gguf"
	add.take(result, false)
	if chosen, _ := add.selected(); chosen.Name != "model-Q4_K_M.gguf" {
		t.Fatalf("the linked file is not picked: %q", chosen.Name)
	}
	only := protocol.LocalSearchResult{Path: "/home/nero/Models/big.gguf", Files: []protocol.LocalFile{{Name: "big.gguf", Role: protocol.LocalFileModel, Verdict: protocol.LocalVerdictTooBig}}}
	add.take(only, true)
	if _, picked := add.selected(); picked || !add.file {
		t.Fatalf("a file too big for this computer was preselected (file source %t)", add.file)
	}
	only.Files[0].Verdict = protocol.LocalVerdictCPU
	add.take(only, true)
	if chosen, picked := add.selected(); !picked || chosen.Name != "big.gguf" {
		t.Fatal("the one file of a local source is not picked")
	}
}

func TestModelDetailSaysWhereAFileModelCameFrom(t *testing.T) {
	imported := protocol.LocalModel{
		ID: "local-3f9a1c2e5b7d9f01", Name: "mine.gguf", Repo: "local/3f9a1c2e5b7d9f01", State: protocol.LocalInstalled, Context: 131072, Params: 7_000_000_000,
		Files: []protocol.LocalFile{{Name: "mine.gguf", Role: protocol.LocalFileModel, Size: 4_700_000_000}},
	}
	if got := modelDetail(imported); got != "4.70 GB · up to 131K context · Added from a file" {
		t.Fatalf("imported model reads %q", got)
	}
	// the context is said once, beside the size, and never again as a chip
	if got := modelChips(imported); !reflect.DeepEqual(got, []string{"7.00B parameters", "No tool calling"}) {
		t.Fatalf("imported model chips = %q", got)
	}
	if shortRevision("d508b88f08a5b4286f2d2805a6a8d69a0ff44c72") != "d508b88f08a5" || shortRevision("main") != "main" {
		t.Fatal("a revision is not cut to what tells two apart")
	}
	// nothing can download the user's own file again, so a broken one asks for the file
	imported.State = protocol.LocalBroken
	if got := modelAction(imported); got != "Add the file again" {
		t.Fatalf("a broken file model offers %q", got)
	}
	if got := modelAction(protocol.LocalModel{Repo: "owner/model-GGUF", State: protocol.LocalBroken}); got != "Download again" {
		t.Fatalf("a broken downloaded model offers %q", got)
	}
	split := protocol.LocalModel{Repo: "owner/model-GGUF", Files: []protocol.LocalFile{
		{Name: "model-F16-00001-of-00002.gguf", Role: protocol.LocalFileModel, Size: 4_000_000_000},
		{Name: "model-F16-00002-of-00002.gguf", Role: protocol.LocalFileModel, Size: 4_000_000_000},
		{Name: "mmproj-model.gguf", Role: protocol.LocalFileMmproj, Size: 320_000_000},
	}}
	if got := modelDetail(split); got != "8.32 GB (model 8.00 GB + picture reader 320 MB)" {
		t.Fatalf("a model in parts reads %q", got)
	}
}
