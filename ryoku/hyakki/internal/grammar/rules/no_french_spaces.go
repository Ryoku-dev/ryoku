package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type NoFrenchSpaces struct{}

func NewNoFrenchSpaces() *NoFrenchSpaces { return &NoFrenchSpaces{} }
func (*NoFrenchSpaces) Description() string {
	return "Stops users from accidentally inserting French spaces."
}
func (*NoFrenchSpaces) Lint(d *doc.Document) []lint.Lint {
	var output []lint.Lint
	for _, sentence := range doc.IterSentences(d.Tokens()) {
		for idx, space := range sentence {
			if !space.Kind.IsSpace() {
				continue
			}
			if space.Kind.Count != 0 && idx == 0 && space.Span.Len() != 1 {
				l := lint.NewLint()
				l.Span = space.Span
				l.LintKind = lint.Formatting
				l.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune{' '})}
				l.Message = "French spaces are generally not recommended."
				l.Priority = 15
				output = append(output, l)
			}
			break
		}
	}
	return output
}
