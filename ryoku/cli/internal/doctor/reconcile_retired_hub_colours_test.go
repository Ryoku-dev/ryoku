package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func seedShellJSON(t *testing.T, body string) string {
	t.Helper()
	cfg := retiredKeysHome(t)
	path := filepath.Join(cfg, "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readShellJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// The hub's colours move into theme while theme holds the shipped values, the hub
// keys go, and a second run is clean.
func TestReconcileRetiredHubColours(t *testing.T) {
	path := seedShellJSON(t, `{"hub":{"colours":"wallpaper","contrast":"high","tint":"low"},"theme":{"colours":"ryoku","design":"material"}}`)
	if r := reconcileRetiredHubColours(true); r.status != recWouldFix {
		t.Fatalf("check-only = %s %q, want todo", r.status.label(), r.detail)
	}
	if r := reconcileRetiredHubColours(false); r.status != recFixed {
		t.Fatalf("fix = %s %q, want fixed", r.status.label(), r.detail)
	}
	got := readShellJSON(t, path)
	theme := got["theme"].(map[string]any)
	if theme["colours"] != "wallpaper" || theme["contrast"] != "high" || theme["tint"] != "low" || theme["design"] != "material" {
		t.Fatalf("theme = %v", theme)
	}
	if _, ok := got["hub"]; ok {
		t.Fatalf("the emptied hub object stayed: %v", got["hub"])
	}
	if r := reconcileRetiredHubColours(false); r.status != recOK {
		t.Fatalf("second run = %s %q, want ok", r.status.label(), r.detail)
	}
}

// A choice already made in theme is kept, light carries as Ryoku's colours, and other
// hub keys stay.
func TestReconcileRetiredHubColoursKeepsChoices(t *testing.T) {
	path := seedShellJSON(t, `{"hub":{"colours":"light","contrast":"high","other":1},"theme":{"contrast":"standard","tint":"medium"}}`)
	if r := reconcileRetiredHubColours(false); r.status != recFixed {
		t.Fatalf("fix = %s %q, want fixed", r.status.label(), r.detail)
	}
	got := readShellJSON(t, path)
	theme := got["theme"].(map[string]any)
	if theme["colours"] != "ryoku" || theme["contrast"] != "high" || theme["tint"] != "medium" {
		t.Fatalf("theme = %v", theme)
	}
	hub := got["hub"].(map[string]any)
	if len(hub) != 1 || hub["other"] != float64(1) {
		t.Fatalf("hub = %v, want only other kept", hub)
	}
}
