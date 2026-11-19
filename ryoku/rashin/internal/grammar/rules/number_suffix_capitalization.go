package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"unicode"
)

// NumberSuffixCapitalization detects incorrect capitalization for number suffixes (e.g. "2ND").
type NumberSuffixCapitalization struct{}

func NewNumberSuffixCapitalization() *NumberSuffixCapitalization {
	return &NumberSuffixCapitalization{}
}
func (*NumberSuffixCapitalization) Description() string {
	return "You should never capitalize number suffixes."
}
func (*NumberSuffixCapitalization) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, tok := range doc.IterNumbers(d.Tokens()) {
		if tok.Kind.Number.Suffix == nil {
			continue
		}
		span := text.NewSpanWithLen(tok.Span.End, 2)
		span, _ = span.PulledBy(2)
		chars := span.GetContent(d.GetSource())
		bad := false
		for _, c := range chars {
			if !unicode.IsLower(c) {
				bad = true
				break
			}
		}
		if bad {
			replacement := append([]rune(nil), chars...)
			for i, c := range replacement {
				if c >= 'A' && c <= 'Z' {
					replacement[i] = c + 32
				}
			}
			l := lint.NewLint()
			l.Span = span
			l.LintKind = lint.Capitalization
			l.Message = "This suffix should be lowercase"
			l.Suggestions = []lint.Suggestion{lint.ReplaceWith(replacement)}
			out = append(out, l)
		}
	}
	return out
}
