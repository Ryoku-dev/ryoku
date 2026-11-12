package wm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// A provider's reasons (what it cannot honour, why a shortcut is off) and its
// plugin notes are shown as row text, so they follow the schema copy's rules:
// a sentence that starts with a capital, in plain British words. The import
// report is left out: it names nodes of the user's own config on purpose.
func TestProviderReasonCopyIsPlain(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("*", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fs := token.NewFileSet()
	checked := 0
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, "_test.go") || strings.HasPrefix(base, "import_") {
			continue
		}
		af, err := parser.ParseFile(fs, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// the literals a reason is built from, in order: the first starts the
		// sentence and the last ends it
		starts, ends := map[*ast.BasicLit]bool{}, map[*ast.BasicLit]bool{}
		// a provider's own shortcut names and hints, which Keyboard shortcuts shows
		binds := map[*ast.BasicLit]bool{}
		ast.Inspect(af, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			k, ok := kv.Key.(*ast.Ident)
			if !ok {
				return true
			}
			if b, lit := kv.Value.(*ast.BasicLit); lit && b.Kind == token.STRING && (k.Name == "label" || k.Name == "hint") {
				binds[b] = true
				return true
			}
			if k.Name != "Reason" && k.Name != "reason" {
				return true
			}
			var parts []*ast.BasicLit
			ast.Inspect(kv.Value, func(m ast.Node) bool {
				if b, ok := m.(*ast.BasicLit); ok && b.Kind == token.STRING {
					parts = append(parts, b)
					return false
				}
				return true
			})
			// a one-word tag is bookkeeping, not copy
			if len(parts) == 0 || (len(parts) == 1 && !strings.Contains(parts[0].Value, " ")) {
				return true
			}
			starts[parts[0]] = true
			ends[parts[len(parts)-1]] = true
			return true
		})
		ast.Inspect(af, func(n ast.Node) bool {
			b, ok := n.(*ast.BasicLit)
			if !ok || b.Kind != token.STRING {
				return true
			}
			s, err := strconv.Unquote(b.Value)
			if err != nil || s == "" {
				return true
			}
			sentence := strings.HasSuffix(s, ".") && strings.Contains(s, " ") && !strings.Contains(s, "\n") &&
				s[0] != ' ' && s[0] != '.'
			if !sentence && !starts[b] && !ends[b] && !binds[b] {
				return true
			}
			checked++
			at := fs.Position(b.Pos())
			if r := []rune(s)[0]; (sentence || starts[b] || binds[b]) && !unicode.IsUpper(r) {
				t.Errorf("%s: %q does not start with a capital", at, s)
			}
			if m := bindOnlyCopy.FindString(s); binds[b] && m != "" {
				t.Errorf("%s: shortcut copy %q shows %q", at, s, m)
			}
			if ends[b] && !strings.HasSuffix(s, ".") {
				t.Errorf("%s: reason %q does not end its sentence", at, s)
			}
			if m := rawCopy.FindString(s); m != "" {
				t.Errorf("%s: %q shows %q", at, s, m)
			}
			if m := usSpelling.FindString(s); m != "" {
				t.Errorf("%s: %q spells %q the US way", at, s, m)
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("found no provider reasons to check")
	}
}
