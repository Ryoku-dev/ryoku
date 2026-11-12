package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Check-only reports the stored switch and leaves the file alone, the fix strips
// it while keeping the live microphone keys and the rest of the store, and a
// second run is ok.
func TestReconcileRetiredPhoneMicSwitch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	path := filepath.Join(home, ".config", "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := []byte(`{"phone":{"enabled":true,"microphone":{"enabled":true,"micGain":140,"monitor":true},"scrcpy":{"appMode":{"displayDpi":320}}},"theme":{"theme":"Default"}}`)
	if err := os.WriteFile(path, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	if res := reconcileRetiredPhoneMicSwitch(true); res.status != recWouldFix {
		t.Fatalf("check-only: status %v (%s), want would-fix", res.status, res.detail)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, seed) {
		t.Fatalf("check-only rewrote shell.json:\n%s", got)
	}

	if res := reconcileRetiredPhoneMicSwitch(false); res.status != recFixed {
		t.Fatalf("fix: status %v (%s), want fixed", res.status, res.detail)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Phone struct {
			Enabled    bool                       `json:"enabled"`
			Microphone map[string]json.RawMessage `json:"microphone"`
			Scrcpy     map[string]json.RawMessage `json:"scrcpy"`
		} `json:"phone"`
		Theme map[string]string `json:"theme"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("fixed shell.json does not parse: %v\n%s", err, raw)
	}
	if _, present := cfg.Phone.Microphone["enabled"]; present {
		t.Errorf("fix left phone.microphone.enabled: %s", raw)
	}
	if string(cfg.Phone.Microphone["micGain"]) != "140" || string(cfg.Phone.Microphone["monitor"]) != "true" {
		t.Errorf("live microphone keys were lost: %s", raw)
	}
	if !cfg.Phone.Enabled || cfg.Phone.Scrcpy["appMode"] == nil || cfg.Theme["theme"] != "Default" {
		t.Errorf("unrelated keys were lost: %s", raw)
	}

	if res := reconcileRetiredPhoneMicSwitch(false); res.status != recOK {
		t.Fatalf("second run: status %v (%s), want ok", res.status, res.detail)
	}
}

// A microphone object holding only the switch is dropped, and stores without a
// phone or microphone object are untouched.
func TestStripRetiredPhoneMicSwitchEdges(t *testing.T) {
	out, changed, err := stripRetiredPhoneMicSwitch([]byte(`{"phone":{"microphone":{"enabled":false},"enabled":true}}`))
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	var cfg map[string]map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	if _, present := cfg["phone"]["microphone"]; present {
		t.Errorf("a microphone object left empty must be dropped: %s", out)
	}
	if cfg["phone"]["enabled"] != true {
		t.Errorf("phone.enabled was lost: %s", out)
	}
	for _, clean := range []string{`{"bars":{}}`, `{"phone":{"enabled":true}}`, `{"phone":{"microphone":{"micGain":100}}}`} {
		if _, changed, err := stripRetiredPhoneMicSwitch([]byte(clean)); err != nil || changed {
			t.Errorf("%s must be untouched: changed=%v err=%v", clean, changed, err)
		}
	}
	if _, _, err := stripRetiredPhoneMicSwitch([]byte(`{"phone":{"microphone":true}}`)); err == nil {
		t.Error("a malformed microphone object must error, not be rewritten")
	}
}
