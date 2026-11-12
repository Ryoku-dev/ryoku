package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/sys"
)

func perfTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	dir := filepath.Join(sys.ConfigHome(), "ryoku")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeTestStore(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestStore(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("%s does not parse: %v\n%s", path, err, raw)
	}
	return m
}

func dig(m map[string]any, path ...string) any {
	var cur any = m
	for _, k := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[k]
	}
	return cur
}

// Set look switches move to their new homes (reduce and speed into shell.json
// theme.motion; blur and shadows off both for windows in desktop.json and for
// the panels and the dock in shell.json theme.effects), the provider
// re-renders, every retired key leaves performance.json, the rest of each store
// is kept, and a second run is a no-op.
func TestReconcileRetiredPerfKeys(t *testing.T) {
	dir := perfTestHome(t)
	applied := ""
	old := applyWindowStyle
	applyWindowStyle = func(store string) { applied = store }
	t.Cleanup(func() { applyWindowStyle = old })

	perfPath := filepath.Join(dir, "performance.json")
	shellPath := filepath.Join(dir, "shell.json")
	desktopPath := filepath.Join(dir, "desktop.json")

	if r := reconcileRetiredPerfKeys(false); r.status != recOK {
		t.Fatalf("missing performance.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}

	perf := `{"lowPowerMode":false,"reduceMotion":true,"motionSpeed":1.5,"disableBlur":true,"disableShadows":true,"liveWallpaper60":true,"pauseLiveWallpaperWhenFullscreen":false,"ambientBarMotion":true}`
	writeTestStore(t, perfPath, perf)
	writeTestStore(t, shellPath, `{"theme":{"theme":"Default","motion":{"scale":1,"reduce":false}},"barStyle":"orpheus"}`)
	writeTestStore(t, desktopPath, `{"desktop":{"appearance":{"blurEnabled":true,"rounding":8}},"wm":{"niri":{"x":1}}}`)

	if r := reconcileRetiredPerfKeys(true); r.status != recWouldFix {
		t.Fatalf("check with retired keys: status=%s detail=%q, want would-fix", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(perfPath); string(got) != perf {
		t.Fatalf("check-only mutated performance.json: %s", got)
	}

	if r := reconcileRetiredPerfKeys(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	p := readTestStore(t, perfPath)
	for _, k := range retiredPerfKeys {
		if _, ok := p[k]; ok {
			t.Errorf("performance.json still carries %s", k)
		}
	}
	if p["ambientBarMotion"] != true || p["lowPowerMode"] != false {
		t.Errorf("live performance keys were lost: %v", p)
	}
	s := readTestStore(t, shellPath)
	if dig(s, "theme", "motion", "reduce") != true || dig(s, "theme", "motion", "scale") != 1.5 {
		t.Errorf("motion not carried into shell.json: %v", s["theme"])
	}
	if dig(s, "theme", "theme") != "Default" || s["barStyle"] != "orpheus" {
		t.Errorf("shell.json lost unrelated keys: %v", s)
	}
	if dig(s, "theme", "effects", "glass") != false || dig(s, "theme", "effects", "shadows") != false {
		t.Errorf("the panels' glass and shadows not turned off in shell.json: %v", s["theme"])
	}
	d := readTestStore(t, desktopPath)
	if dig(d, "desktop", "appearance", "blurEnabled") != false || dig(d, "desktop", "appearance", "shadowEnabled") != false {
		t.Errorf("blur and shadows not turned off in Window style: %v", d)
	}
	if dig(d, "desktop", "appearance", "rounding") != 8.0 || dig(d, "wm", "niri", "x") != 1.0 {
		t.Errorf("desktop.json lost unrelated keys: %v", d)
	}
	if applied != desktopPath {
		t.Errorf("provider not re-applied from desktop.json, got %q", applied)
	}

	if r := reconcileRetiredPerfKeys(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}

// Defaults carry nothing: a false reduceMotion, the shipped speed and false
// blur/shadows only leave performance.json, a user's own theme.motion.scale
// wins over motionSpeed, and no desktop.json is created or re-applied.
func TestReconcileRetiredPerfKeysKeepsDefaultsAndUserValues(t *testing.T) {
	dir := perfTestHome(t)
	applied := false
	old := applyWindowStyle
	applyWindowStyle = func(string) { applied = true }
	t.Cleanup(func() { applyWindowStyle = old })

	perfPath := filepath.Join(dir, "performance.json")
	shellPath := filepath.Join(dir, "shell.json")
	writeTestStore(t, perfPath, `{"reduceMotion":false,"motionSpeed":2,"disableBlur":false,"disableShadows":false}`)
	shell := `{"theme":{"motion":{"scale":0.5,"reduce":false}}}`
	writeTestStore(t, shellPath, shell)

	if r := reconcileRetiredPerfKeys(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(shellPath); string(got) != shell {
		t.Errorf("a user's own speed was overwritten: %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "desktop.json")); !os.IsNotExist(err) {
		t.Errorf("desktop.json was created for default blur and shadows: %v", err)
	}
	if applied {
		t.Error("provider re-applied with nothing carried")
	}
	if len(readTestStore(t, perfPath)) != 0 {
		t.Errorf("retired keys left in performance.json")
	}
}

// This machine's setup: window blur off in Window style while the old flags
// were false, so the panels kept their glass. The reconciler keeps it so: the
// panels' glass and shadows stay at their default (on), Window style is left as
// it is, and nothing is re-applied.
func TestReconcileRetiredPerfKeysKeepsPanelGlassUnderWindowBlurOff(t *testing.T) {
	dir := perfTestHome(t)
	applied := false
	old := applyWindowStyle
	applyWindowStyle = func(string) { applied = true }
	t.Cleanup(func() { applyWindowStyle = old })

	perfPath := filepath.Join(dir, "performance.json")
	shellPath := filepath.Join(dir, "shell.json")
	desktopPath := filepath.Join(dir, "desktop.json")
	writeTestStore(t, perfPath, `{"lowPowerMode":false,"disableBlur":false,"disableShadows":false}`)
	shell := `{"theme":{"theme":"Default","motion":{"scale":1,"reduce":false}}}`
	writeTestStore(t, shellPath, shell)
	desktop := `{"desktop":{"appearance":{"blurEnabled":false,"shadowEnabled":true}}}`
	writeTestStore(t, desktopPath, desktop)

	if r := reconcileRetiredPerfKeys(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if got, _ := os.ReadFile(shellPath); string(got) != shell {
		t.Errorf("shell.json changed, the panels' glass must stay at its default (on): %s", got)
	}
	if got, _ := os.ReadFile(desktopPath); string(got) != desktop {
		t.Errorf("Window style changed: %s", got)
	}
	if applied {
		t.Error("provider re-applied with nothing carried")
	}
	if p := readTestStore(t, perfPath); len(p) != 1 || p["lowPowerMode"] != false {
		t.Errorf("performance.json = %v, want only lowPowerMode left", p)
	}
}

// A true disableBlur on a box whose shell never ran turns window blur off and
// creates no shell.json.
func TestReconcileRetiredPerfKeysWritesNoShellJSONWhereTheShellNeverRan(t *testing.T) {
	dir := perfTestHome(t)
	old := applyWindowStyle
	applyWindowStyle = func(string) {}
	t.Cleanup(func() { applyWindowStyle = old })

	writeTestStore(t, filepath.Join(dir, "performance.json"), `{"disableBlur":true}`)
	if r := reconcileRetiredPerfKeys(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}
	if _, err := os.Stat(filepath.Join(dir, "shell.json")); !os.IsNotExist(err) {
		t.Errorf("shell.json was created: %v", err)
	}
	if d := readTestStore(t, filepath.Join(dir, "desktop.json")); dig(d, "desktop", "appearance", "blurEnabled") != false {
		t.Errorf("window blur not turned off: %v", d)
	}
}

func TestPerfMotionScale(t *testing.T) {
	for _, c := range []struct {
		in   string
		want float64
		ok   bool
	}{
		{`1.5`, 1.5, true}, {`8`, 3, true}, {`0.1`, 0.25, true},
		{`1`, 0, false}, {`0`, 0, false}, {`-2`, 0, false}, {`"fast"`, 0, false},
	} {
		got, ok := perfMotionScale(json.RawMessage(c.in))
		if got != c.want || ok != c.ok {
			t.Errorf("perfMotionScale(%s) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	if _, ok := perfMotionScale(nil); ok {
		t.Error("an absent motionSpeed must read unset")
	}
}
