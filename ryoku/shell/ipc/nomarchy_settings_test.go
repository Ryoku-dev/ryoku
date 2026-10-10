package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeNomarchyBarStyle(t *testing.T, path, style string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"barStyle": style})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readNomarchyBarStyle(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatal(err)
	}
	style, _ := raw["barStyle"].(string)
	return style
}

func TestSettingsPatchActivatesNomarchyBeforePublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	writeNomarchyBarStyle(t, path, "qsbar")
	store := newSettingsStore(path)
	events := []string{}
	store.nomarchyLifecycle = func(active bool) error {
		if !active {
			t.Fatal("activation callback received false")
		}
		if got := readNomarchyBarStyle(t, path); got != "qsbar" {
			t.Fatalf("activation ran after persistence: barStyle = %q", got)
		}
		events = append(events, "activate")
		return nil
	}
	store.onChange = func([]byte) { events = append(events, "publish") }

	if err := store.patch("barStyle", json.RawMessage(`"nomarchy"`)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"activate", "publish"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if got := readNomarchyBarStyle(t, path); got != "nomarchy" {
		t.Fatalf("barStyle = %q, want nomarchy", got)
	}
}

func TestSettingsPatchDeactivatesNomarchyAfterPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	writeNomarchyBarStyle(t, path, "nomarchy")
	store := newSettingsStore(path)
	events := []string{}
	store.nomarchyLifecycle = func(active bool) error {
		if active {
			t.Fatal("deactivation callback received true")
		}
		if got := readNomarchyBarStyle(t, path); got != "qsbar" {
			t.Fatalf("deactivation ran before persistence: barStyle = %q", got)
		}
		events = append(events, "deactivate")
		return nil
	}
	store.onChange = func([]byte) { events = append(events, "publish") }

	if err := store.patch("barStyle", json.RawMessage(`"qsbar"`)); err != nil {
		t.Fatal(err)
	}
	if want := []string{"publish", "deactivate"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSettingsPatchNomarchyActivationFailureBlocksSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	writeNomarchyBarStyle(t, path, "qsbar")
	store := newSettingsStore(path)
	published := false
	store.onChange = func([]byte) { published = true }
	store.nomarchyLifecycle = func(bool) error { return errors.New("assets missing") }

	if err := store.patch("barStyle", json.RawMessage(`"nomarchy"`)); err == nil {
		t.Fatal("activation failure did not block patch")
	}
	if published {
		t.Fatal("failed activation published a settings frame")
	}
	if got := readNomarchyBarStyle(t, path); got != "qsbar" {
		t.Fatalf("failed activation persisted barStyle %q", got)
	}
}

func TestSettingsPatchNomarchyDeactivationFailureDoesNotBlockSwitch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	writeNomarchyBarStyle(t, path, "nomarchy")
	store := newSettingsStore(path)
	store.nomarchyLifecycle = func(active bool) error {
		if active {
			t.Fatal("deactivation callback received true")
		}
		return errors.New("cleanup failed")
	}

	if err := store.patch("barStyle", json.RawMessage(`"qsbar"`)); err != nil {
		t.Fatalf("deactivation failure blocked patch: %v", err)
	}
	if got := readNomarchyBarStyle(t, path); got != "qsbar" {
		t.Fatalf("barStyle = %q, want qsbar", got)
	}
}

func TestSettingsReloadReconcilesNomarchyAroundPublish(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell.json")
	writeNomarchyBarStyle(t, path, "qsbar")
	store := newSettingsStore(path)
	events := []string{}
	store.nomarchyLifecycle = func(active bool) error {
		if active {
			events = append(events, "activate")
		} else {
			events = append(events, "deactivate")
		}
		return nil
	}
	store.onChange = func([]byte) { events = append(events, "publish") }

	writeNomarchyBarStyle(t, path, "nomarchy")
	store.reload()
	writeNomarchyBarStyle(t, path, "qsbar")
	store.reload()
	if want := []string{"activate", "publish", "publish", "deactivate"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSettingsStartupReconcilesNomarchyState(t *testing.T) {
	for _, tc := range []struct {
		style string
		want  bool
	}{
		{style: "nomarchy", want: true},
		{style: "qsbar", want: false},
	} {
		t.Run(tc.style, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shell.json")
			writeNomarchyBarStyle(t, path, tc.style)
			store := newSettingsStore(path)
			called := 0
			store.nomarchyLifecycle = func(active bool) error {
				called++
				if active != tc.want {
					t.Fatalf("active = %t, want %t", active, tc.want)
				}
				return nil
			}
			if err := store.reconcileNomarchyStartup(); err != nil {
				t.Fatal(err)
			}
			if called != 1 {
				t.Fatalf("startup lifecycle calls = %d, want 1", called)
			}
		})
	}
}
