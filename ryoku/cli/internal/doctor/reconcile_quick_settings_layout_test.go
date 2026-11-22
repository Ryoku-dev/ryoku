package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const retiredQuickSettingsOrder = `[{"id":"vitals","visible":true},{"id":"connections","visible":true},{"id":"powerProfile","visible":true},{"id":"media","visible":false},{"id":"levels","visible":true},{"id":"bottomControls","visible":true}]`

func TestMigrateQuickSettingsLayout(t *testing.T) {
	reordered := `[{"id":"levels","visible":true},{"id":"vitals","visible":true},{"id":"connections","visible":true},{"id":"powerProfile","visible":true},{"id":"media","visible":false},{"id":"bottomControls","visible":true}]`
	partlyHidden := `[{"id":"vitals","visible":false},{"id":"connections","visible":true},{"id":"powerProfile","visible":true},{"id":"media","visible":false},{"id":"levels","visible":true},{"id":"bottomControls","visible":true}]`
	tests := []struct {
		name         string
		in           string
		changed      bool
		wantPosition string
		wantFirst    string
	}{
		{
			name:         "retired defaults move together",
			in:           `{"controls":{"position":"top-left","edgeGap":20,"sections":` + retiredQuickSettingsOrder + `,"hidden":["disk"]}}`,
			changed:      true,
			wantPosition: "top-right",
			wantFirst:    "connections",
		},
		{
			name:         "a chosen order stays while the position moves",
			in:           `{"controls":{"position":"top-left","sections":` + reordered + `}}`,
			changed:      true,
			wantPosition: "top-right",
			wantFirst:    "levels",
		},
		{
			name:         "a hidden block makes the order chosen",
			in:           `{"controls":{"position":"center","sections":` + partlyHidden + `}}`,
			changed:      false,
			wantPosition: "center",
			wantFirst:    "vitals",
		},
		{
			name:         "a chosen position stays while the order moves",
			in:           `{"controls":{"position":"center","sections":` + retiredQuickSettingsOrder + `}}`,
			changed:      true,
			wantPosition: "center",
			wantFirst:    "connections",
		},
		{
			name:    "no controls is left to the shipped defaults",
			in:      `{"fontScale":1.2}`,
			changed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var top map[string]json.RawMessage
			if err := json.Unmarshal([]byte(tt.in), &top); err != nil {
				t.Fatal(err)
			}
			changed, err := migrateQuickSettingsLayout(top)
			if err != nil {
				t.Fatalf("migration failed: %v", err)
			}
			if changed != tt.changed {
				t.Fatalf("changed = %v, want %v", changed, tt.changed)
			}
			if tt.wantPosition == "" {
				return
			}
			controls := decodeQuickSettingsControls(t, top)
			if controls.Position != tt.wantPosition {
				t.Fatalf("position = %q, want %q", controls.Position, tt.wantPosition)
			}
			if len(controls.Sections) != 6 || controls.Sections[0].ID != tt.wantFirst {
				t.Fatalf("sections = %+v, want %s first", controls.Sections, tt.wantFirst)
			}
		})
	}
}

func TestMigrateQuickSettingsLayoutKeepsSiblings(t *testing.T) {
	var top map[string]json.RawMessage
	in := `{"fontScale":1.2,"controls":{"position":"top-left","edgeGap":20,"sections":` + retiredQuickSettingsOrder + `,"hidden":["disk"]}}`
	if err := json.Unmarshal([]byte(in), &top); err != nil {
		t.Fatal(err)
	}
	if _, err := migrateQuickSettingsLayout(top); err != nil {
		t.Fatal(err)
	}
	controls := decodeQuickSettingsControls(t, top)
	if controls.EdgeGap != 20 || len(controls.Hidden) != 1 || controls.Hidden[0] != "disk" {
		t.Fatalf("edge gap or hidden parts changed: %+v", controls)
	}
	if string(top["fontScale"]) != "1.2" {
		t.Fatalf("sibling setting changed: %s", top["fontScale"])
	}
}

func TestReconcileQuickSettingsLayoutRunsOnce(t *testing.T) {
	path := quickSettingsLayoutRoots(t)
	writeQuickSettingsStore(t, path, `{"controls":{"position":"top-left","sections":`+retiredQuickSettingsOrder+`}}`)

	if res := reconcileQuickSettingsLayout(false); res.status != recFixed {
		t.Fatalf("retired layout = %s (%s), want fixed", res.status.label(), res.detail)
	}
	if got := readQuickSettingsPosition(t, path); got != "top-right" {
		t.Fatalf("position = %q, want top-right", got)
	}

	// picking top-left again after the move has to stick
	writeQuickSettingsStore(t, path, `{"controls":{"position":"top-left","sections":`+retiredQuickSettingsOrder+`}}`)
	if res := reconcileQuickSettingsLayout(false); res.status != recOK {
		t.Fatalf("second run = %s (%s), want ok", res.status.label(), res.detail)
	}
	if got := readQuickSettingsPosition(t, path); got != "top-left" {
		t.Fatalf("a later choice was overwritten: position = %q", got)
	}
}

func TestReconcileQuickSettingsLayoutCheckOnlyWritesNothing(t *testing.T) {
	path := quickSettingsLayoutRoots(t)
	in := `{"controls":{"position":"top-left","sections":` + retiredQuickSettingsOrder + `}}`
	writeQuickSettingsStore(t, path, in)

	if res := reconcileQuickSettingsLayout(true); res.status != recWouldFix {
		t.Fatalf("check-only = %s (%s), want would", res.status.label(), res.detail)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != in {
		t.Fatalf("check-only rewrote shell.json: %s", raw)
	}
	if _, err := os.Stat(quickSettingsLayoutMarker()); !os.IsNotExist(err) {
		t.Fatalf("check-only wrote the migration marker: %v", err)
	}
}

func TestReconcileQuickSettingsLayoutLeavesMalformedControls(t *testing.T) {
	path := quickSettingsLayoutRoots(t)
	in := `{"controls":"not-an-object"}`
	writeQuickSettingsStore(t, path, in)

	if res := reconcileQuickSettingsLayout(false); res.status != recWarn {
		t.Fatalf("malformed controls = %s (%s), want warn", res.status.label(), res.detail)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != in {
		t.Fatalf("malformed controls were rewritten: %s", raw)
	}
	if _, err := os.Stat(quickSettingsLayoutMarker()); !os.IsNotExist(err) {
		t.Fatalf("a failed migration was marked done: %v", err)
	}
}

type quickSettingsControls struct {
	Position string                 `json:"position"`
	EdgeGap  int                    `json:"edgeGap"`
	Sections []quickSettingsSection `json:"sections"`
	Hidden   []string               `json:"hidden"`
}

func decodeQuickSettingsControls(t *testing.T, top map[string]json.RawMessage) quickSettingsControls {
	t.Helper()
	var controls quickSettingsControls
	if err := json.Unmarshal(top["controls"], &controls); err != nil {
		t.Fatalf("controls do not decode: %v", err)
	}
	return controls
}

func quickSettingsLayoutRoots(t *testing.T) string {
	t.Helper()
	configRoot := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	return filepath.Join(configRoot, "ryoku", "shell.json")
}

func writeQuickSettingsStore(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readQuickSettingsPosition(t *testing.T, path string) string {
	t.Helper()
	top, _, err := readRawStore(path)
	if err != nil {
		t.Fatal(err)
	}
	return decodeQuickSettingsControls(t, top).Position
}
