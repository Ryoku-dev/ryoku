package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

// EllipsisLength checks that an ellipsis contains neither too many nor too few periods.
type EllipsisLength struct{}

func NewEllipsisLength() *EllipsisLength { return &EllipsisLength{} }
func (*EllipsisLength) Description() string {
	return "Make sure you have the correct number of dots in your ellipsis."
}
func (*EllipsisLength) Lint(d *doc.Document) []lint.Lint {
	var results []lint.Lint
	for _, t := range d.Tokens() {
		if !t.Kind.IsEllipsis() {
			continue
		}
		ch := t.GetCh(d.GetSource())
		if len(ch) == 0 || ch[0] != '.' || len(ch) == 3 {
			continue
		}
		all := true
		for _, c := range ch {
			if c != '.' {
				all = false
				break
			}
		}
		if all {
			result := lint.NewLint()
			result.Span = t.Span
			result.LintKind = lint.Formatting
			result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("..."))}
			result.Message = "Horizontal ellipsis must have 3 dots."
			result.Priority = 31
			results = append(results, result)
		}
	}
	return results
}
