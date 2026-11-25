package env

import (
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestLocalSizeReadsAsADownloadPageWould(t *testing.T) {
	cases := map[int64]string{
		1_392_000_000: "1.39 GB",
		1_100_000_000: "1.10 GB",
		292_000_000:   "292 MB",
		36_925_213:    "37 MB",
		4_096:         "4 KB",
		512:           "512 bytes",
	}
	for bytes, want := range cases {
		if got := LocalSize(bytes); got != want {
			t.Errorf("LocalSize(%d) = %q, want %q", bytes, got, want)
		}
	}
}

func TestShortContextRoundsToWhatAModelCardSays(t *testing.T) {
	cases := map[int]string{32_768: "32K", 131_072: "131K", 8_192: "8K", 1_048_576: "1M", 2_000_000: "2M", 512: "512", 999: "999", 1_000: "1K"}
	for tokens, want := range cases {
		if got := ShortContext(tokens); got != want {
			t.Errorf("ShortContext(%d) = %q, want %q", tokens, got, want)
		}
	}
}

// a stopped server with a model is rest, not a fault; a page offers the missing model or runtime first
func TestLocalStandNamesTheStepThatIsMissing(t *testing.T) {
	installed := protocol.LocalModel{ID: "lfm3", Name: "LFM3 Omni 2B", State: protocol.LocalInstalled}
	other := protocol.LocalModel{ID: "qwen", Name: "Qwen3 Coder 7B", State: protocol.LocalInstalled}
	partial := protocol.LocalModel{ID: "gemma", State: protocol.LocalPaused}
	cases := []struct {
		name  string
		local protocol.LocalResult
		want  LocalStanding
		model string
	}{
		{"nothing installed", protocol.LocalResult{Models: []protocol.LocalModel{partial}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped}}, LocalNothing, ""},
		{"no llama.cpp", protocol.LocalResult{Models: []protocol.LocalModel{installed}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeMissing}}, LocalNoEngine, "lfm3"},
		{"resting", protocol.LocalResult{Models: []protocol.LocalModel{partial, installed}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped}}, LocalResting, "lfm3"},
		{"starting", protocol.LocalResult{Models: []protocol.LocalModel{installed}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeStarting}}, LocalStarting, "lfm3"},
		{"the one in memory", protocol.LocalResult{Models: []protocol.LocalModel{installed, other}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeReady, Loaded: []string{"qwen"}}}, LocalReady, "qwen"},
		{"failed", protocol.LocalResult{Models: []protocol.LocalModel{installed}, Runtime: protocol.LocalRuntime{State: protocol.RuntimeFailed}}, LocalFailed, "lfm3"},
	}
	for _, test := range cases {
		standing, model := LocalStand(test.local)
		if standing != test.want || model.ID != test.model {
			t.Errorf("%s: LocalStand = %d on %q, want %d on %q", test.name, standing, model.ID, test.want, test.model)
		}
	}
}

func TestLocalBytesNeverCountsPastAFilesSize(t *testing.T) {
	done, total := LocalBytes(protocol.LocalModel{Files: []protocol.LocalFile{
		{Name: "model.gguf", Size: 1000, Done: 400},
		{Name: "mmproj.gguf", Size: 200, Done: 250},
	}})
	if done != 600 || total != 1200 {
		t.Fatalf("LocalBytes = %d of %d, want 600 of 1200", done, total)
	}
}

func TestLocalMemoryReadsAsAGraphicsCardStatesIt(t *testing.T) {
	cases := map[int]string{4655: "4.5 GB", 1024: "1.0 GB", 18983: "18.5 GB", 900: "900 MB", 0: "0 MB"}
	for mib, want := range cases {
		if got := LocalMemory(mib); got != want {
			t.Errorf("LocalMemory(%d) = %q, want %q", mib, got, want)
		}
	}
}

// the line under a model shows what the load took while it is in memory and the fresh estimate otherwise;
// the contexts on offer always come from the fresh estimate
func TestLocalPlanForPrefersWhatTheLoadTook(t *testing.T) {
	estimate := protocol.LocalPlan{Kind: protocol.LocalDeviceDiscrete, Context: 32768, ContextOptions: []int{32768, 16384, 8192}}
	took := protocol.LocalPlan{Kind: protocol.LocalDeviceDiscrete, Context: 32768, ActualContext: 16384, Retry: "It ran out of memory.", ContextOptions: []int{16384}}
	model := protocol.LocalModel{ID: "lfm3", State: protocol.LocalInstalled}
	if _, ok := LocalPlanFor(protocol.LocalResult{}, model); ok {
		t.Fatal("a model nobody planned has a plan")
	}
	model.Plan = estimate
	resting := protocol.LocalResult{Runtime: protocol.LocalRuntime{Plans: map[string]protocol.LocalPlan{"lfm3": took}}}
	if plan, ok := LocalPlanFor(resting, model); !ok || plan.ActualContext != 0 || LocalPlanContext(plan) != 32768 {
		t.Fatalf("a model at rest shows an old load: %+v", plan)
	}
	loaded := resting
	loaded.Runtime.Loaded = []string{"lfm3"}
	plan, ok := LocalPlanFor(loaded, model)
	if !ok || plan.Retry == "" || LocalPlanContext(plan) != 16384 {
		t.Fatalf("a model in memory does not show what the load took: %+v", plan)
	}
	if !reflect.DeepEqual(plan.ContextOptions, estimate.ContextOptions) {
		t.Fatalf("the contexts on offer are the load's, not the fresh estimate's: %v", plan.ContextOptions)
	}
	model.Plan = protocol.LocalPlan{}
	if plan, ok := LocalPlanFor(resting, model); !ok || plan.ActualContext != 16384 || len(plan.ContextOptions) != 1 {
		t.Fatalf("with no estimate the runtime's plan is all there is: %+v", plan)
	}
}

func TestLocalSourceTellsAFileFromARepository(t *testing.T) {
	t.Setenv("HOME", "/home/reader")
	cases := []struct {
		typed, source string
		file          bool
	}{
		{"  LiquidAI/LFM3-Omni-2B-GGUF ", "LiquidAI/LFM3-Omni-2B-GGUF", false},
		{"https://huggingface.co/owner/model-GGUF/blob/main/model-Q4_K_M.gguf", "https://huggingface.co/owner/model-GGUF/blob/main/model-Q4_K_M.gguf", false},
		{"/home/reader/Models/model.gguf", "/home/reader/Models/model.gguf", true},
		{"~/Models/model.gguf", "/home/reader/Models/model.gguf", true},
		{"'/home/reader/My Models/model.gguf'", "/home/reader/My Models/model.gguf", true},
		{"file:///home/reader/My%20Models/model.gguf", "/home/reader/My Models/model.gguf", true},
		{"model.gguf", "model.gguf", false},
		{"", "", false},
	}
	for _, test := range cases {
		source, file := LocalSource(test.typed)
		if source != test.source || file != test.file {
			t.Errorf("LocalSource(%q) = %q, %t; want %q, %t", test.typed, source, file, test.source, test.file)
		}
	}
}

func TestLocalFromFileKnowsTheUsersOwnModels(t *testing.T) {
	if !LocalFromFile(protocol.LocalModel{ID: "local-3f9a1c2e5b7d9f01", Repo: "local/3f9a1c2e5b7d9f01"}) {
		t.Fatal("a model added from a file reads as downloaded")
	}
	for _, repo := range []string{"LiquidAI/LFM3-Omni-2B-GGUF", "localai/model-GGUF", ""} {
		if LocalFromFile(protocol.LocalModel{Repo: repo}) {
			t.Errorf("a model from %q reads as added from a file", repo)
		}
	}
}

func TestLocalPlaceReadsAPathUnderHomeAsAFileManagerDoes(t *testing.T) {
	t.Setenv("HOME", "/home/reader")
	cases := map[string]string{
		"/home/reader":                          "~",
		"/home/reader/Models":                   "~/Models",
		"/home/reader/.ollama/models/blobs/abc": "~/.ollama/models/blobs/abc",
		"/home/readerly/Models":                 "/home/readerly/Models",
		"/usr/bin/llama-server":                 "/usr/bin/llama-server",
		"":                                      "",
	}
	for path, want := range cases {
		if got := LocalPlace(path); got != want {
			t.Errorf("LocalPlace(%q) = %q, want %q", path, got, want)
		}
	}
}

// only a model with a path of its own is one of the user's files: removing it must never read as deleting
func TestLocalInPlaceKnowsAFileThatStaysItsOwners(t *testing.T) {
	if !LocalInPlace(protocol.LocalModel{ID: "local-3f9a1c2e5b7d9f01", Path: "/home/reader/.ollama/models/blobs/abc"}) {
		t.Fatal("a model used where it lies reads as one Hyakki keeps")
	}
	if LocalInPlace(protocol.LocalModel{ID: "local-3f9a1c2e5b7d9f01", Repo: "local/3f9a1c2e5b7d9f01"}) {
		t.Fatal("a file Hyakki copied reads as used where it lies")
	}
}
