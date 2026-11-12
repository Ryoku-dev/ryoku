package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

// recordRyogamiSets stubs riceRun and returns the batches handed to `ryogami settings set`.
func recordRyogamiSets(t *testing.T) *[]map[string]any {
	t.Helper()
	var sets []map[string]any
	origRun, origReload := riceRun, riceReload
	riceRun = func(name string, args ...string) error {
		if name == "ryogami" && len(args) == 3 && args[0] == "settings" && args[1] == "set" {
			var batch map[string]any
			if err := json.Unmarshal([]byte(args[2]), &batch); err != nil {
				t.Fatalf("batch is not a JSON object: %q", args[2])
			}
			sets = append(sets, batch)
		}
		return nil
	}
	riceReload = func() {}
	t.Cleanup(func() { riceRun, riceReload = origRun, origReload })
	return &sets
}

func matugenHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// matugen set no longer writes the file: it names each field's ryogami key, and
// the roster it sends is merged onto the stored one as before
func TestMatugenSetGoesThroughRyogami(t *testing.T) {
	matugenHome(t)
	sets := recordRyogamiSets(t)
	stored := `{"mode":"dark","templates":{"kitty":false,"btop":true}}`
	writeTestFile(t, matugenConfigPath(), stored)

	if err := runMatugenCmd([]string{"set", `{"mode":"light","contrast":0.3,"sourceColorIndex":2,"templates":{"btop":false}}`}); err != nil {
		t.Fatal(err)
	}
	if len(*sets) != 1 {
		t.Fatalf("want one batch, got %v", *sets)
	}
	got := (*sets)[0]
	want := map[string]any{
		"theme.mode": "light", "matugen.contrast": 0.3, "matugen.colorIndex": float64(2),
		"matugen.templates": map[string]any{"kitty": false, "btop": false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batch = %v, want %v", got, want)
	}
	if b, _ := os.ReadFile(matugenConfigPath()); string(b) != stored {
		t.Fatalf("matugen set wrote the file itself: %s", b)
	}
	if err := runMatugenCmd([]string{"set", `{"engine":"other"}`}); err == nil || !strings.Contains(err.Error(), "engine") {
		t.Fatalf("an unknown field must be refused, got %v", err)
	}
}

// a rice's palette engine lands through the daemon in one batch; a field from an
// older schema is dropped, and matugen.json itself is never written by the hub
func TestRiceApplyAndRestoreMatugenThroughRyogami(t *testing.T) {
	dir := matugenHome(t)
	sets := recordRyogamiSets(t)
	writeDesktopStore(t, `{"appearance":{"rounding":2}}`)
	writeTestFile(t, shellStorePath(), `{}`)
	writeTestFile(t, launcherStorePath(), `{}`)
	writeTestFile(t, themeStatePath(), `{"followWallpaper":true}`)
	writeTestFile(t, matugenStorePath(), `{"mode":"dark","schemeType":"scheme-fidelity","engine":"matugen"}`)
	if err := os.MkdirAll(filepath.Join(ricesDir(), "pal"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, ricePath("pal"), `{"schema":1,"slug":"pal","name":"Pal","color":{"mode":"wallpaper"},
	  "look":{"hypr":{},"shell":{},"launcher":{},
	          "matugen":{"mode":"smart","prefer":"value","themeRyokuApps":false,"templates":{"kitty":true},"engine":"matugen"}}}`)

	if err := applyRice("pal", nil); err != nil {
		t.Fatal(err)
	}
	if len(*sets) != 1 {
		t.Fatalf("want one batch from apply, got %v", *sets)
	}
	want := map[string]any{
		"theme.mode": "smart", "matugen.prefer": "value", "matugen.themeRyokuApps": false,
		"matugen.templates": map[string]any{"kitty": true},
	}
	if !reflect.DeepEqual((*sets)[0], want) {
		t.Fatalf("apply batch = %v, want %v", (*sets)[0], want)
	}
	if got := readJSONMap(matugenStorePath()); got["schemeType"] != "scheme-fidelity" || got["mode"] != "dark" {
		t.Fatalf("apply wrote matugen.json itself: %v", got)
	}

	// the revert sends every field back, the ones the snapshot lacked as null
	if err := restoreRice(".previous"); err != nil {
		t.Fatal(err)
	}
	if len(*sets) != 2 {
		t.Fatalf("want a restore batch, got %v", *sets)
	}
	back := (*sets)[1]
	if back["theme.mode"] != "dark" || back["matugen.schemeType"] != "scheme-fidelity" {
		t.Fatalf("restore batch lost the saved knobs: %v", back)
	}
	for _, key := range []string{"matugen.prefer", "matugen.templates", "matugen.themeRyokuApps", "matugen.contrast"} {
		if v, ok := back[key]; !ok || v != nil {
			t.Fatalf("restore must reset %s with null: %v", key, back)
		}
	}
	if len(back) != len(matugenSettingKeys) {
		t.Fatalf("restore batch has %d keys, want %d", len(back), len(matugenSettingKeys))
	}
	if _, err := os.Stat(filepath.Join(dir, "ryoku", "matugen.json.tmp")); err == nil {
		t.Fatal("stray temp file")
	}
}

// the hub writes only its own theme.json fields; the daemon's go to it as one batch
func TestThemeStateSplitsWriters(t *testing.T) {
	matugenHome(t)
	sets := recordRyogamiSets(t)
	writeTestFile(t, themeStatePath(), `{"followWallpaper":true,"scheme":"","gtkTheme":"system","themeApps":true}`)
	off := false
	if err := saveThemeState(themeState{FollowWallpaper: false, Scheme: "dark", GtkTheme: "adwaita", ThemeApps: &off}); err != nil {
		t.Fatal(err)
	}
	got := readJSONMap(themeStatePath())
	if got["followWallpaper"] != false || got["scheme"] != "dark" || got["gtkTheme"] != "system" || got["themeApps"] != true {
		t.Fatalf("theme.json = %v: the hub must write only its own fields", got)
	}
	want := map[string]any{"theme.gtkTheme": "adwaita", "theme.themeApps": false}
	if len(*sets) != 1 || !reflect.DeepEqual((*sets)[0], want) {
		t.Fatalf("batches = %v, want one %v", *sets, want)
	}
	// unchanged daemon fields send nothing
	if err := saveThemeState(themeState{FollowWallpaper: true, Scheme: "", GtkTheme: "system"}); err != nil {
		t.Fatal(err)
	}
	if len(*sets) != 1 {
		t.Fatalf("an unchanged daemon field was sent: %v", *sets)
	}
}

// a restore puts the hub's fields back as saved and sends the daemon's, resetting what the backup lacked
func TestRestoreThemeThroughRyogami(t *testing.T) {
	dir := matugenHome(t)
	sets := recordRyogamiSets(t)
	writeTestFile(t, themeStatePath(), `{"followWallpaper":true,"scheme":"","gtkTheme":"system","themeApps":false,"stray":1}`)
	backup := filepath.Join(dir, "theme-backup.json")
	writeTestFile(t, backup, `{"followWallpaper":false,"scheme":"mono","gtkTheme":"adwaita"}`)
	if err := restoreTheme(backup); err != nil {
		t.Fatal(err)
	}
	got := readJSONMap(themeStatePath())
	if got["followWallpaper"] != false || got["scheme"] != "mono" {
		t.Fatalf("hub fields not restored: %v", got)
	}
	if _, ok := got["stray"]; ok {
		t.Fatalf("a field the backup lacked survived: %v", got)
	}
	if got["gtkTheme"] != "system" || got["themeApps"] != false {
		t.Fatalf("the hub rewrote daemon fields: %v", got)
	}
	want := map[string]any{"theme.gtkTheme": "adwaita", "theme.themeApps": nil}
	if len(*sets) != 1 || !reflect.DeepEqual((*sets)[0], want) {
		t.Fatalf("batches = %v, want %v", *sets, want)
	}
}

// daemonRun stands in for riceRun with a ryogami that lands theme.* batches in theme.json, as the daemon does
func daemonRun(name string, args ...string) error {
	if name != "ryogami" || len(args) != 3 || args[0] != "settings" || args[1] != "set" {
		return nil
	}
	var batch map[string]any
	if err := json.Unmarshal([]byte(args[2]), &batch); err != nil {
		return err
	}
	doc := readJSONMap(themeStatePath())
	for field, key := range themeSettingKeys {
		v, ok := batch[key]
		switch {
		case !ok:
		case v == nil:
			delete(doc, field)
		default:
			doc[field] = v
		}
	}
	return atomicWrite(themeStatePath(), mustJSON(doc), 0o644)
}

// the daemon is another module so this copies updateDoc's lock, its own test runs the real one against ours
// the pause inside the lock turns a hub write that skips the lock into lost daemon increments
func TestThemeWritersShareTheLock(t *testing.T) {
	matugenHome(t)
	recordRyogamiSets(t)
	writeTestFile(t, themeStatePath(), `{}`)
	daemonWrite := func() error {
		path := themeStatePath()
		lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
			return err
		}
		defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		doc := readJSONMap(path)
		n, _ := doc["daemonWrites"].(float64)
		time.Sleep(100 * time.Microsecond)
		doc["daemonWrites"] = n + 1
		return atomicWrite(path, mustJSON(doc), 0o644)
	}
	const rounds = 150
	done := make(chan error, 1)
	go func() {
		for range rounds {
			if err := daemonWrite(); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	var hubErr error
	for i := range rounds {
		if hubErr = writeThemeFields(map[string]any{"scheme": fmt.Sprintf("s%d", i)}); hubErr != nil {
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if hubErr != nil {
		t.Fatal(hubErr)
	}
	got := readJSONMap(themeStatePath())
	if got["scheme"] != fmt.Sprintf("s%d", rounds-1) || got["daemonWrites"] != float64(rounds) {
		t.Fatalf("a writer lost a write: scheme %v, daemon writes %v of %d", got["scheme"], got["daemonWrites"], rounds)
	}
}
