package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/paths"
)

// initFixture points the init probes at a fresh root with the given files.
func initFixture(t *testing.T, files map[string]string, dirs ...string) {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := initRoot
	initRoot = root
	t.Cleanup(func() { initRoot = old })
}

// noRyokuHost hides ryoku-host so the standalone steps print.
func noRyokuHost(t *testing.T) {
	t.Helper()
	old := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { lookPath = old })
}

// serviceHome gives the steps an empty home with no service folder.
func serviceHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("SVDIR", "")
	return home
}

func TestRunitIsDetectedOnlyWithoutSystemd(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		dirs  []string
		want  bool
	}{
		{"runit as process 1", map[string]string{"proc/1/comm": "runit\n"}, nil, true},
		{"runit state directory", nil, []string{"run/runit"}, true},
		{"systemd wins", map[string]string{"proc/1/comm": "runit\n"}, []string{"run/systemd/system", "run/runit"}, false},
		{"another init", map[string]string{"proc/1/comm": "openrc-init\n"}, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			initFixture(t, c.files, c.dirs...)
			if got := runitPresent(); got != c.want {
				t.Fatalf("runitPresent() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestOnConditionalConsentAndExplicitOverride(t *testing.T) {
	cases := []struct {
		name, document string
		args           []string
		code           int
		enabled        bool
	}{
		{"pending respects off", `{"version":2,"enabled":false}`, []string{"--if-unset"}, 3, false},
		{"explicit on overrides off", `{"version":2,"enabled":false}`, nil, 0, true},
		{"invalid consent is an error, not a skip", `{"version":2,`, []string{"--if-unset"}, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initFixture(t, nil)
			home := serviceHome(t)
			for _, key := range []string{"XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_RUNTIME_DIR"} {
				t.Setenv(key, filepath.Join(home, key))
			}
			t.Setenv("PATH", t.TempDir())
			path := paths.ConfigFile()
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(tc.document), 0o600); err != nil {
				t.Fatal(err)
			}
			if code := runOn(tc.args); code != tc.code {
				t.Fatalf("on %v = %d, want %d", tc.args, code, tc.code)
			}
			if tc.code == 0 {
				settings, err := config.Load()
				if err != nil || settings.Enabled != tc.enabled {
					t.Fatalf("explicit activation = %v, %v", settings.Enabled, err)
				}
			} else {
				after, err := os.ReadFile(path)
				if err != nil || string(after) != tc.document {
					t.Fatalf("conditional activation changed consent: %q, %v", after, err)
				}
			}
		})
	}
}

func TestRunitOnPrintsExactStepsForTheFoundServiceFolderAndWritesNothing(t *testing.T) {
	noRyokuHost(t)
	home := serviceHome(t)
	dir := filepath.Join(home, ".config", "service")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	runitSteps(&out, true)
	want := "  cp -R /usr/share/hyakki/runit/oni-routines /usr/share/hyakki/runit/hyakki-usage " + dir + "/\n" +
		"  sv up " + dir + "/oni-routines " + dir + "/hyakki-usage\n"
	if !strings.HasSuffix(out.String(), want) {
		t.Fatalf("steps:\n%s\nwant suffix:\n%s", out.String(), want)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("service folder changed: %v %v", entries, err)
	}
}

func TestRunitOnSkipsTheCopyForInstalledServicesAndHonoursSVDIR(t *testing.T) {
	noRyokuHost(t)
	serviceHome(t)
	dir := filepath.Join(t.TempDir(), "my services")
	for _, name := range []string{"oni-routines", "hyakki-usage"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, "run"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("SVDIR", dir)
	var out strings.Builder
	runitSteps(&out, true)
	if strings.Contains(out.String(), "cp -R") {
		t.Fatalf("copy offered for installed services:\n%s", out.String())
	}
	quoted := "'" + dir + "'"
	if !strings.Contains(out.String(), "  sv up "+quoted+"/oni-routines "+quoted+"/hyakki-usage\n") {
		t.Fatalf("steps do not quote the folder:\n%s", out.String())
	}
}

func TestRunitStepsNameSVDIRWhenNoServiceFolderExists(t *testing.T) {
	noRyokuHost(t)
	serviceHome(t)
	var out strings.Builder
	runitSteps(&out, false)
	got := out.String()
	for _, line := range []string{
		"SVDIR",
		`  sv down "$SVDIR"/oni "$SVDIR"/oni-routines "$SVDIR"/hyakki-usage` + "\n",
		`  rm -r "$SVDIR"/oni "$SVDIR"/oni-routines "$SVDIR"/hyakki-usage` + "\n",
	} {
		if !strings.Contains(got, line) {
			t.Fatalf("off steps missing %q:\n%s", line, got)
		}
	}
}

func TestRunitOffNamesOnlyInstalledServices(t *testing.T) {
	noRyokuHost(t)
	home := serviceHome(t)
	dir := filepath.Join(home, "service")
	if err := os.MkdirAll(filepath.Join(dir, "hyakki-usage"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hyakki-usage", "run"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	runitSteps(&out, false)
	got := out.String()
	if !strings.Contains(got, "  sv down "+dir+"/hyakki-usage\n") || strings.Contains(got, "/oni ") || strings.Contains(got, "oni-routines") {
		t.Fatalf("off steps:\n%s", got)
	}
	if err := os.RemoveAll(filepath.Join(dir, "hyakki-usage")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	runitSteps(&out, false)
	if strings.Contains(out.String(), "sv down") {
		t.Fatalf("nothing installed, but steps printed:\n%s", out.String())
	}
}

func TestRunitOnRyokuPointsAtRyokuHost(t *testing.T) {
	serviceHome(t)
	old := lookPath
	lookPath = func(name string) (string, error) {
		if name == "ryoku-host" {
			return "/usr/bin/ryoku-host", nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = old })
	var on, off strings.Builder
	runitSteps(&on, true)
	runitSteps(&off, false)
	if !strings.HasSuffix(on.String(), "  ryoku-host svc --user enable --now hyakki-usage\n") || strings.Contains(on.String(), " sv ") {
		t.Fatalf("on steps on Ryoku:\n%s", on.String())
	}
	if !strings.HasSuffix(off.String(), "  ryoku-host svc --user disable --now hyakki-usage\n  ryoku-host svc --user disable --now oni-routines\n") {
		t.Fatalf("off steps on Ryoku:\n%s", off.String())
	}
}
