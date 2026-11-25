package reflex

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The package plans only: it may import these and nothing else, so it can
// open no file, start no process, open no socket and read no clock.
var allowedImports = map[string]bool{
	"cmp": true, "regexp": true, "slices": true, "strings": true, "sync": true,
	"time": true, "unicode": true, "unicode/utf8": true,
	"ryoku-i18n":                        true,
	"ryoku-rashin/internal/oni/feature": true,
	"ryoku-rashin/internal/oni/spec":    true,
}

// Clock reads and waits in package time.
var clockCalls = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true, "After": true, "AfterFunc": true,
	"Tick": true, "NewTimer": true, "NewTicker": true, "LoadLocation": true,
}

func TestPackageIsPure(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	seen := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		seen++
		file, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		timeName := ""
		for _, im := range file.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if !allowedImports[p] {
				t.Errorf("%s imports %s", f, p)
			}
			if p == "time" {
				timeName = "time"
				if im.Name != nil {
					timeName = im.Name.Name
				}
			}
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == timeName && clockCalls[n.Sel.Name] {
					t.Errorf("%s: calls time.%s", fset.Position(n.Pos()), n.Sel.Name)
				}
			case *ast.GoStmt:
				t.Errorf("%s: starts a goroutine", fset.Position(n.Pos()))
			}
			return true
		})
	}
	if seen == 0 {
		t.Fatal("no source files found")
	}
}
