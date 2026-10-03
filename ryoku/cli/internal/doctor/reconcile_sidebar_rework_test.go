package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"ryoku-cli/internal/sys"
	"testing"
)

const canonicalSidebarsJSON = `{
	"width": 380,
	"motion": "standard",
	"depth": true,
	"push": true,
	"wallpaperSlide": 1.15,
	"left": {"enabled": true, "cards": ["system", "notifications", "weather", "media", "capture", "stage"]},
	"right": {"enabled": true, "cards": ["usage", "tools", "chat"]}
}`

func TestMigrateSidebarRework(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		changed bool
		wantErr bool
	}{
		{
			name:    "retired leaves removed and defaults seeded",
			in:      `{"frameBars":{"version":1,"menus":{"quick-settings":{"anchor":"left","modules":["home","stage"]},"wallpaper":{"anchor":"bottom"}},"surfaces":{"stash":{"anchor":"right","panes":["stash"]},"future":{"anchor":"top"}},"dock":{"pinned":["firefox"]}},"fontScale":1.3}`,
			want:    `{"frameBars":{"version":1,"menus":{"wallpaper":{"anchor":"bottom"}},"surfaces":{"future":{"anchor":"top"}},"dock":{"pinned":["firefox"]}},"fontScale":1.3,"sidebars":` + canonicalSidebarsJSON + `}`,
			changed: true,
		},
		{
			name:    "missing frame bars seeds defaults",
			in:      `{"weatherLocation":"Oslo","fontScale":0.96}`,
			want:    `{"weatherLocation":"Oslo","fontScale":0.96,"sidebars":` + canonicalSidebarsJSON + `}`,
			changed: true,
		},
		{
			name:    "existing sidebar settings stay authoritative",
			in:      `{"frameBars":{"menus":{"quick-settings":{},"theme":{"anchor":"right"}},"surfaces":{"stash":{},"system":{"anchor":"right"}}},"sidebars":{"width":512,"motion":"calm","left":{"enabled":false,"cards":["weather"]},"extension":{"future":true}}}`,
			want:    `{"frameBars":{"menus":{"theme":{"anchor":"right"}},"surfaces":{"system":{"anchor":"right"}}},"sidebars":{"width":512,"motion":"calm","left":{"enabled":false,"cards":["weather"]},"extension":{"future":true}}}`,
			changed: true,
		},
		{
			name:    "quick-settings leaf retires independently",
			in:      `{"frameBars":{"menus":{"quick-settings":{"future":true}},"surfaces":{"future":{"anchor":"top"}}},"sidebars":{}}`,
			want:    `{"frameBars":{"menus":{},"surfaces":{"future":{"anchor":"top"}}},"sidebars":{}}`,
			changed: true,
		},
		{
			name:    "stash leaf retires independently",
			in:      `{"frameBars":{"menus":{"theme":{"anchor":"right"}},"surfaces":{"stash":{"future":true}}},"sidebars":{}}`,
			want:    `{"frameBars":{"menus":{"theme":{"anchor":"right"}},"surfaces":{}},"sidebars":{}}`,
			changed: true,
		},
		{
			name:    "current store is unchanged",
			in:      `{"frameBars":{"menus":{"theme":{"anchor":"right"}},"surfaces":{"future":{"anchor":"top"}}},"sidebars":` + canonicalSidebarsJSON + `,"custom":{"nested":[1,2,3]}}`,
			changed: false,
		},
		{
			name:    "invalid json errors",
			in:      `not json`,
			wantErr: true,
		},
		{
			name:    "null top level errors",
			in:      `null`,
			wantErr: true,
		},
		{
			name:    "malformed retired parent errors",
			in:      `{"frameBars":{"menus":"not-an-object"}}`,
			wantErr: true,
		},
		{
			name:    "malformed surface parent errors",
			in:      `{"frameBars":{"surfaces":"not-an-object"}}`,
			wantErr: true,
		},
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
			if !tt.changed {
				if out != nil {
					t.Fatalf("no-op migration returned output: %s", out)
				}
				return
			}

			var got, want map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("migrated JSON does not parse: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.want), &want); err != nil {
				t.Fatalf("test expectation does not parse: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("migrated store = %#v, want %#v", got, want)
			}

			if second, changed, err := migrateSidebarRework(out); err != nil || changed || second != nil {
				t.Errorf("second migration must be a no-op: changed=%v out=%s err=%v", changed, second, err)
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

	stored := `{"frameBars":{"menus":{"quick-settings":{"modules":["home"]},"theme":{"anchor":"right"}},"surfaces":{"stash":{"anchor":"right"},"future":{"anchor":"top"}}},"theme":"paper"}`
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
	frameBars := cfg["frameBars"].(map[string]any)
	menus := frameBars["menus"].(map[string]any)
	if _, present := menus["quick-settings"]; present {
		t.Error("fix left the retired quick-settings menu in place")
	}
	if _, present := menus["theme"]; !present {
		t.Error("fix removed a sibling menu")
	}
	surfaces := frameBars["surfaces"].(map[string]any)
	if _, present := surfaces["stash"]; present {
		t.Error("fix left the retired stash surface in place")
	}
	if _, present := surfaces["future"]; !present {
		t.Error("fix removed a sibling surface")
	}
	if cfg["theme"] != "paper" {
		t.Errorf("fix changed an unrelated top-level key: %v", cfg["theme"])
	}
	var wantSidebars map[string]any
	if err := json.Unmarshal([]byte(canonicalSidebarsJSON), &wantSidebars); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg["sidebars"], wantSidebars) {
		t.Errorf("sidebars = %#v, want %#v", cfg["sidebars"], wantSidebars)
	}
	if r := reconcileSidebarRework(false); r.status != recOK {
		t.Fatalf("clean store: status=%s detail=%q, want ok", r.status.label(), r.detail)
	}
}
