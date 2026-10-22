package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func caelestiaTestHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return filepath.Join(dir, "ryoku", "caelestia", "shell.json")
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestApplyCaelestiaDraftWritesOnlyDifferences(t *testing.T) {
	defaults := map[string]any{"bar": map[string]any{"persistent": true, "scale": 1.0, "workspaces": map[string]any{"shown": 5.0}}}
	file := map[string]any{"bar": map[string]any{"scale": 1.4, "customKey": "kept"}}
	draft := map[string]any{"bar": map[string]any{"persistent": false, "scale": 1.0, "workspaces": map[string]any{"shown": 5.0}}}
	if err := applyCaelestiaDraft(defaults, draft, file, ""); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"bar": map[string]any{"persistent": false, "customKey": "kept"}}
	if !reflect.DeepEqual(file, want) {
		t.Fatalf("file = %v, want %v", file, want)
	}
}

func TestApplyCaelestiaDraftPrunesEmptyObjects(t *testing.T) {
	defaults := map[string]any{"bar": map[string]any{"workspaces": map[string]any{"shown": 5.0}}}
	file := map[string]any{"bar": map[string]any{"workspaces": map[string]any{"shown": 8.0}}}
	draft := map[string]any{"bar": map[string]any{"workspaces": map[string]any{"shown": 5.0}}}
	if err := applyCaelestiaDraft(defaults, draft, file, ""); err != nil {
		t.Fatal(err)
	}
	if len(file) != 0 {
		t.Fatalf("file = %v, want empty", file)
	}
}

func TestApplyCaelestiaDraftRejectsWrongType(t *testing.T) {
	defaults := map[string]any{"bar": map[string]any{"persistent": true}}
	draft := map[string]any{"bar": map[string]any{"persistent": "yes"}}
	if err := applyCaelestiaDraft(defaults, draft, map[string]any{}, ""); err == nil {
		t.Fatal("a string for a boolean option was accepted")
	}
}

func TestCaelestiaSaveThenGet(t *testing.T) {
	path := caelestiaTestHome(t)
	defaults, err := caelestiaDefaultsTree()
	if err != nil {
		t.Fatal(err)
	}
	bar, ok := defaults["bar"].(map[string]any)
	if !ok {
		t.Fatal("embedded defaults carry no bar section")
	}
	draft := mergeTrees(defaults, map[string]any{"bar": map[string]any{"persistent": false}})
	body, _ := json.Marshal(map[string]any{"caelestia": draft})
	if err := runCaelestia([]string{"save", string(body)}); err != nil {
		t.Fatal(err)
	}
	saved := readJSONFile(t, path)
	if !reflect.DeepEqual(saved, map[string]any{"bar": map[string]any{"persistent": false}}) {
		t.Fatalf("saved file = %v, want only the changed option (default persistent %v)", saved, bar["persistent"])
	}
}

func TestCaelestiaFileThatDoesNotParseBlocksSave(t *testing.T) {
	path := caelestiaTestHome(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCaelestia([]string{"save", `{"caelestia":{}}`}); err == nil {
		t.Fatal("save went ahead over a file it could not read")
	}
	if b, _ := os.ReadFile(path); string(b) != "{ not json" {
		t.Fatalf("the unreadable file was changed: %q", b)
	}
}

// Every row the generator wrote must name an option the embedded defaults carry, with
// the type the row says, or the Hub would show a control the file could never hold.
func TestCaelestiaSchemaRowsMatchTheDefaults(t *testing.T) {
	defaults, err := caelestiaDefaultsTree()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../quickshell/schema/Caelestia*Page.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no generated schema files found: %v", err)
	}
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		body := string(src)
		body = body[strings.Index(body, "var rows = ")+len("var rows = "):]
		var rows []struct {
			Key string `json:"key"`
			Def any    `json:"def"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimSpace(body), ";")), &rows); err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, row := range rows {
			var cur any = defaults
			for _, part := range strings.Split(strings.TrimPrefix(row.Key, "caelestia."), ".") {
				// "@dock" picks the list element whose id is "dock" (the bar components)
				if id, isID := strings.CutPrefix(part, "@"); isID {
					list, ok := cur.([]any)
					if !ok {
						t.Fatalf("%s: %s has no default", file, row.Key)
					}
					cur = nil
					for _, el := range list {
						if m, ok := el.(map[string]any); ok && m["id"] == id {
							cur = m
						}
					}
					if cur == nil {
						t.Fatalf("%s: %s has no default", file, row.Key)
					}
					continue
				}
				m, ok := cur.(map[string]any)
				if !ok {
					t.Fatalf("%s: %s has no default", file, row.Key)
				}
				cur, ok = m[part]
				if !ok {
					t.Fatalf("%s: %s has no default", file, row.Key)
				}
			}
			if !reflect.DeepEqual(cur, row.Def) {
				t.Errorf("%s: %s default is %v in the defaults but %v on the row", file, row.Key, cur, row.Def)
			}
		}
	}
}

// status answers from the plugin's qmldir on the import path, so the Hub can tell
// whether picking the style needs the package first.
func TestCaelestiaPluginInstalledReadsTheHomeImportPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sys := caelestiaPluginInstalled() // a system copy may exist on the build box
	dir := filepath.Join(home, ".local", "lib", "qt6", "qml", "Caelestia")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "qmldir"), []byte("module Caelestia\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !caelestiaPluginInstalled() {
		t.Fatalf("a qmldir under ~/.local/lib/qt6/qml reads as not installed (system copy: %v)", sys)
	}
}
