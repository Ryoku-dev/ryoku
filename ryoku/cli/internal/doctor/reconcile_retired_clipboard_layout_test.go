package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/sys"
)

func compactRaw(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compact %s: %v", raw, err)
	}
	return buf.String()
}

func TestStripRetiredClipboardLayout(t *testing.T) {
	full := []byte(`{"clipboard":{"widthPercent":65,"heightPercent":42,"bottomPercent":0,"panelRadius":18,"paneRadius":12,"cardRadius":9,"pruneWeekly":true,"extra":{"a":[1, 2.50]}},"weatherLocation":"Berlin","dock":{"pinned":["kitty"],"frost":true}}`)
	out, changed, err := stripRetiredClipboardLayout(full)
	if err != nil || !changed {
		t.Fatalf("retired keys must be stripped: changed=%v err=%v", changed, err)
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(full, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out, &after); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	for _, key := range []string{"weatherLocation", "dock"} {
		if compactRaw(t, after[key]) != compactRaw(t, before[key]) {
			t.Errorf("sibling %s changed: %s -> %s", key, before[key], after[key])
		}
	}
	var clipBefore, clipAfter map[string]json.RawMessage
	if err := json.Unmarshal(before["clipboard"], &clipBefore); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after["clipboard"], &clipAfter); err != nil {
		t.Fatalf("clipboard with live keys was lost: %s", out)
	}
	for _, retired := range retiredClipboardLayoutKeys {
		if _, present := clipAfter[retired]; present {
			t.Errorf("fix did not strip clipboard.%s", retired)
		}
	}
	if len(clipAfter) != 2 {
		t.Errorf("clipboard must keep exactly pruneWeekly and extra, got %s", after["clipboard"])
	}
	for _, kept := range []string{"pruneWeekly", "extra"} {
		if compactRaw(t, clipAfter[kept]) != compactRaw(t, clipBefore[kept]) {
			t.Errorf("clipboard.%s changed: %s -> %s", kept, clipBefore[kept], clipAfter[kept])
		}
	}

	if _, changed, err := stripRetiredClipboardLayout(out); err != nil || changed {
		t.Errorf("re-stripping a clean store must be a no-op: changed=%v err=%v", changed, err)
	}

	out, changed, err = stripRetiredClipboardLayout([]byte(`{"clipboard":{"widthPercent":70,"cardRadius":4},"theme":"x"}`))
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	var only map[string]any
	if err := json.Unmarshal(out, &only); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	if _, present := only["clipboard"]; present {
		t.Errorf("a clipboard left empty after the strip must be dropped, got %v", only["clipboard"])
	}
	if only["theme"] != "x" {
		t.Errorf("passthrough key theme was lost: %v", only)
	}

	if _, changed, err := stripRetiredClipboardLayout([]byte(`{"dock":{}}`)); err != nil || changed {
		t.Errorf("a store with no clipboard must be untouched: changed=%v err=%v", changed, err)
	}
	if _, changed, err := stripRetiredClipboardLayout([]byte(`{"clipboard":{"pruneWeekly":false}}`)); err != nil || changed {
		t.Errorf("a clean clipboard must be untouched: changed=%v err=%v", changed, err)
	}
	if _, _, err := stripRetiredClipboardLayout([]byte(`{"clipboard":[65,42]}`)); err == nil {
		t.Error("a malformed clipboard must error, not be rewritten")
	}
}

func TestReconcileRetiredClipboardLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if r := reconcileRetiredClipboardLayout(false); r.status != recOK {
		t.Fatalf("missing shell.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("a missing shell.json must not be created: %v", err)
	}

	malformed := `{"clipboard":"wide","dock":{"frost":true}}`
	if err := os.WriteFile(path, []byte(malformed), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRetiredClipboardLayout(false); r.status != recWarn {
		t.Fatalf("malformed clipboard: status=%s detail=%q, want warn", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); string(got) != malformed {
		t.Fatalf("a malformed clipboard must leave the file untouched, got %s", got)
	}

	stored := `{"clipboard":{"widthPercent":65,"heightPercent":42,"bottomPercent":0,"panelRadius":18,"paneRadius":12,"cardRadius":9,"pruneWeekly":true},"weatherLocation":"Berlin"}`
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		t.Fatal(err)
	}

	if r := reconcileRetiredClipboardLayout(true); r.status != recWouldFix {
		t.Fatalf("check with retired keys: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); string(got) != stored {
		t.Fatalf("check-only mutated the store: %s", got)
	}

	if r := reconcileRetiredClipboardLayout(false); r.status != recFixed {
		t.Fatalf("fix with retired keys: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("rewritten store does not parse: %v", err)
	}
	clip, ok := cfg["clipboard"].(map[string]any)
	if !ok || len(clip) != 1 || clip["pruneWeekly"] != true {
		t.Fatalf("clipboard must keep only pruneWeekly=true, got %v", cfg["clipboard"])
	}
	if cfg["weatherLocation"] != "Berlin" {
		t.Errorf("passthrough key weatherLocation was lost: %v", cfg)
	}
	if _, err := os.Stat(path + ".ryoku-tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}

	if r := reconcileRetiredClipboardLayout(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, raw) {
		t.Fatalf("a clean store must stay byte-identical, got %s", got)
	}
}
