package feature

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// A character changes presentation, never permission: policy must not be able
// to read a profile, the profile loader must not reach the code that acts,
// presence decides what Oni shows without reading a character, and nothing
// policy links may drive input. The window reader, the capture, the
// screen-ask parser, the text reader, the keep store, the settle watcher and
// the guide machine stay read-only: none of them reaches the team screens'
// typing and pointer code, a character, the daemon or the model gateway.
// Dictation typing is text entry, not Hands: no Oni package may reach the
// insert package, and the dictation target and insert code reach neither
// the Oni frames and presence nor the team screens. The reflex executors
// live outside internal/oni and no Oni package reaches them: the reflex
// rules, the decision policy, the reminder and routine stores only plan
// and keep, and reach no character, presence, screen, daemon or gateway.
// Hands is split the same way: its rules under internal/oni/hands are pure
// and reach no character, presence, screen, daemon, gateway or executor,
// no Oni package reaches the executors under internal/hands, and those
// reach only the pure rules, never presence, a character, the daemon, the
// gateway, dictation's typing, the reflex executors or the team screens.
// The focus guard's feed matcher is pure the same way, and its window
// closer, outside internal/oni, reaches no Oni feature, presence,
// character, screen, daemon, gateway, typing or Hands executor. A from
// ending in "/..." covers every package under it.
var boundaries = []struct {
	from      string
	forbidden []string
}{
	{"internal/oni/feature", []string{"internal/oni/profile", "internal/oni/characters", "internal/screen", "internal/daemon"}},
	{"internal/oni/profile", []string{"internal/oni/feature", "internal/daemon", "internal/gateway"}},
	{"internal/oni/presence", []string{"internal/oni/profile", "internal/oni/characters", "internal/screen", "internal/daemon"}},
	{"internal/oni/window/...", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/dictate/insert", "internal/reflex", "internal/hands"}},
	{"internal/oni/capture", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon"}},
	{"internal/oni/screenask", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway"}},
	{"internal/oni/ocr", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway"}},
	{"internal/oni/keep", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway"}},
	{"internal/oni/settle", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway"}},
	{"internal/oni/guide", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway"}},
	{"internal/oni/reflex/...", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway"}},
	{"internal/oni/remind", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway"}},
	{"internal/oni/routine", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway"}},
	{"internal/oni/hands/...", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway", "internal/hands"}},
	{"internal/oni/guard", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway"}},
	{"internal/oni/note", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway", "internal/memory"}},
	{"internal/oni/recall", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway", "internal/memory", "internal/index"}},
	{"internal/oni/assign", []string{"internal/screen", "internal/oni/profile", "internal/oni/characters", "internal/oni/presence", "internal/daemon", "internal/gateway", "internal/oni/capture"}},
	{"internal/imports/...", []string{"internal/oni/feature", "internal/oni/presence", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway", "internal/screen", "internal/dictate/insert", "internal/hands", "internal/reflex", "internal/index", "internal/memory"}},
	{"internal/oni/...", []string{"internal/dictate/insert", "internal/reflex", "internal/hands"}},
	{"internal/hands/...", []string{"internal/oni/presence", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway", "internal/dictate/insert", "internal/reflex", "internal/screen"}},
	{"internal/dictate/target", []string{"internal/dictate/insert", "internal/oni/feature", "internal/oni/presence", "internal/screen", "internal/daemon"}},
	{"internal/dictate/insert", []string{"internal/oni/feature", "internal/oni/presence", "internal/screen", "internal/daemon"}},
	{"internal/feedclose", []string{"internal/oni/feature", "internal/oni/presence", "internal/oni/profile", "internal/oni/characters", "internal/daemon", "internal/gateway", "internal/screen", "internal/dictate/insert", "internal/hands"}},
}

func TestPolicyAndProfilesStayApart(t *testing.T) {
	root, module := moduleRoot(t)
	for _, b := range boundaries {
		for _, from := range expand(t, root, b.from) {
			start := module + "/" + from
			bad, reached, err := violations(root, module, start, b.forbidden)
			if err != nil {
				t.Fatal(err)
			}
			for _, chain := range bad {
				t.Errorf("forbidden import: %s", chain)
			}
			if from == "internal/oni/feature" && !reached[module+"/internal/protocol"] {
				t.Errorf("the walk from %s never reached internal/protocol, so it followed no imports", from)
			}
		}
	}
}

// expand turns a from ending in "/..." into every directory under it that
// holds non-test Go files, and any other from into itself.
func expand(t *testing.T, root, from string) []string {
	t.Helper()
	base, ok := strings.CutSuffix(from, "/...")
	if !ok {
		return []string{from}
	}
	walkRoot := filepath.Join(root, filepath.FromSlash(base))
	if gomod, err := os.ReadFile(filepath.Join(root, "go.mod")); err == nil {
		var resolveErr error
		walkRoot, resolveErr = packageSourceDir(root, modulePath(gomod), modulePath(gomod)+"/"+base)
		if resolveErr != nil {
			t.Fatal(resolveErr)
		}
	}
	var out []string
	err := filepath.WalkDir(walkRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(walkRoot, filepath.Dir(path))
		if err != nil {
			return err
		}
		if pkg := filepath.ToSlash(filepath.Join(base, rel)); !slices.Contains(out, pkg) {
			out = append(out, pkg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatalf("%s matches no package", from)
	}
	return out
}

func TestBoundaryCheckerCatchesAnIndirectImport(t *testing.T) {
	root := t.TempDir()
	const module = "example.test/app"
	files := map[string]string{
		"go.mod":                             "module " + module + "\n\ngo 1.26\n\nreplace " + module + "/internal/relay => ./providers/relay\n",
		"internal/oni/feature/a.go":          "package feature\n\nimport (\n\t\"fmt\"\n\t_ \"" + module + "/internal/relay\"\n)\n\nvar _ = fmt.Sprint\n",
		"internal/oni/feature/a_test.go":     "package feature\n\nimport _ \"" + module + "/internal/oni/characters\"\n",
		"internal/oni/presence/s.go":         "package presence\n\nimport _ \"" + module + "/internal/oni/feature\"\n",
		"providers/relay/r.go":               "package relay\n\nimport _ \"" + module + "/internal/oni/profile/load\"\n",
		"internal/oni/profile/load/l.go":     "package load\n",
		"internal/oni/profile/p.go":          "package profile\n\nimport _ \"" + module + "/internal/daemon\"\n",
		"internal/oni/characters/c.go":       "package characters\n",
		"internal/daemon/d.go":               "package daemon\n",
		"internal/oni/profile/p_unused.json": "{}\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if got := modulePath(gomod); got != module {
		t.Fatalf("module = %q", got)
	}
	want := map[string]string{
		"internal/oni/feature":  module + "/internal/oni/feature -> " + module + "/internal/relay -> " + module + "/internal/oni/profile/load",
		"internal/oni/profile":  module + "/internal/oni/profile -> " + module + "/internal/daemon",
		"internal/oni/presence": module + "/internal/oni/presence -> " + module + "/internal/oni/feature -> " + module + "/internal/relay -> " + module + "/internal/oni/profile/load",
	}
	for _, b := range boundaries {
		if _, ok := want[b.from]; !ok {
			continue
		}
		bad, _, err := violations(root, module, module+"/"+b.from, b.forbidden)
		if err != nil {
			t.Fatal(err)
		}
		if len(bad) != 1 || bad[0] != want[b.from] {
			t.Errorf("violations from %s = %q, want [%q]", b.from, bad, want[b.from])
		}
	}

	clean := map[string]string{
		"providers/relay/r.go":      "package relay\n",
		"internal/oni/profile/p.go": "package profile\n\nimport _ \"" + module + "/internal/oni/profile/load\"\n",
	}
	for name, body := range clean {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, b := range boundaries {
		if _, ok := want[b.from]; !ok {
			continue
		}
		bad, _, err := violations(root, module, module+"/"+b.from, b.forbidden)
		if err != nil {
			t.Fatal(err)
		}
		if len(bad) != 0 {
			t.Errorf("a clean tree reported %q from %s", bad, b.from)
		}
	}
}

func TestEveryOniPackageIsKeptFromDictationTyping(t *testing.T) {
	root := t.TempDir()
	const module = "example.test/app"
	files := map[string]string{
		"internal/oni/feature/f.go":           "package feature\n",
		"internal/oni/voice/route/r.go":       "package route\n\nimport _ \"" + module + "/internal/relay\"\n",
		"internal/oni/voice/route/r_test.go":  "package route\n\nimport _ \"" + module + "/internal/dictate/insert\"\n",
		"internal/oni/voice/testdata/x.go":    "package x\n\nimport _ \"" + module + "/internal/dictate/insert\"\n",
		"internal/relay/r.go":                 "package relay\n\nimport _ \"" + module + "/internal/dictate/insert\"\n",
		"internal/dictate/insert/insert.go":   "package insert\n",
		"internal/oni/voice/clips/c.go":       "package clips\n",
		"internal/oni/voice/clips/c_test.go":  "package clips\n",
		"internal/oni/voice/empty/e_test.go":  "package empty\n",
		"internal/oni/voice/empty/notes.json": "{}\n",
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	i := slices.IndexFunc(boundaries, func(b struct {
		from      string
		forbidden []string
	}) bool {
		return b.from == "internal/oni/..."
	})
	if i < 0 {
		t.Fatal("no boundary keeps the Oni packages from the insert package")
	}
	froms := expand(t, root, boundaries[i].from)
	slices.Sort(froms)
	if want := []string{"internal/oni/feature", "internal/oni/voice/clips", "internal/oni/voice/route"}; !slices.Equal(froms, want) {
		t.Fatalf("expanded to %q, want %q", froms, want)
	}
	var bad []string
	for _, from := range froms {
		got, _, err := violations(root, module, module+"/"+from, boundaries[i].forbidden)
		if err != nil {
			t.Fatal(err)
		}
		bad = append(bad, got...)
	}
	want := module + "/internal/oni/voice/route -> " + module + "/internal/relay -> " + module + "/internal/dictate/insert"
	if len(bad) != 1 || bad[0] != want {
		t.Fatalf("violations %q, want [%q]", bad, want)
	}
}

func moduleRoot(t *testing.T) (string, string) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			module := modulePath(b)
			if module == "" {
				t.Fatalf("%s names no module", filepath.Join(dir, "go.mod"))
			}
			return dir, module
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod above the package")
		}
		dir = parent
	}
}

func modulePath(gomod []byte) string {
	for line := range strings.SplitSeq(string(gomod), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

// violations walks the in-module imports of the non-test files reachable
// from start and returns, for each forbidden package reached, the chain of
// imports that reaches it.
func violations(root, module, start string, forbidden []string) ([]string, map[string]bool, error) {
	parent := map[string]string{start: ""}
	queue := []string{start}
	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		imports, err := packageImports(root, module, pkg)
		if err != nil {
			return nil, nil, err
		}
		for _, imp := range imports {
			if _, seen := parent[imp]; !seen {
				parent[imp] = pkg
				queue = append(queue, imp)
			}
		}
	}
	reached := map[string]bool{}
	var bad []string
	for pkg := range parent {
		reached[pkg] = true
		for _, f := range forbidden {
			full := module + "/" + f
			if pkg == full || strings.HasPrefix(pkg, full+"/") {
				var chain []string
				for p := pkg; p != ""; p = parent[p] {
					chain = append(chain, p)
				}
				slices.Reverse(chain)
				bad = append(bad, strings.Join(chain, " -> "))
			}
		}
	}
	slices.Sort(bad)
	return bad, reached, nil
}

func packageSourceDir(root, module, pkg string) (string, error) {
	base, prefix := root, module
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	block := false
	for line := range strings.SplitSeq(string(gomod), "\n") {
		line = strings.TrimSpace(strings.SplitN(line, "//", 2)[0])
		if line == "replace (" {
			block = true
			continue
		}
		if line == ")" {
			block = false
			continue
		}
		if rest, ok := strings.CutPrefix(line, "replace "); ok {
			line = rest
		} else if !block {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[1] != "=>" {
			continue
		}
		name, target := strings.Trim(fields[0], `"`), strings.Trim(fields[2], `"`)
		if len(name) <= len(prefix) || (pkg != name && !strings.HasPrefix(pkg, name+"/")) {
			continue
		}
		if !strings.HasPrefix(target, ".") && !filepath.IsAbs(target) {
			continue
		}
		prefix = name
		base = target
		if !filepath.IsAbs(base) {
			base = filepath.Join(root, base)
		}
	}
	return filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(strings.TrimPrefix(pkg, prefix), "/"))), nil
}

func packageImports(root, module, pkg string) ([]string, error) {
	dir, err := packageSourceDir(root, module, pkg)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var out []string
	files := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		files++
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			if (path == module || strings.HasPrefix(path, module+"/")) && !slices.Contains(out, path) {
				out = append(out, path)
			}
		}
	}
	if files == 0 {
		return nil, fmt.Errorf("package %s has no Go files", pkg)
	}
	return out, nil
}
