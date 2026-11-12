package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func writeDoc(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(ryokuConfigDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A palette knob merges into matugen.json without the hub: fields the
// setting does not own (the roster, prefer, engine) survive the write.
func TestMatugenKnobsWriteNatively(t *testing.T) {
	d := newSettingsDaemon(t)
	writeDoc(t, matugenStorePath(), `{"engine":"matugen","prefer":"value","templates":{"kitty":false},"mode":"dark"}`)
	for key, value := range map[string]interface{}{
		"theme.mode":         "light",
		"matugen.schemeType": "scheme-vibrant",
		"matugen.contrast":   0.35,
		"matugen.colorIndex": float64(4),
	} {
		if err := d.setSetting(key, value); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
	got := readJSONFile(t, matugenStorePath())
	want := map[string]interface{}{
		"engine": "matugen", "prefer": "value", "templates": map[string]interface{}{"kitty": false},
		"mode": "light", "schemeType": "scheme-vibrant", "contrast": 0.35, "sourceColorIndex": float64(4),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matugen.json = %v, want %v", got, want)
	}
	if d.settingNumber("matugen.colorIndex") != 4 {
		t.Fatalf("colour index read back %v", d.setting("matugen.colorIndex"))
	}
}

// The ranges are the shell's: five dominant colours and a -1 to 1 contrast.
func TestMatugenRangesMatchTheShell(t *testing.T) {
	cases := []struct {
		key   string
		value interface{}
		ok    bool
	}{
		{"matugen.colorIndex", float64(4), true},
		{"matugen.colorIndex", float64(5), false},
		{"matugen.contrast", -1.0, true},
		{"matugen.contrast", 0.5, true},
		{"matugen.contrast", 50.0, false},
		{"matugen.schemeType", "scheme-smart", false},
		{"theme.mode", "smart", true},
		{"theme.gtkTheme", "adwaita", true},
		{"theme.gtkTheme", "breeze", false},
	}
	for _, c := range cases {
		if err := validateSetting(c.key, c.value); (err == nil) != c.ok {
			t.Errorf("validate(%s, %v) = %v, want ok=%v", c.key, c.value, err, c.ok)
		}
	}
}

// auto was the shell's luminance rule under another name: it is accepted,
// stored as smart, and a stored auto reads back as smart.
func TestThemeModeAutoIsSmart(t *testing.T) {
	d := newSettingsDaemon(t)
	if err := d.setSetting("theme.mode", "auto"); err != nil {
		t.Fatalf("auto rejected: %v", err)
	}
	if got := readJSONFile(t, matugenStorePath())["mode"]; got != "smart" {
		t.Fatalf("stored mode = %v, want smart", got)
	}
	writeDoc(t, matugenStorePath(), `{"mode":"auto","schemeType":"scheme-smart"}`)
	if got := d.settingString("theme.mode"); got != "smart" {
		t.Fatalf("stored auto reads as %q", got)
	}
	if got := d.settingString("matugen.schemeType"); got != "scheme-tonal-spot" {
		t.Fatalf("stored scheme-smart reads as %q", got)
	}
	if v := d.collectValues([]string{"theme.mode"}); len(v) != 0 {
		t.Fatalf("auto is the default smart; want no override, got %v", v)
	}
}

// The theme keys live in theme.json beside the hub's own fields, and a reset
// drops the field so every reader falls back to its default.
func TestThemeDocKeys(t *testing.T) {
	d := newSettingsDaemon(t)
	writeDoc(t, themeStorePath(), `{"followWallpaper":true,"scheme":"mono"}`)
	if !d.settingBool("theme.themeApps") || !d.settingBool("theme.gnomeAccent") || d.settingString("theme.gtkTheme") != "adw" {
		t.Fatal("absent theme keys must read as their defaults")
	}
	if err := d.setSetting("theme.themeApps", false); err != nil {
		t.Fatal(err)
	}
	if err := d.setSetting("theme.gtkTheme", "system"); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, themeStorePath())
	if got["themeApps"] != false || got["gtkTheme"] != "system" || got["scheme"] != "mono" || got["followWallpaper"] != true {
		t.Fatalf("theme.json = %v", got)
	}
	if err := d.resetSetting("theme.gtkTheme"); err != nil {
		t.Fatal(err)
	}
	if _, ok := readJSONFile(t, themeStorePath())["gtkTheme"]; ok {
		t.Fatal("reset left gtkTheme behind")
	}
}

// The app roster toggles one entry and keeps the rest; an app the roster does
// not name is listed as themed, the way the shell renders it.
func TestAppThemeRoster(t *testing.T) {
	d := newSettingsDaemon(t)
	writeDoc(t, matugenStorePath(), `{"mode":"dark","templates":{"kitty":false,"hyprland":true}}`)
	if err := setAppTheme("btop", false); err != nil {
		t.Fatal(err)
	}
	got := readJSONFile(t, matugenStorePath())
	want := map[string]interface{}{"kitty": false, "hyprland": true, "btop": false}
	if !reflect.DeepEqual(got["templates"], want) || got["mode"] != "dark" {
		t.Fatalf("matugen.json = %v", got)
	}
	rows, _ := d.themeAppsResult()["apps"].([]map[string]interface{})
	if len(rows) != len(appThemeNames) {
		t.Fatalf("listed %d apps, want %d", len(rows), len(appThemeNames))
	}
	for _, r := range rows {
		if r["id"] == "zed" && r["enabled"] != true {
			t.Fatal("an app the roster does not name must read as themed")
		}
	}
}

// A cold start lands where the event would: tab, section and control, or a collection.
func TestPickerPendingFor(t *testing.T) {
	got := pickerPendingFor([]string{"RYOGAMI_START_SETTINGS=colours", "RYOGAMI_START_SECTION=scheme", "RYOGAMI_START_CONTROL=theme.mode"})
	if got.name != "ryogami.wall.settings" || !reflect.DeepEqual(got.data, map[string]interface{}{"tab": "colours", "section": "scheme", "control": "theme.mode"}) {
		t.Fatalf("settings pending = %+v", got)
	}
	got = pickerPendingFor([]string{"RYOGAMI_START_COLLECTION=rices"})
	if got.name != "ryogami.wall.collection" || got.data["name"] != "rices" {
		t.Fatalf("collection pending = %+v", got)
	}
	if got = pickerPendingFor(nil); got.name != "ryogami.wall.toggle" {
		t.Fatalf("plain pending = %+v", got)
	}
}

func TestCollectionVerbRejectsUnknown(t *testing.T) {
	d := newSettingsDaemon(t)
	if err := d.openCollection("appearance"); err == nil {
		t.Fatal("unknown collection accepted")
	}
}

func TestParseSettingArg(t *testing.T) {
	for in, want := range map[string]interface{}{
		"0.25": 0.25, "true": true, "dark": "dark", `"4"`: "4", `["a"]`: []interface{}{"a"},
	} {
		if got := parseSettingArg(in); !reflect.DeepEqual(got, want) {
			t.Errorf("parseSettingArg(%q) = %#v, want %#v", in, got, want)
		}
	}
}

// one batch is one write per document: the whole roster and the knobs land
// together, null drops a field, and the shell's other fields survive
func TestDocBatchSetWritesOnce(t *testing.T) {
	d := newSettingsDaemon(t)
	writeDoc(t, matugenStorePath(), `{"engine":"matugen","prefer":"value","contrast":0.5,"templates":{"kitty":false,"btop":false}}`)
	before, _ := os.Stat(matugenStorePath())
	req := &request{Method: "settings.set", ID: 1, Params: json.RawMessage(
		`{"values":{"theme.mode":"light","matugen.contrast":null,"matugen.lightnessDark":0.2,"matugen.templates":{"kitty":true},"matugen.themeRyokuApps":false,"transition.durationMs":400}}`)}
	if resp, _ := d.dispatchSettings(req, req.params()); resp.Error != nil {
		t.Fatalf("batch rejected: %+v", resp.Error)
	}
	got := readJSONFile(t, matugenStorePath())
	want := map[string]interface{}{
		"engine": "matugen", "prefer": "value", "mode": "light", "lightnessDark": 0.2,
		"templates": map[string]interface{}{"kitty": true}, "themeRyokuApps": false,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matugen.json = %v, want %v", got, want)
	}
	after, _ := os.Stat(matugenStorePath())
	if os.SameFile(before, after) {
		t.Fatal("the batch never replaced the file")
	}
	if d.settingNumber("transition.durationMs") != 400 {
		t.Fatal("a ryogami.json key in the same batch was lost")
	}

	// a batch that changes nothing leaves the file alone, so the shell does not repaint
	again := &request{Method: "settings.set", ID: 2, Params: json.RawMessage(`{"values":{"theme.mode":"light","matugen.templates":{"kitty":true}}}`)}
	if resp, _ := d.dispatchSettings(again, again.params()); resp.Error != nil {
		t.Fatalf("repeat rejected: %+v", resp.Error)
	}
	if same, _ := os.Stat(matugenStorePath()); !os.SameFile(after, same) {
		t.Fatal("an unchanged batch rewrote matugen.json")
	}
}

// a bad value anywhere refuses the whole batch before a byte is written
func TestDocBatchRefusesBadRoster(t *testing.T) {
	d := newSettingsDaemon(t)
	writeDoc(t, matugenStorePath(), `{"mode":"dark"}`)
	req := &request{Method: "settings.set", ID: 1, Params: json.RawMessage(`{"values":{"theme.mode":"light","matugen.templates":{"kitty":"yes"}}}`)}
	if resp, _ := d.dispatchSettings(req, req.params()); resp.Error == nil {
		t.Fatal("a non-bool roster entry was accepted")
	}
	if got := readJSONFile(t, matugenStorePath())["mode"]; got != "dark" {
		t.Fatalf("a refused batch still wrote mode %v", got)
	}
	for _, bad := range []string{`{"matugen.prefer":"brightest"}`, `{"matugen.lightnessLight":2}`, `{"no.such.key":null}`} {
		req := &request{Method: "settings.set", ID: 2, Params: json.RawMessage(`{"values":` + bad + `}`)}
		if resp, _ := d.dispatchSettings(req, req.params()); resp.Error == nil {
			t.Errorf("%s accepted", bad)
		}
	}
}

// custom outputs follow theme.mode: light and dark pass through, smart reads the wallpaper
func TestCustomOutputModeFollowsThemeMode(t *testing.T) {
	d := newSettingsDaemon(t)
	dir := t.TempDir()
	still := func(name string, c color.RGBA) string {
		img := image.NewRGBA(image.Rect(0, 0, 32, 18))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
		p := filepath.Join(dir, name)
		f, err := os.Create(p)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		return p
	}
	bright, dim := still("bright.png", color.RGBA{230, 230, 220, 255}), still("dim.png", color.RGBA{20, 24, 30, 255})
	if got := d.customOutputMode(bright); got != "light" {
		t.Fatalf("smart (the default) on a bright wall = %s", got)
	}
	if got := d.customOutputMode(dim); got != "dark" {
		t.Fatalf("smart on a dim wall = %s", got)
	}
	if err := d.setSetting("theme.mode", "dark"); err != nil {
		t.Fatal(err)
	}
	if got := d.customOutputMode(bright); got != "dark" {
		t.Fatalf("dark on a bright wall = %s", got)
	}
}

// the hub rewrites its own theme.json fields under the same lock, the pause turns a missing lock into lost counts
func TestThemeDocSharesTheLockWithExternalWriters(t *testing.T) {
	newSettingsDaemon(t)
	writeDoc(t, themeStorePath(), `{}`)
	count := func(m map[string]interface{}, field string) float64 {
		n, _ := m[field].(float64)
		return n
	}
	external := func() error {
		path := themeStorePath()
		lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
			return err
		}
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		m := loadDoc("theme")
		if m == nil {
			return fmt.Errorf("theme.json unreadable under the lock")
		}
		m["hubWrites"] = count(m, "hubWrites") + 1
		time.Sleep(100 * time.Microsecond)
		b, err := json.Marshal(m)
		if err != nil {
			return err
		}
		tmp := path + ".hub.tmp"
		if err := os.WriteFile(tmp, b, 0o644); err != nil {
			return err
		}
		return os.Rename(tmp, path)
	}
	const rounds = 150
	done := make(chan error, 1)
	go func() {
		for range rounds {
			if err := external(); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	var docErr error
	for range rounds {
		docErr = updateDoc("theme", func(m map[string]interface{}) bool {
			m["daemonWrites"] = count(m, "daemonWrites") + 1
			time.Sleep(100 * time.Microsecond)
			return true
		})
		if docErr != nil {
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if docErr != nil {
		t.Fatal(docErr)
	}
	got := readJSONFile(t, themeStorePath())
	if count(got, "daemonWrites") != rounds || count(got, "hubWrites") != rounds {
		t.Fatalf("a writer lost a write: daemon %v, hub %v of %d", got["daemonWrites"], got["hubWrites"], rounds)
	}
}
