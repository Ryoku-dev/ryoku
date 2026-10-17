package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestListIconThemes: the icon picker offers a theme (index.theme plus an icon
// directory) and never a cursor-only set, which carries an index.theme too.
func TestListIconThemes(t *testing.T) {
	dataHome := filepath.Join(t.TempDir(), "xdg")
	icons := filepath.Join(dataHome, "icons")

	// a real theme: index.theme + scalable icons
	mkTheme(t, filepath.Join(icons, "TestFull"), "[Icon Theme]\nName=TestFull\n")
	if err := os.MkdirAll(filepath.Join(icons, "TestFull", "scalable", "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a raster-only theme (no scalable dir) still counts
	mkTheme(t, filepath.Join(icons, "TestRaster"), "[Icon Theme]\nName=TestRaster\n")
	if err := os.MkdirAll(filepath.Join(icons, "TestRaster", "48x48", "apps"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a cursor set: index.theme + cursors/, no icon dirs
	mkTheme(t, filepath.Join(icons, "TestCursorOnly"), "[Icon Theme]\nName=TestCursorOnly\n")
	if err := os.MkdirAll(filepath.Join(icons, "TestCursorOnly", "cursors"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a bare directory with no index at all
	if err := os.MkdirAll(filepath.Join(icons, "TestBare"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("HOME", filepath.Dir(dataHome))
	t.Setenv("XDG_DATA_HOME", dataHome)

	got := listIconThemes()
	for _, want := range []string{"TestFull", "TestRaster"} {
		if !slices.Contains(got, want) {
			t.Errorf("listIconThemes() = %v, missing %q", got, want)
		}
	}
	for _, drop := range []string{"TestCursorOnly", "TestBare", "hicolor"} {
		if slices.Contains(got, drop) {
			t.Errorf("listIconThemes() = %v, should not offer %q", got, drop)
		}
	}
}

func TestSizedIconDir(t *testing.T) {
	for _, in := range []string{"48x48", "16x16@2x", "512x512"} {
		if !sizedIconDir(in) {
			t.Errorf("sizedIconDir(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"cursors", "scalable", "48x", "axb", "16X16"} {
		if sizedIconDir(in) {
			t.Errorf("sizedIconDir(%q) = true, want false", in)
		}
	}
}

func mkTheme(t *testing.T, dir, index string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.theme"), []byte(index), 0o644); err != nil {
		t.Fatal(err)
	}
}
