package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-cli/internal/sys"
)

func TestThemeColoursPolicyReconciler(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"theme": map[string]any{"colours": "ryoku"}}
	b, _ := json.Marshal(doc)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	// With no Ryogami binary in PATH the reconciler reports nothing to do.
	got := reconcileThemeColoursPolicy(true)
	if got.status != recOK {
		t.Fatalf("status = %v, detail = %s", got.status, got.detail)
	}
}

func TestThemeColoursPolicyRepairsMismatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	ryogami := `#!/bin/sh
if [ "$1" = settings ] && [ "$2" = get ]; then printf '"wallpaper"\n'; fi
`
	ryoku := `#!/bin/sh
printf '%s\n' "$*" > "` + home + `/called"
`
	if err := os.WriteFile(filepath.Join(bin, "ryogami"), []byte(ryogami), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "ryoku-shell"), []byte(ryoku), 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"theme": map[string]any{"colours": "ryoku"}})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := reconcileThemeColoursPolicy(false); got.status != recOK {
		t.Fatalf("status = %v, detail = %s", got.status, got.detail)
	}
	called, err := os.ReadFile(filepath.Join(home, "called"))
	if err != nil {
		t.Fatal(err)
	}
	if string(called) != "colours wallpaper\n" {
		t.Fatalf("called = %q", called)
	}
}

func TestThemeColoursPolicyWarnsOnMalformedShellSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("PATH", t.TempDir())
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := reconcileThemeColoursPolicy(true); got.status != recWarn {
		t.Fatalf("status = %v, detail = %s", got.status, got.detail)
	}
}

func TestThemeColoursPolicyWarnsOnRyogamiFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	ryogami := "#!/bin/sh\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "ryogami"), []byte(ryogami), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"theme": map[string]any{"colours": "ryoku"}})
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	got := reconcileThemeColoursPolicy(true)
	if got.status != recWarn {
		t.Fatalf("status = %v, detail = %s", got.status, got.detail)
	}
	if strings.Contains(got.detail, "not installed") {
		t.Fatalf("detail = %q, should not report a missing binary", got.detail)
	}
}
