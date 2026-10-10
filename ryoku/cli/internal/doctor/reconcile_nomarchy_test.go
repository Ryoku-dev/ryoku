package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileNomarchyCommandsFollowsBarStyle(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, "config")
	data := filepath.Join(home, "data")
	source := filepath.Join(home, "source")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("RYOKU_NOMARCHY_SOURCE", source)

	for _, dir := range []string{
		filepath.Join(source, "bin"),
		filepath.Join(source, "default"),
		filepath.Join(source, "config", "omarchy"),
		filepath.Join(source, "themes"),
		filepath.Join(config, "ryoku"),
		filepath.Join(config, "quickshell", "shell", "modules", "bar", "barstyles", "nomarchy", "services"),
		filepath.Join(config, "quickshell", "shell", "modules", "bar", "barstyles", "nomarchy", "plugins"),
		filepath.Join(config, "quickshell", "shell", "Commons"),
		filepath.Join(config, "quickshell", "shell", "Ui"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, body string, mode os.FileMode) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"omarchy", "omarchy-shell", "omarchy-example"} {
		write(filepath.Join(source, "bin", name), "#!/bin/bash\n", 0o755)
	}
	write(filepath.Join(source, "bin", "omarchy-theme-set"), "#!/bin/bash\nset -eu\nstate=${XDG_STATE_HOME:-$HOME/.local/state}/omarchy/current/theme\nmkdir -p \"$state\"\nprintf 'accent = \"#ffffff\"\\n' >\"$state/colors.toml\"\nprintf 'bar-position = \"top\"\\n' >\"$state/shell.toml\"\n", 0o755)
	write(filepath.Join(source, "config", "omarchy", "shell.json"), "{}\n", 0o644)
	write(filepath.Join(config, "quickshell", "shell", "modules", "bar", "barstyles", "nomarchy", "Host.qml"), "import QtQuick\nItem {}\n", 0o644)

	for _, name := range []string{"nomarchy-activate", "nomarchy-deactivate"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "..", "shell", "nomarchy", name))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(source, name), string(body), 0o755)
	}

	write(filepath.Join(config, "ryoku", "shell.json"), "{\"barStyle\":\"nomarchy\"}\n", 0o644)
	if got := reconcileNomarchyCommands(false); got.status != recFixed {
		t.Fatalf("activate status = %v: %s", got.status, got.detail)
	}
	for _, name := range []string{"omarchy", "omarchy-shell", "omarchy-example", "omarchy-theme-set"} {
		path := filepath.Join(home, ".local", "bin", name)
		if target, err := os.Readlink(path); err != nil || target != filepath.Join(data, "ryoku", "nomarchy", "bin", name) {
			t.Fatalf("%s link = %q, %v", name, target, err)
		}
	}
	if got := reconcileNomarchyCommands(false); got.status != recOK {
		t.Fatalf("second activate status = %v: %s", got.status, got.detail)
	}

	write(filepath.Join(config, "ryoku", "shell.json"), "{\"barStyle\":\"qsbar\"}\n", 0o644)
	if got := reconcileNomarchyCommands(false); got.status != recFixed {
		t.Fatalf("deactivate status = %v: %s", got.status, got.detail)
	}
	if _, err := os.Lstat(filepath.Join(home, ".local", "bin", "omarchy")); !os.IsNotExist(err) {
		t.Fatalf("omarchy remained on PATH after deactivation: %v", err)
	}
	if got := reconcileNomarchyCommands(false); got.status != recOK {
		t.Fatalf("second deactivate status = %v: %s", got.status, got.detail)
	}
}
