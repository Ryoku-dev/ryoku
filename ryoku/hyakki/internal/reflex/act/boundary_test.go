package act

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const module = "ryoku-rashin/internal/"

// imports maps every .go file under internal/<dir> to its import paths.
func imports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	root := filepath.Join("..", "..", dir)
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			out[p] = append(out[p], path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("no Go files under %s", root)
	}
	return out
}

// Oni's packages stay read-only: none of them may reach an executor.
func TestOniDoesNotImportReflexExecutors(t *testing.T) {
	for file, paths := range imports(t, "oni") {
		for _, p := range paths {
			if p == module+"reflex" || strings.HasPrefix(p, module+"reflex/") {
				t.Errorf("%s imports %s", file, p)
			}
		}
	}
}

// The executors stay apart from Oni's presence, screen and profile, from
// the daemon, and from the dictation inserter.
func TestReflexKeepsItsDistance(t *testing.T) {
	forbidden := []string{
		module + "oni/feature", module + "oni/presence", module + "oni/profile", module + "oni/screen",
		module + "daemon", module + "dictate/insert",
	}
	for file, paths := range imports(t, "reflex") {
		for _, p := range paths {
			for _, f := range forbidden {
				if p == f || strings.HasPrefix(p, f+"/") {
					t.Errorf("%s imports %s", file, p)
				}
			}
		}
	}
}

// No executor names an input tool or protocol: there are no key chords
// and nothing types into another window. Processes start in proc alone;
// launch imports os/exec only to read a start's exit status.
func TestReflexNamesNoInputExecutor(t *testing.T) {
	marks := []string{"wtype", "ydotool", "dotool", "xdotool", "uinput", "virtual_pointer", "virtual-pointer",
		"virtual_keyboard", "virtual-keyboard"}
	for file, paths := range imports(t, "reflex") {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		low := strings.ToLower(string(src))
		for _, m := range marks {
			if strings.Contains(low, m) {
				t.Errorf("%s names %q", file, m)
			}
		}
		for _, p := range paths {
			if p == "os/exec" && !strings.Contains(filepath.ToSlash(file), "/reflex/proc/") && !strings.Contains(filepath.ToSlash(file), "/reflex/launch/") {
				t.Errorf("%s starts processes outside proc", file)
			}
		}
	}
}
