package weir

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
	"sync"
)

//go:embed weir_rules
var ruleFiles embed.FS

type NamedRule struct {
	Name   string
	Linter lint.Linter
}
type EmbeddedRule struct{ Name, PublicName, Path, Source string }

func EmbeddedRules() []EmbeddedRule {
	var out []EmbeddedRule
	err := fs.WalkDir(ruleFiles, "weir_rules", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".weir") {
			return nil
		}
		relative := strings.TrimPrefix(p, "weir_rules/")
		parts := strings.SplitN(relative, "/", 2)
		name := strings.TrimSuffix(relative, ".weir")
		public := name
		if len(parts) == 2 {
			public = parts[0]
			name = strings.TrimSuffix(parts[1], ".weir")
		}
		b, err := ruleFiles.ReadFile(p)
		if err != nil {
			return err
		}
		out = append(out, EmbeddedRule{name, public, relative, string(b)})
		return nil
	})
	if err != nil {
		panic(err)
	}
	return out
}

var compiledRules = sync.OnceValue(func() []NamedRule {
	var out []NamedRule
	groups := map[string]*lint.LintGroup{}
	for _, f := range EmbeddedRules() {
		r, err := NewWeirLinter(f.Source)
		if err != nil {
			panic(fmt.Errorf("%s: %w", f.Path, err))
		}
		if path.Dir(f.Path) == "." {
			out = append(out, NamedRule{f.PublicName, r})
		} else {
			g := groups[f.PublicName]
			if g == nil {
				g = lint.NewLintGroup()
				groups[f.PublicName] = g
				out = append(out, NamedRule{f.PublicName, g})
			}
			if r.scope == Sentence {
				g.AddSentenceExprLinter(f.Name, r)
			} else {
				g.AddChunkExprLinter(f.Name, r)
			}
			g.Config.SetRuleEnabled(f.Name, true)
		}
	}
	return out
})

// Rules returns the public Harper registrations: a top-level file is a rule,
// while each collection directory is one group containing its private rules.
func Rules() []NamedRule { return compiledRules() }
