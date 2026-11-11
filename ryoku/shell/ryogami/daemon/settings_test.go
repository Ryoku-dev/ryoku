package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func resetSettingsState() {
	settingsMu.Lock()
	settingsPending = map[string]interface{}{}
	if settingsFlushTimer != nil {
		settingsFlushTimer.Stop()
		settingsFlushTimer = nil
	}
	settingsMu.Unlock()
	setSettingsBaseline(nil)
}

func newSettingsDaemon(t *testing.T) *daemon {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("RYOGAMI_WALL_CONFIG", "")
	resetSettingsState()
	t.Cleanup(resetSettingsState)
	return &daemon{events: newEventHub()}
}

func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return m
}

func TestSettingsWatchRenameReplaceReconcilesOnce(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "ryogami.json")
	if err := os.WriteFile(path, []byte(`{"general":{"randomInterval":60}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fd, err := openSettingsWatch(path)
	if err != nil {
		t.Fatal(err)
	}
	const debounce = 40 * time.Millisecond
	reconciled := make(chan struct{}, 4)
	done := make(chan error, 1)
	go func() {
		done <- runSettingsWatch(fd, filepath.Base(path), debounce, func() {
			reconciled <- struct{}{}
		})
	}()

	replacement := filepath.Join(parent, ".ryogami.json.next")
	if err := os.WriteFile(replacement, []byte(`{"general":{"randomInterval":120}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reconciled:
	case <-time.After(time.Second):
		t.Fatal("rename-replace did not trigger reconciliation")
	}
	select {
	case <-reconciled:
		t.Fatal("rename-replace triggered more than one reconciliation")
	case <-time.After(3 * debounce):
	}

	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		syscall.Close(fd)
		t.Fatal("settings watcher did not stop after its directory was removed")
	}
}

func TestSettingsWatchOverflowForcesReconcileWithoutDroppingPendingChange(t *testing.T) {
	scheduled := 0
	reconciled := 0
	events := []inotifyEvent{
		{name: "ryogami.json", mask: syscall.IN_MOVED_TO},
		{wd: -1, mask: syscall.IN_Q_OVERFLOW},
	}
	err := handleSettingsWatchEvents(events, "ryogami.json", func() {
		scheduled++
	}, func() {
		reconciled++
	})
	if err != nil {
		t.Fatal(err)
	}
	if scheduled != 1 || reconciled != 1 {
		t.Fatalf("scheduled=%d reconciled=%d, want one of each", scheduled, reconciled)
	}
}

func TestValidateSetting(t *testing.T) {
	cases := []struct {
		key   string
		value interface{}
		ok    bool
	}{
		{"transition.enabled", true, true},
		{"transition.enabled", "yes", false},
		{"general.randomInterval", 300, true},
		{"general.randomInterval", 42.0, true},
		{"transition.durationMs", 900.0, true},
		{"transition.durationMs", 40.0, false},
		{"transition.durationMs", 20000.0, false},
		{"display.fillMode", "fit", true},
		{"display.fillMode", "bogus", false},
		{"display.fillMode", 5, false},
		{"general.language", "de", true},
		{"general.language", 3, false},
		{"theme.mode", "sun", true},
		{"theme.mode", "bogus", false},
		{"postProcessing", []interface{}{"cmd"}, true},
		{"postProcessing", "cmd", false},
		{"transition.shaderScopes", map[string]interface{}{}, true},
		{"transition.shaderScopes", []interface{}{}, false},
		{"nope.not.a.key", true, false},
	}
	for _, c := range cases {
		err := validateSetting(c.key, c.value)
		if c.ok && err != nil {
			t.Errorf("%s=%v: want valid, got %v", c.key, c.value, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s=%v: want error, got nil", c.key, c.value)
		}
	}
}

func TestPersistRyogamiRoundTrip(t *testing.T) {
	d := newSettingsDaemon(t)

	if err := d.persistSetting("general.randomInterval", 120.0); err != nil {
		t.Fatal(err)
	}
	if got := d.settingNumber("general.randomInterval"); got != 120 {
		t.Fatalf("pending read = %v, want 120", got)
	}
	if got := d.settingBool("transition.enabled"); got != true {
		t.Fatalf("unset key must fall back to its default true, got %v", got)
	}

	d.flushSettingsNow()
	m := readJSONFile(t, configPath())
	gen, _ := m["general"].(map[string]interface{})
	if gen["randomInterval"] != 120.0 {
		t.Fatalf("flushed file = %v, want general.randomInterval 120", m["general"])
	}
}

func TestFlushPreservesForeignKeys(t *testing.T) {
	d := newSettingsDaemon(t)
	seed := map[string]interface{}{
		"matugen": map[string]interface{}{"videoFrame": 2.5},
		"paths":   map[string]interface{}{"wallpaper": "/keep/me"},
		"custom":  map[string]interface{}{"handEdited": true},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	if err := os.WriteFile(configPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.persistSetting("selector.livePreview", false); err != nil {
		t.Fatal(err)
	}
	d.flushSettingsNow()

	m := readJSONFile(t, configPath())
	if mat, _ := m["matugen"].(map[string]interface{}); mat["videoFrame"] != 2.5 {
		t.Errorf("matugen.videoFrame lost: %v", m["matugen"])
	}
	if p, _ := m["paths"].(map[string]interface{}); p["wallpaper"] != "/keep/me" {
		t.Errorf("paths.wallpaper lost: %v", m["paths"])
	}
	if c, _ := m["custom"].(map[string]interface{}); c["handEdited"] != true {
		t.Errorf("hand-edited key lost: %v", m["custom"])
	}
	if sel, _ := m["selector"].(map[string]interface{}); sel["livePreview"] != false {
		t.Errorf("new value not written: %v", m["selector"])
	}
}

func TestResetSetting(t *testing.T) {
	d := newSettingsDaemon(t)
	if err := d.persistSetting("selector.livePreview", false); err != nil {
		t.Fatal(err)
	}
	d.flushSettingsNow()
	if err := d.resetSetting("selector.livePreview"); err != nil {
		t.Fatal(err)
	}
	d.flushSettingsNow()
	if got := d.settingBool("selector.livePreview"); got != true {
		t.Fatalf("after reset = %v, want default true", got)
	}
	m := readJSONFile(t, configPath())
	if _, ok := m["selector"]; ok {
		t.Fatalf("reset left the key behind: %v", m["selector"])
	}
}

// The video engine is a shell-owned key the picker writes directly. A settings
// change must return promptly (a nested store write once deadlocked every later
// change and stalled the picker) and the apply path must read the new engine.
func TestVideoEngineWritesThrough(t *testing.T) {
	d := newSettingsDaemon(t)
	done := make(chan error, 1)
	go func() { done <- d.setSetting("wallpaper.video_engine", "in_shell") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("setSetting never returned: a store write deadlocked")
	}
	if got := wallPrefs().Engine; got != "in_shell" {
		t.Fatalf("apply reads video engine %q, want in_shell", got)
	}
}

func TestDisplayFillModeMapping(t *testing.T) {
	d := newSettingsDaemon(t)

	if err := d.persistSetting("display.fillMode", "fit"); err != nil {
		t.Fatal(err)
	}
	shell := readJSONFile(t, shellPath())
	w, _ := shell["wallpaper"].(map[string]interface{})
	if w["content_fit"] != "Contain" {
		t.Fatalf("fit must map to content_fit Contain, got %v", w["content_fit"])
	}
	if got := d.settingString("display.fillMode"); got != "fit" {
		t.Fatalf("read back = %q, want fit", got)
	}

	if err := d.persistSetting("display.fillMode", "span"); err != nil {
		t.Fatal(err)
	}
	d.flushSettingsNow()
	ryo := readJSONFile(t, configPath())
	if disp, _ := ryo["display"].(map[string]interface{}); disp["fillMode"] != "span" {
		t.Fatalf("span must persist in ryogami.json, got %v", ryo["display"])
	}
	if got := d.settingString("display.fillMode"); got != "span" {
		t.Fatalf("read back = %q, want span", got)
	}
	shell = readJSONFile(t, shellPath())
	if w, _ = shell["wallpaper"].(map[string]interface{}); w["content_fit"] != "Contain" {
		t.Fatalf("span must not overwrite content_fit, got %v", w["content_fit"])
	}

	if err := d.persistSetting("display.fillMode", "tile"); err != nil {
		t.Fatal(err)
	}
	d.flushSettingsNow()
	ryo = readJSONFile(t, configPath())
	if disp, _ := ryo["display"].(map[string]interface{}); disp != nil {
		if _, ok := disp["fillMode"]; ok {
			t.Fatalf("span override should be cleared, got %v", disp)
		}
	}
	if got := d.settingString("display.fillMode"); got != "tile" {
		t.Fatalf("read back = %q, want tile", got)
	}
}

func TestShellKeyRoundTrip(t *testing.T) {
	d := newSettingsDaemon(t)
	seed := map[string]interface{}{"theme": map[string]interface{}{"name": "keep"}}
	b, _ := json.MarshalIndent(seed, "", "  ")
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	if err := os.WriteFile(shellPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.persistSetting("general.language", "de"); err != nil {
		t.Fatal(err)
	}
	shell := readJSONFile(t, shellPath())
	if shell["language"] != "de" {
		t.Fatalf("language not written to shell.json: %v", shell)
	}
	if th, _ := shell["theme"].(map[string]interface{}); th["name"] != "keep" {
		t.Fatalf("shell.json foreign key lost: %v", shell["theme"])
	}
	if got := d.settingString("general.language"); got != "de" {
		t.Fatalf("read back = %q, want de", got)
	}
}

func TestCollectValuesOverridesOnly(t *testing.T) {
	d := newSettingsDaemon(t)
	shell := map[string]interface{}{"wallpaper": map[string]interface{}{"content_fit": "Cover"}}
	b, _ := json.MarshalIndent(shell, "", "  ")
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	if err := os.WriteFile(shellPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if v := d.collectValues([]string{"display.fillMode"}); len(v) != 0 {
		t.Fatalf("content_fit Cover == default fill; want no override, got %v", v)
	}
	shell = map[string]interface{}{"wallpaper": map[string]interface{}{"content_fit": "Tile"}}
	b, _ = json.MarshalIndent(shell, "", "  ")
	if err := os.WriteFile(shellPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if v := d.collectValues([]string{"display.fillMode"}); v["display.fillMode"] != "tile" {
		t.Fatalf("differing content_fit must surface as tile, got %v", v)
	}
}

func TestSettingsSetValidationError(t *testing.T) {
	d := newSettingsDaemon(t)
	req := &request{Method: "settings.set", ID: 7, Params: json.RawMessage(`{"values":{"transition.durationMs":40}}`)}
	resp, claimed := d.dispatchSettings(req, req.params())
	if !claimed {
		t.Fatal("settings.set should be claimed by dispatchSettings")
	}
	if resp.Error == nil || resp.Error.Code != 5 {
		t.Fatalf("want error code 5, got %+v", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "transition.durationMs") {
		t.Fatalf("error must name the offending key, got %q", resp.Error.Message)
	}
	if _, err := os.Stat(configPath()); !os.IsNotExist(err) {
		t.Fatalf("a validation failure must not persist: %v", err)
	}
}

func TestDiffSettings(t *testing.T) {
	prev := map[string]interface{}{
		"transition.enabled":     true,
		"general.randomInterval": 300.0,
	}
	fresh := map[string]interface{}{
		"transition.enabled":     true,
		"general.randomInterval": 120.0,
		"selector.livePreview":   false,
	}
	changed := diffSettings(prev, fresh)
	if _, ok := changed["transition.enabled"]; ok {
		t.Errorf("unchanged key should not be reported: %v", changed)
	}
	if changed["general.randomInterval"] != 120.0 {
		t.Errorf("changed key wrong: %v", changed["general.randomInterval"])
	}
	if changed["selector.livePreview"] != false {
		t.Errorf("added key wrong: %v", changed["selector.livePreview"])
	}
	changed = diffSettings(map[string]interface{}{"selector.livePreview": false}, map[string]interface{}{})
	if changed["selector.livePreview"] != true {
		t.Errorf("removed key should revert to default true, got %v", changed["selector.livePreview"])
	}
}

func TestMigrateLegacyConfig(t *testing.T) {
	d := newSettingsDaemon(t)

	seed := map[string]interface{}{"paths": map[string]interface{}{"wallpaper": "/daemon/walls"}}
	sb, _ := json.MarshalIndent(seed, "", "  ")
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	if err := os.WriteFile(configPath(), sb, 0o644); err != nil {
		t.Fatal(err)
	}

	old := map[string]interface{}{
		"general":        map[string]interface{}{"closeOnSelection": true, "randomInterval": 120},
		"components":     map[string]interface{}{"wallpaperSelector": map[string]interface{}{"displayMode": "hex", "skewOffset": 76, "sliceWidth": 131, "customPresets": map[string]interface{}{"Mine": map[string]interface{}{"sliceHeight": 400}}}},
		"paper":          map[string]interface{}{"engine": "ryogami-paper"},
		"paths":          map[string]interface{}{"wallpaper": "/picker/walls", "steam": "/home/u/.steam"},
		"transition":     map[string]interface{}{"enabled": false, "shader": "morph", "durationMs": 800},
		"integrations":   []interface{}{map[string]interface{}{"name": "x"}},
		"postProcessing": []interface{}{"echo hi"},
		"matugen":        map[string]interface{}{"schemeType": "scheme-vibrant", "mode": "light"},
		"daynight":       map[string]interface{}{"enabled": true, "dayDir": "/d", "nightDir": "/n", "rotateIntervalMinutes": 30, "noRepeatWithinDay": false},
		"wallpaperMute":  false,
	}
	ob, _ := json.MarshalIndent(old, "", "  ")
	oldPath := filepath.Join(ryogamiWallConfigDir(), "config.json")
	_ = os.MkdirAll(ryogamiWallConfigDir(), 0o755)
	if err := os.WriteFile(oldPath, ob, 0o644); err != nil {
		t.Fatal(err)
	}

	d.migrateLegacyConfig()
	resetSettingsState()

	m := readJSONFile(t, configPath())

	if p, _ := m["paths"].(map[string]interface{}); p["wallpaper"] != "/daemon/walls" {
		t.Errorf("daemon path must win, got %v", p["wallpaper"])
	} else if p["steam"] != "/home/u/.steam" {
		t.Errorf("picker steam path must migrate, got %v", p["steam"])
	}
	if got := d.settingBool("general.closeOnSelection"); got != true {
		t.Errorf("general.closeOnSelection not migrated: %v", got)
	}
	if got := d.settingString("components.wallpaperSelector.displayMode"); got != "hex" {
		t.Errorf("displayMode not migrated: %v", got)
	}
	if got := d.settingBool("transition.enabled"); got != false {
		t.Errorf("transition.enabled not migrated: %v", got)
	}
	if got := d.settingString("transition.shader"); got != "morph" {
		t.Errorf("transition.shader not migrated: %v", got)
	}
	if got := d.settingBool("wallpaperMute"); got != false {
		t.Errorf("wallpaperMute not migrated: %v", got)
	}

	comp, _ := m["components"].(map[string]interface{})
	sel, _ := comp["wallpaperSelector"].(map[string]interface{})
	for _, k := range []string{"skewOffset", "sliceWidth", "customPresets", "presets"} {
		if _, ok := sel[k]; ok {
			t.Errorf("old look key %s must not migrate: %v", k, sel)
		}
	}

	if paper, _ := m["paper"].(map[string]interface{}); paper["engine"] != nil {
		t.Errorf("an invalid engine value must not migrate: %v", paper)
	}

	mat, _ := m["matugen"].(map[string]interface{})
	if _, ok := mat["schemeType"]; ok {
		t.Errorf("hub-owned matugen.schemeType must not migrate into ryogami.json: %v", mat)
	}
	if mat["mode"] != "light" {
		t.Errorf("matugen.mode should migrate: %v", mat)
	}

	if _, ok := m["daynight"]; ok {
		t.Errorf("daynight.* must not be copied verbatim: %v", m["daynight"])
	}
	if got := d.settingBool("schedule.enabled"); got != true {
		t.Errorf("schedule.enabled must follow daynight.enabled: %v", got)
	}
	rules := d.settingList("schedule.rules")
	if len(rules) != 2 {
		t.Fatalf("want two schedule rules, got %d: %v", len(rules), rules)
	}
	day, _ := rules[0].(map[string]interface{})
	if day["name"] != "Day" {
		t.Fatalf("first rule name = %v, want Day", day["name"])
	}
	if target, _ := day["target"].(map[string]interface{}); target["type"] != "folder" || target["value"] != "/d" {
		t.Errorf("day target wrong: %v", day["target"])
	}
	cond, _ := day["condition"].(map[string]interface{})
	children, _ := cond["children"].([]interface{})
	pred, _ := children[0].(map[string]interface{})
	if pred["block"] != "timewindow" || pred["from"] != "sunrise" || pred["to"] != "sunset" {
		t.Errorf("day predicate wrong: %v", pred)
	}
	if day["rotateMinutes"] != 30.0 {
		t.Errorf("rotateMinutes wrong: %v", day["rotateMinutes"])
	}
	if day["noRepeat"] != false {
		t.Errorf("noRepeat wrong: %v", day["noRepeat"])
	}
	if night, _ := rules[1].(map[string]interface{}); night["name"] != "Night" {
		t.Errorf("second rule wrong: %v", rules[1])
	} else if nt, _ := night["target"].(map[string]interface{}); nt["value"] != "/n" {
		t.Errorf("night target wrong: %v", nt)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("old config.json should be renamed away")
	}
	if _, err := os.Stat(oldPath + ".migrated"); err != nil {
		t.Errorf("config.json.migrated should exist: %v", err)
	}
}

func TestMigratePickerState(t *testing.T) {
	d := newSettingsDaemon(t)
	d.store = openStore(t.TempDir())

	tree := map[string]interface{}{
		"filterBar": map[string]interface{}{
			"last": map[string]interface{}{"sort": "name"},
		},
	}
	b, _ := json.MarshalIndent(tree, "", "  ")
	_ = os.MkdirAll(ryokuConfigDir(), 0o755)
	if err := os.WriteFile(configPath(), b, 0o644); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"type":       "video",
		"folder":     "Downloaded",
		"color":      "99",
		"sort":       "recent",
		"favourites": "true",
	} {
		d.store.stateSet("filterBar.last."+key, &value)
	}

	d.migratePickerState()
	resetSettingsState()

	got := readJSONFile(t, configPath())
	if value, _ := lookupDotted(got, "filterBar.last.sort"); value != "name" {
		t.Errorf("saved sort was overwritten: %v", value)
	}
	if value, _ := lookupDotted(got, "filterBar.last.type"); value != "video" {
		t.Errorf("type was not migrated: %v", value)
	}
	if value, _ := lookupDotted(got, "filterBar.last.folder"); value != "Downloaded" {
		t.Errorf("folder was not migrated: %v", value)
	}
	if value, _ := lookupDotted(got, "filterBar.last.color"); value != float64(99) {
		t.Errorf("colour was not migrated: %v", value)
	}
	if value, _ := lookupDotted(got, "filterBar.last.favourites"); value != true {
		t.Errorf("favourites was not migrated: %v", value)
	}
	for _, key := range []string{"type", "folder", "color", "sort", "favourites"} {
		if value, exists := d.store.stateGet("filterBar.last." + key); exists {
			t.Errorf("legacy state %s remains: %q", key, value)
		}
	}
}

func TestPickerStateDefaultsPersistChoices(t *testing.T) {
	d := newSettingsDaemon(t)
	if !d.settingBool("filterBar.sticky") {
		t.Fatal("filter choices should persist by default")
	}
	if got := d.settingString("components.wallpaperSelector.startPosition"); got != "applied" {
		t.Fatalf("start position = %q, want applied", got)
	}
}

// TestPerOutputSettingRoundTrip: the Displays page writes placement and lock
// under each output's own name. The pattern spec validates them, the daemon's
// readers see them, and a full settings.get hands them back on the next open.
func TestPerOutputSettingRoundTrip(t *testing.T) {
	d := newSettingsDaemon(t)
	d.surface = newWallSurface()
	req := &request{Method: "settings.set", ID: 8, Params: json.RawMessage(
		`{"values":{"display.fillModes.DP-1":"fit","display.outputLocks.DP-1":true}}`)}
	if resp, _ := d.dispatchSettings(req, req.params()); resp.Error != nil {
		t.Fatalf("per-output set rejected: %+v", resp.Error)
	}
	if got := d.fillModeOverrides()["DP-1"]; got != modeToContentFit("fit") {
		t.Fatalf("fill override for DP-1 = %q, want %q", got, modeToContentFit("fit"))
	}
	if locked := d.lockedOutputs(); !locked["DP-1"] || locked["HDMI-A-1"] {
		t.Fatalf("locked outputs = %v, want DP-1 only", locked)
	}
	all := d.collectValues(nil)
	if all["display.fillModes.DP-1"] != "fit" || all["display.outputLocks.DP-1"] != true {
		t.Fatalf("settings.get dropped the per-output values: %v", all)
	}

	bad := &request{Method: "settings.set", ID: 9, Params: json.RawMessage(`{"values":{"display.fillModes.DP-1":"zoom"}}`)}
	if resp, _ := d.dispatchSettings(bad, bad.params()); resp.Error == nil {
		t.Fatal("an unknown placement must still be rejected")
	}
}
