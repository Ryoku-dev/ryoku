package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain points HOME and the XDG dirs at a throwaway tree, so no test can
// write into the developer's or CI runner's real config, cache, data, or trash.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "ryogami-test-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryogami tests: temp home:", err)
		os.Exit(1)
	}
	env := map[string]string{
		"HOME":            root,
		"XDG_CONFIG_HOME": filepath.Join(root, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(root, ".cache"),
		"XDG_DATA_HOME":   filepath.Join(root, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(root, ".local", "state"),
	}
	for k, v := range env {
		os.Setenv(k, v)
	}
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}

func TestImportLineResolvesRelativePathsInTheCaller(t *testing.T) {
	dir := t.TempDir()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
	cases := map[string][]string{
		"wallpaper import " + filepath.Join(dir, "my photo.jpg"):                {"wallpaper", "import", "my", "photo.jpg"},
		"wallpaper import --screen DP-1 " + filepath.Join(dir, "a.png"):         {"wallpaper", "import", "--screen", "DP-1", "a.png"},
		"wallpaper import /already/abs.png":                                     {"wallpaper", "import", "/already/abs.png"},
		"wallpaper set photo.jpg":                                               {"wallpaper", "set", "photo.jpg"},
		"wallpaper import --screen DP-1 " + filepath.Join(dir, "sub", "b.webp"): {"wallpaper", "import", "--screen", "DP-1", "sub/b.webp"},
	}
	for want, args := range cases {
		if got := clientLine(args); got != want {
			t.Errorf("clientLine(%q) = %q, want %q", args, got, want)
		}
	}
}
