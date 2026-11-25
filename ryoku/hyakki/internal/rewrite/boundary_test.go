package rewrite_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// goFiles parses every non-test Go file under dir.
func goFiles(t *testing.T, dir string) map[string]*ast.File {
	t.Helper()
	out := map[string]*ast.File{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(p)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("no Go files under %s", dir)
	}
	return out
}

func imports(f *ast.File) []string {
	var out []string
	for _, i := range f.Imports {
		p, _ := strconv.Unquote(i.Path.Value)
		out = append(out, p)
	}
	return out
}

// The rewrite executors stay out of Oni's presence, policy and character,
// the daemon, the gateway and quick commands; only the selection reader
// starts a process.
func TestRewriteStaysAnExecutor(t *testing.T) {
	banned := []string{"internal/oni/feature", "internal/oni/presence", "internal/oni/profile", "internal/oni/characters",
		"internal/daemon", "internal/gateway", "internal/reflex", "internal/oni/reflex", "internal/screen"}
	for file, f := range goFiles(t, ".") {
		for _, p := range imports(f) {
			for _, b := range banned {
				if strings.HasSuffix(p, b) || strings.Contains(p, b+"/") {
					t.Errorf("%s imports %s", file, p)
				}
			}
			if (p == "os/exec" || p == "syscall") && !strings.HasPrefix(file, "sel/") {
				t.Errorf("%s imports %s outside the selection reader", file, p)
			}
			if p == "net" || strings.HasPrefix(p, "net/") {
				t.Errorf("%s imports %s", file, p)
			}
		}
	}
}

// No code that serves Oni, dictation, Hands or writing names a clipboard
// tool, and only the selection reader names wl-paste.
func TestOnlyTheSelectionReaderNamesWlPaste(t *testing.T) {
	tools := []string{"wl-paste", "wl-copy", "xclip", "xsel", "cliphist", "copyq"}
	scanned := 0
	for _, dir := range []string{".", "../daemon", "../oni", "../dictate", "../hands", "../writing"} {
		for file, f := range goFiles(t, dir) {
			scanned++
			owner := strings.HasPrefix(file, "sel/")
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				v, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				low := strings.ToLower(v)
				for _, tool := range tools {
					if strings.Contains(low, tool) && !(owner && tool == "wl-paste") {
						t.Errorf("%s holds %q", file, v)
					}
				}
				return true
			})
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d files: the walk missed internal", scanned)
	}
}
