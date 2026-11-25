// Package hands holds the executors: cdp (the pipe protocol), chrome
// (starting the browser), page (driving the one authorized tab), input (the
// one pointer and keyboard driver) and nest (the separate nested desktop).
// This file pins what they may reach.
package hands

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const module = "ryoku-rashin/internal/"

// imports maps every .go file under internal/<dir> to its import paths.
func imports(t *testing.T, dir string) map[string][]string {
	t.Helper()
	root := filepath.Join("..", dir)
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		out[p] = []string{}
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

func under(p, pkg string) bool { return p == module+pkg || strings.HasPrefix(p, module+pkg+"/") }

// The executor reaches Oni only through the pure consent rules, and never
// the daemon, the dictation inserter, the reflex executors or the network
// layer. The live app driver alone also reads Oni's window facts, its
// capture and text reader, and the pure app rules and window discipline.
func TestHandsKeepsItsDistance(t *testing.T) {
	forbidden := []string{"daemon", "dictate/insert", "reflex", "gateway", "screen"}
	appOni := []string{"oni/window", "oni/capture", "oni/ocr", "oni/screenask", "oni/hands/discipline", "oni/hands/apprules"}
	for file, paths := range imports(t, "hands") {
		inApp := strings.Contains(filepath.ToSlash(file), "/hands/app/")
		for _, p := range paths {
			if under(p, "oni") && p != module+"oni/hands/consent" && !(inApp && slices.Contains(appOni, strings.TrimPrefix(p, module))) {
				t.Errorf("%s imports %s", file, p)
			}
			for _, f := range forbidden {
				if under(p, f) {
					t.Errorf("%s imports %s", file, p)
				}
			}
		}
	}
}

// Oni stays read-only: nothing under it reaches the executor.
func TestOniDoesNotImportHands(t *testing.T) {
	for file, paths := range imports(t, "oni") {
		for _, p := range paths {
			if under(p, "hands") {
				t.Errorf("%s imports %s", file, p)
			}
		}
	}
}

// inputMarks name an input tool or a virtual input device.
var inputMarks = []string{"wtype", "ydotool", "dotool", "xdotool", "uinput", "virtual_pointer", "virtual-pointer",
	"virtual_keyboard", "virtual-keyboard", "virtualkeyboard", "virtualpointer"}

func inInput(file string) bool { return strings.Contains(filepath.ToSlash(file), "/hands/input/") }

// Only input names an input tool or a virtual input device; the browser
// executor steps inside the one tab through the browser's own protocol.
// Processes start in chrome, nest and input alone, and only input's
// pointer dials a socket, a local Wayland one.
func TestHandsNamesNoInputExecutor(t *testing.T) {
	for file, paths := range imports(t, "hands") {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		slash := filepath.ToSlash(file)
		if !inInput(file) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			low := strings.ToLower(string(src))
			for _, m := range inputMarks {
				if strings.Contains(low, m) {
					t.Errorf("%s names %q", file, m)
				}
			}
		}
		for _, p := range paths {
			starts := strings.Contains(slash, "/hands/chrome/") || strings.Contains(slash, "/hands/nest/") || inInput(file)
			if (p == "os/exec" || p == "syscall") && !starts {
				t.Errorf("%s imports %s outside chrome, nest and input", file, p)
			}
			if p == "net" && !inInput(file) || p == "net/http" || strings.HasPrefix(p, "net/http/") {
				t.Errorf("%s imports %s: the executor has no network", file, p)
			}
		}
	}
}

// The agent-session screen and Oni drive input only through
// internal/hands/input: neither names an input tool, and the screen dials no
// socket of its own.
func TestScreenAndOniLeaveInputToInput(t *testing.T) {
	for _, dir := range []string{"screen", "oni"} {
		for file, paths := range imports(t, dir) {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			low := strings.ToLower(string(src))
			for _, m := range inputMarks {
				if strings.Contains(low, m) {
					t.Errorf("%s names %q", file, m)
				}
			}
			for _, p := range paths {
				if dir == "screen" && p == "net" {
					t.Errorf("%s imports net", file)
				}
			}
		}
	}
}

// Across the whole of internal, a string that names an input tool (the
// tool itself or a virtual input interface) lives only in input and in
// dictation's own inserter, so no other code can start one.
func TestOnlyInputAndDictationStartInputTools(t *testing.T) {
	tools := map[string]bool{"wtype": true, "ydotool": true, "dotool": true, "xdotool": true}
	interfaces := []string{"virtual_pointer", "virtual_keyboard", "uinput"}
	scanned, owners := 0, map[string]bool{}
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		slash := filepath.ToSlash(p)
		if inInput(p) || strings.Contains(slash, "/dictate/insert/") {
			owners[filepath.Base(filepath.Dir(p))] = true
			return nil
		}
		scanned++
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			low := strings.ToLower(value)
			if tools[strings.TrimSpace(low)] {
				t.Errorf("%s holds the tool name %q", p, value)
			}
			for _, m := range interfaces {
				if strings.Contains(low, m) {
					t.Errorf("%s holds %q", p, value)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned < 100 || !owners["input"] || !owners["insert"] {
		t.Fatalf("scanned %d files, owners %v: the walk missed internal", scanned, owners)
	}
}

// Only the protocol's pipe speaks to the page: no other package builds a
// command, and page script lives only in cdp's table.
func TestOnlyCdpHoldsPageScript(t *testing.T) {
	for file := range imports(t, "hands") {
		if strings.HasSuffix(file, "_test.go") || strings.Contains(filepath.ToSlash(file), "/hands/cdp/") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range []string{"functionDeclaration", "Runtime.evaluate", "function (", "function("} {
			if strings.Contains(string(src), m) {
				t.Errorf("%s holds page script (%q)", file, m)
			}
		}
	}
}
