package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ryoku-cli/internal/sys"
)

func TestTakeRetiredMatugenMode(t *testing.T) {
	out, carried, changed, err := takeRetiredMatugenMode([]byte(`{"matugen":{"mode":"auto","videoFrame":2},"general":{"uiScale":1}}`))
	if err != nil || !changed || carried != "smart" {
		t.Fatalf("changed=%v carried=%q err=%v, want the auto mode carried as smart", changed, carried, err)
	}
	var cfg map[string]map[string]any
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("stripped JSON does not parse: %v", err)
	}
	if _, present := cfg["matugen"]["mode"]; present {
		t.Errorf("matugen.mode survived: %v", cfg["matugen"])
	}
	if cfg["matugen"]["videoFrame"] != 2.0 || cfg["general"]["uiScale"] != 1.0 {
		t.Errorf("live keys were lost: %v", cfg)
	}
	if _, _, changed, err := takeRetiredMatugenMode(out); err != nil || changed {
		t.Errorf("a clean store must be a no-op: changed=%v err=%v", changed, err)
	}

	out, carried, changed, err = takeRetiredMatugenMode([]byte(`{"matugen":{"mode":"sepia"}}`))
	if err != nil || !changed || carried != "" {
		t.Fatalf("changed=%v carried=%q err=%v, want an unknown mode stripped and not carried", changed, carried, err)
	}
	if bytes.Contains(out, []byte("matugen")) {
		t.Errorf("a matugen object left empty must be dropped: %s", out)
	}

	if _, _, _, err := takeRetiredMatugenMode([]byte(`null`)); err == nil {
		t.Error("a null store must error rather than be rewritten")
	}
}

func TestReconcileMatugenDoc(t *testing.T) {
	cases := []struct {
		name, in, carried string
		absent            bool
		want              map[string]any // nil: unchanged
	}{
		{name: "old spellings", in: `{"mode":"auto","schemeType":"scheme-smart","prefer":"value"}`, carried: "dark",
			want: map[string]any{"mode": "smart", "schemeType": "scheme-tonal-spot", "prefer": "value"}},
		{name: "set mode wins over the carry", in: `{"mode":"light"}`, carried: "dark"},
		{name: "unset mode takes the carry", in: `{"contrast":0.5}`, carried: "dark",
			want: map[string]any{"mode": "dark", "contrast": 0.5}},
		{name: "absent file takes the carry", absent: true, carried: "sun", want: map[string]any{"mode": "sun"}},
		{name: "absent file and no carry", absent: true},
		{name: "current values", in: `{"mode":"smart","schemeType":"scheme-vibrant"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw []byte
			if !tc.absent {
				raw = []byte(tc.in)
			}
			out, changed, err := reconcileMatugenDoc(raw, tc.carried)
			if err != nil {
				t.Fatal(err)
			}
			if changed != (tc.want != nil) {
				t.Fatalf("changed=%v out=%s", changed, out)
			}
			if tc.want == nil {
				return
			}
			var got map[string]any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("rewritten JSON does not parse: %v", err)
			}
			if !sameMaps(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// check-only leaves both files' bytes, the fix carries and strips, and a rerun is ok.
func TestReconcileRetiredMatugenMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	dir := filepath.Join(sys.ConfigHome(), "ryoku")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ryogamiPath, matugenPath := filepath.Join(dir, "ryogami.json"), filepath.Join(dir, "matugen.json")

	if r := reconcileRetiredMatugenMode(false); r.status != recOK {
		t.Fatalf("no stores: status=%s detail=%q", r.status.label(), r.detail)
	}

	ryogami := []byte(`{"matugen":{"mode":"light","videoFrame":3},"paths":{"wallpaper":"/w"}}`)
	matugen := []byte(`{"schemeType":"scheme-smart","templates":{"kitty":false}}`)
	if err := os.WriteFile(ryogamiPath, ryogami, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(matugenPath, matugen, 0o644); err != nil {
		t.Fatal(err)
	}

	if r := reconcileRetiredMatugenMode(true); r.status != recWouldFix {
		t.Fatalf("check-only: status=%s detail=%q", r.status.label(), r.detail)
	}
	if b, _ := os.ReadFile(ryogamiPath); !bytes.Equal(b, ryogami) {
		t.Fatalf("check-only rewrote ryogami.json: %s", b)
	}
	if b, _ := os.ReadFile(matugenPath); !bytes.Equal(b, matugen) {
		t.Fatalf("check-only rewrote matugen.json: %s", b)
	}

	if r := reconcileRetiredMatugenMode(false); r.status != recFixed {
		t.Fatalf("fix: status=%s detail=%q", r.status.label(), r.detail)
	}
	var gotRyogami, gotMatugen map[string]any
	readJSONInto(t, ryogamiPath, &gotRyogami)
	readJSONInto(t, matugenPath, &gotMatugen)
	if !sameMaps(gotRyogami, map[string]any{"matugen": map[string]any{"videoFrame": 3.0}, "paths": map[string]any{"wallpaper": "/w"}}) {
		t.Fatalf("ryogami.json = %v", gotRyogami)
	}
	if !sameMaps(gotMatugen, map[string]any{"mode": "light", "schemeType": "scheme-tonal-spot", "templates": map[string]any{"kitty": false}}) {
		t.Fatalf("matugen.json = %v", gotMatugen)
	}

	if r := reconcileRetiredMatugenMode(false); r.status != recOK {
		t.Fatalf("rerun: status=%s detail=%q", r.status.label(), r.detail)
	}

	if err := os.WriteFile(matugenPath, []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := reconcileRetiredMatugenMode(false); r.status != recWarn {
		t.Fatalf("broken matugen.json: status=%s detail=%q", r.status.label(), r.detail)
	}
}

func readJSONInto(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s does not parse: %v", path, err)
	}
}

func sameMaps(a, b map[string]any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}
