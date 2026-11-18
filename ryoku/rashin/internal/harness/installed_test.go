package harness

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestInstalledReadsCachedModelsWithoutRunningHarnesses(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	binary := writeTempBinary(t)
	marker := filepath.Join(t.TempDir(), "ran")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nprintf ran > '"+marker+"'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	models := []protocol.Choice{{ID: "model-a", Name: "Model A"}}
	New().recordCapabilities("work", binary, userHome, protocol.HarnessCapabilities{ModelList: models})
	settings := protocol.Settings{Agent: protocol.AgentSettings{Instances: []protocol.HarnessInstance{
		{ID: "work", Name: "Work account", Command: []string{binary}},
		{ID: "missing", Command: []string{"not-installed"}},
	}}}
	list := Installed(settings)
	if len(list) != 1 || list[0].ID != "work" || list[0].Name != "Work account" || !reflect.DeepEqual(list[0].Models, models) {
		t.Fatalf("installed harnesses = %#v", list)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("discovery ran the harness: %v", err)
	}
}
