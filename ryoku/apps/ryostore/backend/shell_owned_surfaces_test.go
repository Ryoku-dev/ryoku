package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestShellManifestOwnsRecording(t *testing.T) {
	shell := testShellManifest()
	shell.Owns = []string{"recording"}
	shell.OwnsSetting = map[string]string{"recording": "demo.recordingInBar"}
	if err := validateShellManifest("barstyles/demo", shell); err != nil {
		t.Fatalf("recording refused: %v", err)
	}
	shell.Owns = []string{"recordings"}
	shell.OwnsSetting = map[string]string{}
	if err := validateShellManifest("barstyles/demo", shell); err == nil || !strings.Contains(err.Error(), "invalid owned surface") {
		t.Fatalf("unknown surface accepted: %v", err)
	}
}

// The shipped Orpheus item.toml must stay valid for the ryostore that ships with
// it: an owned surface ryostore does not know drops the whole item.
func TestShippedOrpheusOwnsRecordingSurface(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "quickshell", "shell", "subshells", "orpheus", "item.toml"))
	if err != nil {
		t.Fatal(err)
	}
	f := newShellFixture(t)
	f.addShipped("orpheus", string(raw))
	f.writeConfig("shell.json", map[string]any{"barStyle": "orpheus"})

	if _, err := runShellsSync(false); err != nil {
		t.Fatal(err)
	}
	index := f.readIndex()
	if !reflect.DeepEqual(index.ActiveOwns, []string{"recording"}) {
		t.Fatalf("activeOwns = %v", index.ActiveOwns)
	}
	orpheus := indexItem(t, index, "subshells", "orpheus")
	if !orpheus.InUse || orpheus.OwnsSetting["recording"] != "orpheus.recordingInBar" {
		t.Fatalf("orpheus record = %#v", orpheus)
	}
}
