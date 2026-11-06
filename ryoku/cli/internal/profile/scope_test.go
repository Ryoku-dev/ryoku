package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHiddenOverrideAndScopeRegeneration(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "system", "app.desktop")
	writeTestFile(t, source, "[Desktop Entry]\nName=App\nNoDisplay=false\nExec=app\n[Other]\nNoDisplay=false\n")
	entry := DesktopEntry{ID: "app.desktop", Path: source, Name: "App"}
	body, err := HiddenOverride(entry)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Count(text, "NoDisplay=true") != 1 || !strings.Contains(text, "[Other]\nNoDisplay=false") {
		t.Fatalf("override = %q", text)
	}
	r := &AccountReceipt{}
	added, removed, err := ApplyScope(home, []DesktopEntry{entry}, map[string]bool{}, r)
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || len(removed) != 0 {
		t.Fatalf("added=%v removed=%v", added, removed)
	}
	override := filepath.Join(home, ".local", "share", "applications", "app.desktop")
	first, _ := os.ReadFile(override)
	writeTestFile(t, source, "[Desktop Entry]\nName=Renamed\nExec=app\n")
	added, _, err = ApplyScope(home, []DesktopEntry{entry}, map[string]bool{}, r)
	if err != nil || len(added) != 1 {
		t.Fatalf("regenerate: added=%v err=%v", added, err)
	}
	second, _ := os.ReadFile(override)
	if string(first) == string(second) || !strings.Contains(string(second), "Name=Renamed") {
		t.Fatal("override was not regenerated")
	}
	unrelated := filepath.Join(home, ".local", "share", "applications", "unrelated.desktop")
	writeTestFile(t, unrelated, "keep\n")
	_, removed, err = ApplyScope(home, []DesktopEntry{entry}, map[string]bool{"app.desktop": true}, r)
	if err != nil || len(removed) != 1 {
		t.Fatalf("remove: %v %v", removed, err)
	}
	if _, err := os.Stat(override); !os.IsNotExist(err) {
		t.Fatal("managed override remains")
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Fatal("unlisted override was removed")
	}
}

func TestMergeDesktopEnvKeepsUserKeys(t *testing.T) {
	input := []byte(`{"desktop":{"appearance":{"theme":"dark"},"env":[{"key":"USER_KEY","value":"mine"},{"key":"OLD_PROFILE","value":"old"}]},"wm":{"niri":{"x":1}}}`)
	out, owned, err := MergeDesktopEnv(input, []string{"OLD_PROFILE"}, []EnvVar{{Key: "PATH", Value: "/profile/bin"}, {Key: "NEW_PROFILE", Value: "new"}})
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(out, &root); err != nil {
		t.Fatal(err)
	}
	desktop := root["desktop"].(map[string]any)
	body, _ := json.Marshal(desktop["env"])
	text := string(body)
	if !strings.Contains(text, "USER_KEY") || strings.Contains(text, "OLD_PROFILE") || !strings.Contains(text, "NEW_PROFILE") {
		t.Fatalf("env = %s", text)
	}
	if root["wm"] == nil || desktop["appearance"] == nil {
		t.Fatal("unrelated desktop.json keys were lost")
	}
	if len(owned) != 2 || owned[0] != "NEW_PROFILE" || owned[1] != "PATH" {
		t.Fatalf("owned = %v", owned)
	}
}

func TestEnvironmentExpandsProfileTokens(t *testing.T) {
	m := &Manifest{ID: "hacker", Path: []string{"@PROFILE_BIN@"}, Env: []EnvVar{{Key: "PLUGIN_DIR", Value: "@HOME@/plugins/@USER@/@ID@"}}}
	v := Vars{Home: "/home/hacker", User: "hacker", ID: "hacker"}
	env := DesktopEnv(m, v)
	if env[1].Value != "/home/hacker/plugins/hacker/hacker" {
		t.Fatalf("desktop env = %q", env[1].Value)
	}
	if body := string(EnvironmentD(m, v)); !strings.Contains(body, "PLUGIN_DIR=/home/hacker/plugins/hacker/hacker") {
		t.Fatalf("environment.d = %q", body)
	}
}
