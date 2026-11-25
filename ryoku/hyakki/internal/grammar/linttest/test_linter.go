package linttest

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"slices"
	"strings"
)

type TestLinterMapping struct{ Errors, Fixes []string }
type TestLinterMap []TestLinterMapping
type TestLinter struct{ mappings TestLinterMap }

func NewTestLinter(mappings TestLinterMap) *TestLinter { return &TestLinter{mappings: mappings} }
func (*TestLinter) Description() string                { return "Test linter for 'linting assertion' tests" }
func (l *TestLinter) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, tok := range d.GetTokens() {
		if !tok.Kind.IsWord() {
			continue
		}
		word := tok.GetCh(d.GetSource())
		for _, mapping := range l.mappings {
			if !slices.Contains(mapping.Errors, string(word)) {
				continue
			}
			entry := lint.NewLint()
			entry.Span = tok.Span
			entry.LintKind = lint.Spelling
			entry.Message = l.Description()
			entry.Suggestions = lint.ToReplaceSuggestions(mapping.Fixes, word)
			out = append(out, entry)
		}
	}
	return out
}
func SpansToStrings(spans []text.Span, d *doc.Document) []string {
	out := make([]string, 0, len(spans))
	for _, span := range spans {
		var s strings.Builder
		for _, tok := range d.GetTokens()[span.Start:span.End] {
			s.WriteString(tok.GetStr(d.GetSource()))
		}
		out = append(out, s.String())
	}
	return out
}
