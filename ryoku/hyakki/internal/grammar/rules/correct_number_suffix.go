package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// CorrectNumberSuffix detects incorrect number suffixes (e.g. "2st").
type CorrectNumberSuffix struct{}

func NewCorrectNumberSuffix() *CorrectNumberSuffix { return &CorrectNumberSuffix{} }
func (*CorrectNumberSuffix) Description() string {
	return "When making quick edits, it is common for authors to change the value of a number without changing its suffix. This rule looks for these cases, for example: `2st`."
}
func (*CorrectNumberSuffix) Lint(d *doc.Document) []lint.Lint {
	var output []lint.Lint
	for _, tok := range d.IterNumbers() {
		span, ok := text.NewSpanWithLen(tok.Span.End, 2).PulledBy(2)
		if !ok {
			continue
		}
		number, ok := tok.Kind.AsNumber()
		if !ok || number.Suffix == nil {
			continue
		}
		correct, ok := text.CorrectSuffixFor(number.Value)
		if !ok || *number.Suffix == correct {
			continue
		}
		result := lint.NewLint()
		result.Span = span
		result.LintKind = lint.Miscellaneous
		result.Message = "This number needs a different suffix to sound right."
		result.Suggestions = []lint.Suggestion{lint.ReplaceWith(correct.ToChars())}
		output = append(output, result)
	}
	return output
}
