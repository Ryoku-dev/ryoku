package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	oldHyprland = "0aa6745f705f2c4b0a5004eb019ef76e5bbdb426"
	newHyprland = "acc1aa0bfd4bd575dcb0f76c35ccf12b660a1ab9"
)

// fakeHeaders points pkg-config at a version.h for the given Hyprland build.
func fakeHeaders(t *testing.T, root, commit, aquamarine string) {
	t.Helper()
	inc := filepath.Join(root, "include")
	if err := os.MkdirAll(filepath.Join(inc, "hyprland", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	h := "#define GIT_COMMIT_HASH \"" + commit + "\"\n#define GIT_TAG \"v0.57.1\"\n" +
		"#define AQUAMARINE_VERSION \"" + aquamarine + "\"\n#define HYPRUTILS_VERSION \"0.14.3\"\n" +
		"#define HYPRGRAPHICS_VERSION \"0.6.0\"\n#define HYPRCURSOR_VERSION \"0.1.13\"\n#define HYPRLANG_VERSION \"0.6.8\"\n"
	if err := os.WriteFile(filepath.Join(inc, "hyprland", "src", "version.h"), []byte(h), 0o644); err != nil {
		t.Fatal(err)
	}
	pc := "#!/bin/sh\necho " + inc + "\n"
	if err := os.WriteFile(filepath.Join(root, "bin", "pkg-config"), []byte(pc), 0o755); err != nil {
		t.Fatal(err)
	}
}

// fakePluginRepo is a plugin with dynamic-cursors' Makefile shape: objects in
// out/ that depend on their .cpp only, never on the Hyprland headers.
func fakePluginRepo(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"hyprpm.toml": "[repository]\nname = \"fake\"\n\n[fake]\ndescription = \"fake\"\noutput = \"out/fake.so\"\nbuild = [\"make all\"]\n",
		"Makefile": "HDR := $(shell pkg-config --variable=prefix hyprland)/hyprland/src/version.h\n" +
			"OBJ := $(patsubst src/%.cpp,out/%.o,$(wildcard src/*.cpp))\n" +
			"all: out/fake.so\n" +
			"out/%.o: src/%.cpp\n\t@mkdir -p out\n\tgrep GIT_COMMIT_HASH $(HDR) > $@\n" +
			"out/fake.so: $(OBJ)\n\tcat $(OBJ) > $@\n",
		".gitignore":   "out/\n",
		"src/main.cpp": "int x;\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"add", "."},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "fake"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// A Hyprland or aquamarine bump must rebuild every object, not relink the ones
// the last build left behind and label the result with the new ABI.
func TestRebuildAfterHyprlandBumpCompilesFresh(t *testing.T) {
	for _, tool := range []string{"git", "make"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(root, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"))
	repo := filepath.Join(root, "upstream", "fake-cursor")
	fakePluginRepo(t, repo)
	def := pluginDef{ID: "fake-cursor", Name: "Fake cursor", Repo: repo, Plugin: "fake"}

	fakeHeaders(t, root, oldHyprland, "0.15.1")
	if res := buildPlugins([]buildJob{{def: def, reason: "requested"}}, "", io.Discard); len(res.Built) != 1 {
		t.Fatalf("first build: %+v", res)
	}

	fakeHeaders(t, root, newHyprland, "0.16.0")
	if res := buildPlugins([]buildJob{{def: def, reason: "requested"}}, "", io.Discard); len(res.Built) != 1 {
		t.Fatalf("rebuild: %+v", res)
	}
	so := filepath.Join(userPluginDir(), "fake-cursor.so")
	b, err := os.ReadFile(so)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), newHyprland) || strings.Contains(string(b), oldHyprland) {
		t.Fatalf("rebuilt .so still carries the old objects: %q", b)
	}
	if got := readABI(so); got.Commit != newHyprland || got.Aquamarine != "0.16" {
		t.Fatalf("sidecar = %q", got.String())
	}
}

// A copy whose sidecar claims the installed Hyprland while the .so itself names
// another one is stale, so `plugins rebuild --stale` and doctor rebuild it.
func TestNeedsBuildDistrustsSidecarOverBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	hdr := parseABI(newHyprland + "_aq_0.16_hu_0.14_hg_0.6_hc_0.1_hlg_0.6")
	def := pluginDef{ID: "fake-cursor", Repo: "https://example.invalid/fake"}
	so := filepath.Join(userPluginDir(), "fake-cursor.so")
	if err := os.MkdirAll(filepath.Dir(so), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abiSidecar(so), []byte(hdr.String()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		baked string
		need  bool
	}{{oldHyprland, true}, {newHyprland, false}, {"", false}} {
		if err := os.WriteFile(so, []byte("\x7fELF\x00"+tc.baked+"\x00_aq_\x00"), 0o755); err != nil {
			t.Fatal(err)
		}
		if need, why := needsBuild(def, hdr, ""); need != tc.need {
			t.Fatalf("baked %q: needsBuild = %v (%s), want %v", tc.baked, need, why, tc.need)
		}
	}
}
