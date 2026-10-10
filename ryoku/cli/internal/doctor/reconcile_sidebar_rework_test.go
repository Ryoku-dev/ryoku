package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"
	"testing"
)

func TestMigrateSidebarRework(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		changed bool
		wantErr bool
	}{
		{
			name:    "retires global tree and original frame surfaces",
			in:      `{"frameBars":{"version":1,"rails":{"top":{"enabled":true}},"menus":{"quick-settings":{"anchor":"left","modules":["home"]},"theme":{"anchor":"right"}},"surfaces":{"stash":{"anchor":"right"},"system":{"anchor":"right"},"future":{"anchor":"top"}}},"sidebars":{"layout":"classic","left":{"cards":["weather"],"geometry":{"DP-1":{"x":2}}},"right":{"pinned":true}},"controls":{"sections":[{"id":"levels","visible":true},{"id":"vitals","visible":false}],"hidden":["liveGraph"]},"custom":{"nested":[1,2,3]},"fontScale":1.3}`,
			changed: true,
		},
		{
			name:    "retires malformed sidebar tree as one obsolete value",
			in:      `{"sidebars":"not-an-object","weatherLocation":"Oslo"}`,
			changed: true,
		},
		{
			name:    "clean store is unchanged",
			in:      `{"frameBars":{"menus":{"theme":{"anchor":"right"}},"surfaces":{"future":{"anchor":"top"}}},"custom":{"nested":[1,2,3]}}`,
			changed: false,
		},
		{name: "invalid json errors", in: `not json`, wantErr: true},
		{name: "null top level errors", in: `null`, wantErr: true},
		{name: "malformed frame bars errors", in: `{"frameBars":"not-an-object"}`, wantErr: true},
		{name: "malformed menus errors", in: `{"frameBars":{"menus":"not-an-object"}}`, wantErr: true},
		{name: "malformed surfaces errors", in: `{"frameBars":{"surfaces":"not-an-object"}}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, changed, err := migrateSidebarRework([]byte(tt.in))
			if tt.wantErr {
				if err == nil {
					t.Fatal("migration succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("migration failed: %v", err)
			}
			if changed != tt.changed {
				t.Fatalf("changed = %v, want %v", changed, tt.changed)
			}
			if !changed {
				if out != nil {
					t.Fatalf("no-op migration returned output: %s", out)
				}
				return
			}

			var before, after map[string]any
			if err := json.Unmarshal([]byte(tt.in), &before); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(out, &after); err != nil {
				t.Fatalf("migrated JSON does not parse: %v", err)
			}
			if _, present := after["sidebars"]; present {
				t.Fatal("retired global sidebars tree survived")
			}
			if before["weatherLocation"] != nil && after["weatherLocation"] != before["weatherLocation"] {
				t.Fatalf("weather location changed: before=%v after=%v", before["weatherLocation"], after["weatherLocation"])
			}
			if before["custom"] != nil {
				got, _ := json.Marshal(after["custom"])
				want, _ := json.Marshal(before["custom"])
				if string(got) != string(want) {
					t.Fatalf("unrelated custom settings changed: before=%s after=%s", want, got)
				}
			}
			if before["controls"] != nil {
				got, _ := json.Marshal(after["controls"])
				want, _ := json.Marshal(before["controls"])
				if string(got) != string(want) {
					t.Fatalf("current controls settings changed: before=%s after=%s", want, got)
				}
			}
			if frameBars, ok := after["frameBars"].(map[string]any); ok {
				if menus, ok := frameBars["menus"].(map[string]any); ok {
					if _, present := menus["quick-settings"]; present {
						t.Fatal("retired quick-settings frame menu survived")
					}
					if _, present := menus["theme"]; !present {
						t.Fatal("sibling frame menu was removed")
					}
				}
				if surfaces, ok := frameBars["surfaces"].(map[string]any); ok {
					for _, key := range []string{"stash", "system"} {
						if _, present := surfaces[key]; present {
							t.Fatalf("retired %s frame surface survived", key)
						}
					}
					if _, present := surfaces["future"]; !present {
						t.Fatal("sibling frame surface was removed")
					}
				}
				if _, present := frameBars["rails"]; !present && before["frameBars"] != nil {
					beforeFrameBars, _ := before["frameBars"].(map[string]any)
					if _, hadRails := beforeFrameBars["rails"]; hadRails {
						t.Fatal("unrelated frame rails were removed")
					}
				}
			}
			if second, changed, err := migrateSidebarRework(out); err != nil || changed || second != nil {
				t.Fatalf("second migration must be a no-op: changed=%v out=%s err=%v", changed, second, err)
			}
		})
	}
}

func TestReconcileSidebarRework(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if r := reconcileSidebarRework(false); r.status != recOK {
		t.Fatalf("missing shell.json: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}

	stored := `{"frameBars":{"menus":{"quick-settings":{"modules":["home"]},"theme":{"anchor":"right"}},"surfaces":{"stash":{"anchor":"right"},"system":{"anchor":"right"},"future":{"anchor":"top"}}},"sidebars":{"layout":"classic"},"controls":{"sections":[{"id":"levels","visible":true}],"hidden":["liveGraph"]},"theme":"paper"}`
	if err := os.WriteFile(path, []byte(stored), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileSidebarRework(true); r.status != recWouldFix {
		t.Fatalf("check with retired records: status=%s detail=%q, want todo", r.status.label(), r.detail)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != stored {
		t.Fatalf("check-only changed shell.json: got=%q err=%v", got, err)
	}
	if r := reconcileSidebarRework(false); r.status != recFixed {
		t.Fatalf("fix with retired records: status=%s detail=%q, want fixed", r.status.label(), r.detail)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("rewritten shell.json does not parse: %v", err)
	}
	if _, present := cfg["sidebars"]; present {
		t.Fatal("fix left the retired global sidebar tree in place")
	}
	controls := cfg["controls"].(map[string]any)
	if hidden := controls["hidden"].([]any); len(hidden) != 1 || hidden[0] != "liveGraph" {
		t.Fatalf("fix changed current Controls settings: %v", controls)
	}
	frameBars := cfg["frameBars"].(map[string]any)
	menus := frameBars["menus"].(map[string]any)
	if _, present := menus["quick-settings"]; present {
		t.Error("fix left the retired quick-settings menu in place")
	}
	if _, present := menus["theme"]; !present {
		t.Error("fix removed a sibling menu")
	}
	surfaces := frameBars["surfaces"].(map[string]any)
	for _, key := range []string{"stash", "system"} {
		if _, present := surfaces[key]; present {
			t.Errorf("fix left the retired %s surface in place", key)
		}
	}
	if _, present := surfaces["future"]; !present {
		t.Error("fix removed a sibling surface")
	}
	if cfg["theme"] != "paper" {
		t.Errorf("fix changed an unrelated top-level key: %v", cfg["theme"])
	}
	if r := reconcileSidebarRework(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}
