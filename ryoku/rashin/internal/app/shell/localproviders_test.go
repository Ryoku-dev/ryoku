package shell

import (
	"image"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

// A local provider's address is typed in its row and saved; one off this
// computer and the local network is refused and the saved one stays.
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
	// Settings is one long list and other groups have a Save of their own: Enter is this field's
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
	// the refusal belongs to the text it was given: typing the saved address back clears it
	stage.typeOver(len("http://203.0.113.9:8080/v1"), saved)
	if shown := stage.anyNamed(refusal); shown != "" {
		t.Fatalf("the refusal outlives the text it was about: %q", shown)
	}
}
