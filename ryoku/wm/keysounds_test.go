package wm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Installed profiles join the row's own options after the shipped ones, once
// each; other rows and loose files are left alone.
func TestWithInstalledKeysounds(t *testing.T) {
	user, shipped := t.TempDir(), t.TempDir()
	for _, d := range []string{filepath.Join(user, "my-pack"), filepath.Join(shipped, "topre"), filepath.Join(shipped, "oreo")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(user, "README"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []json.RawMessage{
		json.RawMessage(`{"key":"x.profile","opts":["topre","cherry-mx-brown"]}`),
		json.RawMessage(`{"key":"x.volume","opts":["a"]}`),
	}
	out := WithInstalledKeysounds(rows, "x.profile", user, shipped, filepath.Join(user, "missing"))
	var got []struct {
		Opts []string `json:"opts"`
	}
	for _, r := range out {
		var row struct {
			Opts []string `json:"opts"`
		}
		if err := json.Unmarshal(r, &row); err != nil {
			t.Fatal(err)
		}
		got = append(got, row)
	}
	if want := []string{"topre", "cherry-mx-brown", "my-pack", "oreo"}; !reflect.DeepEqual(got[0].Opts, want) {
		t.Errorf("profile row = %v, want %v", got[0].Opts, want)
	}
	if !reflect.DeepEqual(got[1].Opts, []string{"a"}) {
		t.Errorf("another row changed: %v", got[1].Opts)
	}
}

func TestKeysoundsUserDirFollowsDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	if got := KeysoundsUserDir(); got != "/data/ryoku/keysounds" {
		t.Errorf("KeysoundsUserDir() = %q", got)
	}
}
