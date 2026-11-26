package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/sys"
)

func TestStripRetiredAskBubble(t *testing.T) {
	out, changed, err := stripRetiredAskBubble([]byte(`{"ask":{"bubble":{"enabled":true,"x":0.9,"y":0.6,"screen":"DP-1"}},"weatherLocation":"Berlin"}`))
	if err != nil || !changed {
		t.Fatalf("the bubble must be stripped: changed=%v err=%v", changed, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	if _, present := cfg["ask"]; present {
		t.Errorf("an ask left empty must be dropped, got %v", cfg["ask"])
	}
	if cfg["weatherLocation"] != "Berlin" {
		t.Errorf("passthrough key weatherLocation was lost: %v", cfg)
	}

	out, changed, err = stripRetiredAskBubble([]byte(`{"ask":{"bubble":{},"other":1}}`))
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatal(err)
	}
	ask, _ := cfg["ask"].(map[string]any)
	if len(ask) != 1 || ask["other"] != float64(1) {
		t.Errorf("ask must keep exactly its other key, got %v", cfg["ask"])
	}

	if _, changed, err := stripRetiredAskBubble(out); err != nil || changed {
		t.Errorf("re-stripping a clean store must be a no-op: changed=%v err=%v", changed, err)
	}
	if _, changed, err := stripRetiredAskBubble([]byte(`{"dock":{}}`)); err != nil || changed {
		t.Errorf("a store with no ask must be untouched: changed=%v err=%v", changed, err)
	}
	if _, _, err := stripRetiredAskBubble([]byte(`{"ask":[1]}`)); err == nil {
		t.Error("a malformed ask must error, not be rewritten")
	}
}

func TestReconcileRetiredAskBubble(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if r := reconcileRetiredAskBubble(false); r.status != recOK {
		t.Fatalf("missing shell.json: status=%s, want ok", r.status.label())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a missing shell.json must not be created: %v", err)
	}

	stored := `{"ask":{"bubble":{"enabled":false,"x":0.94,"y":0.68}},"theme":{"theme":"Default"}}`
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRetiredAskBubble(true); r.status != recWouldFix {
		t.Fatalf("check with the bubble: status=%s, want would-fix", r.status.label())
	}
	if got, _ := os.ReadFile(path); string(got) != stored {
		t.Fatalf("check-only mutated the store: %s", got)
	}
	if r := reconcileRetiredAskBubble(false); r.status != recFixed {
		t.Fatalf("fix with the bubble: status=%s, want fixed", r.status.label())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("rewritten store does not parse: %v", err)
	}
	if _, present := cfg["ask"]; present {
		t.Fatalf("ask survived the fix: %s", raw)
	}
	if _, present := cfg["theme"]; !present {
		t.Fatalf("theme was lost: %s", raw)
	}
	if r := reconcileRetiredAskBubble(false); r.status != recOK {
		t.Fatalf("a clean store: status=%s, want ok", r.status.label())
	}
}
