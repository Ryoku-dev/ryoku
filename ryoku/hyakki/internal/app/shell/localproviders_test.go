package shell

import (
	"image"
	"reflect"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/app/desktop"
	"ryoku-rashin/internal/protocol"
)

// saving a local server address must not allow a host outside this computer and the local network
func TestALocalProviderAddressIsSavedAndRefusedOffTheNetwork(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/providers")
	if !stage.provider(protocol.ProviderOllamaLocal).Running || stage.provider(protocol.ProviderLlamaCpp).Running {
		t.Fatal("the demo daemon starts with Ollama running and llama.cpp stopped")
	}
	if _, shown := stage.named("Not running"); !shown {
		t.Fatal("the stopped server does not read Not running")
	}
	stage.click("llama.cpp")
	stage.click("llama.cpp address")
	stage.typeOver(len(stage.provider(protocol.ProviderLlamaCpp).Address), "http://192.168.1.20:8080")
	// other groups have a Save of their own, so Enter must submit this address field
	stage.press(key.NameReturn, 0)
	saved := "http://192.168.1.20:8080/v1"
	stage.await("the address is saved", func() bool { return stage.provider(protocol.ProviderLlamaCpp).Address == saved })
	if got := stage.runtime.store.Snapshot().Settings.Providers.Local[protocol.ProviderLlamaCpp]; got != saved {
		t.Fatalf("the settings hold %q", got)
	}
	stage.click("llama.cpp address")
	stage.typeOver(len(saved), "http://203.0.113.9:8080/v1")
	stage.press(key.NameReturn, 0)
	stage.settle()
	if got := stage.provider(protocol.ProviderLlamaCpp).Address; got != saved {
		t.Fatalf("an address off the network was saved: %q", got)
	}
	refusal := "Use an address on this computer or your local network"
	if stage.anyNamed(refusal) == "" {
		t.Fatalf("the refusal is not shown; on screen: %v", stage.names())
	}
	// a refusal belongs only to the text that produced it
	stage.typeOver(len("http://203.0.113.9:8080/v1"), saved)
	if shown := stage.anyNamed(refusal); shown != "" {
		t.Fatalf("the refusal outlives the text it was about: %q", shown)
	}
}

func TestHyakkiModelsManagerShowsRuntimeModelsAndLocalFirstSwitch(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/providers")
	if _, ok := stage.named("Hyakki models"); !ok {
		t.Fatalf("Hyakki's managed provider is absent: %v", stage.names())
	}
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	for _, word := range []string{"Starts when needed", "LFM3 Omni 2B", "Light", "Tools", "Pictures", "Sound", "Resume", "Add a model", "Light work model", "Stop after idle"} {
		stage.scrollToFeatureControl(word)
		if _, shown := stage.named(word); !shown {
			t.Fatalf("%q missing from local models manager: %v", word, stage.names())
		}
	}
	if stage.anyNamed("Runs with: NVIDIA GeForce RTX 4070") == "" {
		t.Fatalf("the installed model does not say what it runs with: %v", stage.names())
	}
	stage.scrollToFeatureControl("Device: Automatic")
	stage.click("Device: Automatic")
	if _, shown := stage.named("NVIDIA GeForce RTX 4070"); !shown {
		t.Fatalf("the device picker missed the probed graphics card: %v", stage.names())
	}
	stage.click("NVIDIA GeForce RTX 4070")
	stage.await("graphics card preference is saved", func() bool {
		return stage.runtime.store.Snapshot().Settings.Providers.Models.Device == "Vulkan0"
	})
	stage.scrollToFeatureControl("Use online models before ones on this computer")
	stage.click("Use online models before ones on this computer")
	stage.await("online-first preference is saved", func() bool {
		return stage.runtime.store.Snapshot().Settings.Providers.Models.RemoteFirst
	})
}

func TestHyakkiManagedModelDownloadRequiresLicenceConfirmation(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	stage.scrollToFeatureControl("Download")
	stage.click("Download")
	if _, shown := stage.named("Read the model licence"); !shown {
		t.Fatalf("download skipped licence confirmation: %v", stage.names())
	}
	if _, shown := stage.named("Download model"); !shown {
		t.Fatalf("licence confirmation offers no action: %v", stage.names())
	}
	stage.click("Download model")
	stage.await("chosen model finishes downloading", func() bool {
		for _, model := range stage.runtime.store.Snapshot().Local.Models {
			if model.ID == "qwen3-coder-7b" && model.State == protocol.LocalInstalled {
				return true
			}
		}
		return false
	})
}

func TestManagedModelStopShowsConfirmation(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("local inventory", func() bool { return len(stage.runtime.store.Snapshot().Local.Models) > 0 })
	stage.scrollToFeatureControl("Load now")
	stage.click("Load now")
	stage.await("local model loaded", func() bool {
		return stage.runtime.store.Snapshot().Local.Runtime.State == protocol.RuntimeReady
	})
	stage.scrollToFeatureControl("Stop the model")
	stage.click("Stop the model")
	stage.await("model stopped", func() bool {
		return stage.runtime.store.Snapshot().Local.Runtime.State == protocol.RuntimeStopped
	})
	if _, shown := stage.named("Stopped. No model runs on this computer now."); !shown {
		t.Fatalf("stopped notice missing: %v", stage.names())
	}
	if _, shown := stage.named("Stop the model"); shown {
		t.Fatal("stop control still shown after the model exited")
	}
}

// the row of an installed model says how it runs, and its Settings fold moves that and hands it back
func TestInstalledModelSaysWhatItRunsWithAndTakesAnOverride(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	override := func() protocol.LocalModelOverride {
		return stage.runtime.store.Snapshot().Settings.Providers.Models.Overrides["lfm3-omni-2b"]
	}
	plan := func() protocol.LocalPlan { return stage.runtime.store.Snapshot().Local.Models[0].Plan }
	before := plan()
	before.ContextOptions = append([]int(nil), before.ContextOptions...)
	beforeLoaded := stage.runtime.store.Snapshot().Local.Runtime.Plans["lfm3-omni-2b"]
	beforeLoaded.ContextOptions = append([]int(nil), beforeLoaded.ContextOptions...)
	stage.scrollToFeatureControl("LFM3 Omni 2B settings")
	if _, shown := stage.named("Run on: Automatic"); shown {
		t.Fatal("the overrides are shown before the fold is opened")
	}
	stage.click("LFM3 Omni 2B settings")
	stage.await("the Settings fold opens", func() bool {
		_, shown := stage.named("Run on: Automatic")
		return shown
	})
	if got := plan(); !reflect.DeepEqual(got, before) {
		t.Fatalf("opening a model's settings changed its published plan: before %+v, after %+v", before, got)
	}
	if got := stage.runtime.store.Snapshot().Local.Runtime.Plans["lfm3-omni-2b"]; !reflect.DeepEqual(got, beforeLoaded) {
		t.Fatalf("opening a model's settings changed its loaded plan: before %+v, after %+v", beforeLoaded, got)
	}
	for _, word := range []string{"Context: Automatic", "Context cache: Automatic", "Back to automatic"} {
		if _, shown := stage.named(word); !shown {
			t.Fatalf("%q missing from the model's settings: %v", word, stage.names())
		}
	}
	stage.scrollToFeatureControl("Run on: Automatic")
	stage.click("Run on: Automatic")
	stage.click("Processor only")
	stage.await("the model is set to the processor and planned again", func() bool {
		return override().Device == protocol.BackendCPU && plan().Kind == protocol.LocalDeviceCPU
	})
	if stage.anyNamed("Runs with: Processor") == "" || stage.anyNamed("Set by you: Processor only") == "" {
		t.Fatalf("the row does not show the override: %v", stage.names())
	}
	// what fits is offered, and what the model was not trained for is not
	stage.scrollToFeatureControl("Context: Automatic")
	stage.click("Context: Automatic")
	if _, shown := stage.named("65K"); shown {
		t.Fatalf("a context past the model's own is offered: %v", stage.names())
	}
	stage.click("16K")
	stage.await("the context is set", func() bool { return override().Context == 16384 && plan().Context == 16384 })
	stage.scrollToFeatureControl("Back to automatic")
	stage.click("Back to automatic")
	stage.await("the model is back to automatic", func() bool {
		return override() == (protocol.LocalModelOverride{}) && plan().Kind == protocol.LocalDeviceDiscrete && plan().Context == 32768
	})
	if stage.anyNamed("Runs with: NVIDIA GeForce RTX 4070") == "" {
		t.Fatalf("the row did not go back to the graphics card: %v", stage.names())
	}
}

func TestCustomGGUFWarningAppearsBeforeDownload(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 1043))
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	before := len(stage.runtime.store.Snapshot().Local.Models)
	stage.scrollToFeatureControl("Add a model")
	stage.click("Add a model")
	stage.scrollToFeatureControl("Model to add")
	stage.click("Model to add")
	stage.typeText("SupraLabs/Supra2.5-Pro-Instruct")
	stage.scrollToFeatureControl("Find model files")
	stage.click("Find model files")
	stage.await("GGUF preview metadata", func() bool {
		return stage.anyNamed("This model was trained on 512 tokens") != ""
	})
	stage.scrollToFeatureControl("Add and download")
	for _, chip := range []string{"512 context tokens", "0.07B parameters", "No tool calling", "Context too small"} {
		if _, shown := stage.named(chip); !shown {
			t.Fatalf("the verdict chip %q is missing before download: %v", chip, stage.names())
		}
	}
	stage.click("Add and download")
	if _, shown := stage.named("Read the model licence"); !shown {
		t.Fatalf("a model from Hugging Face skipped the licence: %v", stage.names())
	}
	if stage.anyNamed("This model was trained on 512 tokens") == "" {
		t.Fatalf("the warning vanished from the download confirmation: %v", stage.names())
	}
	for _, model := range stage.runtime.store.Snapshot().Local.Models {
		if model.State == protocol.LocalDownloading {
			t.Fatal("download started before consent")
		}
	}
	if got := len(stage.runtime.store.Snapshot().Local.Models); got != before {
		t.Fatalf("a model was added before consent: %d models, %d before", got, before)
	}
}

// a repository lists its files ranked for this computer with the daemon's pick taken, and a link adds it
func TestAddFromARepositoryStartsOnTheRecommendedFile(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 1043))
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	stage.scrollToFeatureControl("Add a model")
	stage.click("Add a model")
	stage.scrollToFeatureControl("Model to add")
	stage.click("Model to add")
	stage.typeText("https://huggingface.co/owner/model-GGUF")
	stage.press(key.NameReturn, 0)
	stage.await("the ranked files", func() bool {
		_, shown := stage.named("Recommended")
		return shown
	})
	for _, word := range []string{"Q6_K", "Q5_K_M", "Q4_K_M", "Q8_0", "Show all 5 files", "Picture reader (optional): mmproj-model.gguf"} {
		if _, shown := stage.named(word); !shown {
			t.Fatalf("%q missing from the ranked files: %v", word, stage.names())
		}
	}
	if _, shown := stage.named("model-F16-00002-of-00002.gguf"); shown {
		t.Fatal("a later part of a split set is offered as a model")
	}
	stage.scrollToFeatureControl("Add and download")
	stage.click("Add and download")
	stage.click("Download model")
	stage.await("the recommended file is installed with its picture reader", func() bool {
		for _, model := range stage.runtime.store.Snapshot().Local.Models {
			if model.ID == "model-q6_k" && model.State == protocol.LocalInstalled && model.Vision && model.Plan.Context > 0 {
				return true
			}
		}
		return false
	})
	if _, shown := stage.named("Model to add"); shown {
		t.Fatal("the add form stayed open after the model went in")
	}
}

// a file picked in the system chooser is the user's own: read, judged and added with no licence to agree to
func TestAddTakesAModelFileFromTheSystemChooser(t *testing.T) {
	stage := newStage(t, image.Pt(1612, 1043))
	path := "/home/nero/Models/Tiny Stories.F16.gguf"
	asked := ""
	stage.chooser = func(request desktop.ChooseRequest) []string {
		asked = request.Title
		if request.Folder || request.Multiple {
			t.Errorf("the chooser was asked for more than one file: %+v", request)
		}
		return []string{path}
	}
	stage.open("settings/providers")
	stage.scrollToFeatureControl("Hyakki models")
	stage.click("Hyakki models")
	stage.await("the local model inventory arrives", func() bool {
		return len(stage.runtime.store.Snapshot().Local.Models) >= 3
	})
	stage.scrollToFeatureControl("Add a model")
	stage.click("Add a model")
	stage.scrollToFeatureControl("Choose a model file")
	stage.click("Choose a model file")
	stage.await("the file is read", func() bool {
		return stage.anyNamed("This model was trained on 512 tokens") != ""
	})
	if asked == "" {
		t.Fatal("the system chooser was not asked")
	}
	stage.scrollToFeatureControl("Add this file")
	for _, chip := range []string{"No tool calling", "Context too small"} {
		if _, shown := stage.named(chip); !shown {
			t.Fatalf("the verdict chip %q is missing for the file: %v", chip, stage.names())
		}
	}
	stage.click("Add this file")
	if _, shown := stage.named("Read the model licence"); shown {
		t.Fatal("the user's own file asked for a licence")
	}
	stage.await("the file is in the models", func() bool {
		for _, model := range stage.runtime.store.Snapshot().Local.Models {
			if strings.HasPrefix(model.Repo, "local/") && model.Name == "Tiny Stories.F16.gguf" && model.State == protocol.LocalInstalled && model.Plan.Context == 512 {
				return true
			}
		}
		return false
	})
	if _, shown := stage.named("Model to add"); shown {
		t.Fatal("the add form stayed open after the file went in")
	}
	if _, shown := stage.named("Tiny Stories.F16.gguf"); !shown {
		t.Fatalf("the file is not in the list of models: %v", stage.names())
	}
}
