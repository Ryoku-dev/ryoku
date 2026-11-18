package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type MissingSpace struct{}

func NewMissingSpace() *MissingSpace      { return &MissingSpace{} }
func (*MissingSpace) Description() string { return "Looks for missing spaces after a comma or period." }
func (*MissingSpace) Lint(d *doc.Document) []lint.Lint {
	toks := d.Tokens()
	var out []lint.Lint
	for i := 0; i+2 < len(toks); i++ {
		a, b, c := toks[i], toks[i+1], toks[i+2]
		p, punctuation := b.Kind.AsPunctuation()
		if a.Kind.IsWord() && c.Kind.IsWord() && punctuation && (p.IsPeriod() || p.IsBang() || p.IsQuestion() || p.IsSemicolon()) {
			l := lint.NewLint()
			l.Span = b.Span
			l.LintKind = lint.Formatting
			l.Priority = 31
			l.Message = "It looks like you're missing a space here."
			l.Suggestions = []lint.Suggestion{lint.InsertAfter([]rune{' '})}
			out = append(out, l)
		}
	}
	return out
}
