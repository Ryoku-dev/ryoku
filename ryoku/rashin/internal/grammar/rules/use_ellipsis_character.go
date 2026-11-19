package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

// UseEllipsisCharacter converts three periods used as an ellipsis into the Unicode ellipsis character.
type UseEllipsisCharacter struct{}

func NewUseEllipsisCharacter() *UseEllipsisCharacter { return &UseEllipsisCharacter{} }
func (*UseEllipsisCharacter) Description() string {
	return "Replaces three-period ellipses with the single Unicode ellipsis character."
}
func (*UseEllipsisCharacter) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, tok := range doc.IterEllipsiss(d.Tokens()) {
		if tok.GetStr(d.GetSource()) != "..." {
			continue
		}
		l := lint.NewLint()
		l.Span = tok.Span
		l.LintKind = lint.Formatting
		l.Priority = 31
		l.Message = "Use the Unicode ellipsis character (…)."
		l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune{'…'})}
		out = append(out, l)
	}
	return out
}
