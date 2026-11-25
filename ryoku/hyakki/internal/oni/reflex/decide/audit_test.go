package decide_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"ryoku-rashin/internal/oni/reflex/decide"
	"ryoku-rashin/internal/oni/voice/route"
)

// thirdParty names products and models no user-facing string may mention.
var thirdParty = regexp.MustCompile(`(?i)\b(open ?ai|chat ?gpt|gpt[- ]?\d|claude|anthropic|gemini|gemma|google|llama|mistral|mixtral|qwen|deep ?seek|ollama|phi-\d|microsoft|copilot|meta ai|hugging ?face|cohere|grok|perplexity|nvidia|jev)\b`)

// marks are the reference product marks; built in pieces so this file holds none.
var marks = []*regexp.Regexp{
	regexp.MustCompile("(?i)n\u016b" + "s"),
	regexp.MustCompile(`\bK` + `nots?\b`),
	regexp.MustCompile("(?i)ni" + "xi"),
	regexp.MustCompile("(?i)cou" + "cou"),
}

func TestCopyNamesNoThirdPartyProductOrModel(t *testing.T) {
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	strs := 0
	for _, name := range sources {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		text := norm.NFKC.String(string(raw))
		for _, mark := range marks {
			if mark.MatchString(text) {
				t.Errorf("%s carries a reference product mark", name)
			}
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, raw, 0)
		if err != nil {
			t.Fatal(err)
		}
		imports := map[*ast.BasicLit]bool{}
		for _, spec := range file.Imports {
			imports[spec.Path] = true
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING || imports[lit] {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("%s: %v", fset.Position(lit.Pos()), err)
			}
			strs++
			if thirdParty.MatchString(value) {
				t.Errorf("%s: %q names a third-party product or model", fset.Position(lit.Pos()), value)
			}
			return true
		})
	}
	if strs == 0 {
		t.Fatal("the audit read no strings")
	}
	if !route.Fits(decide.CreditLine) || thirdParty.MatchString(decide.CreditLine) {
		t.Fatalf("the plain credit line %q breaks the voice discipline or names a product", decide.CreditLine)
	}
	for _, r := range decide.CreditLine {
		if r > unicode.MaxASCII {
			t.Fatalf("the plain credit line %q is not plain", decide.CreditLine)
		}
	}
	if !thirdParty.MatchString("routed to Claude") || thirdParty.MatchString("Ryoku Ember") {
		t.Fatal("the audit pattern does not discriminate")
	}
}
