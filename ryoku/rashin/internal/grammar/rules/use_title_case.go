package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type UseTitleCase struct{ dictionary dict.Dictionary }

func NewUseTitleCase(dictionary dict.Dictionary) *UseTitleCase { return &UseTitleCase{dictionary} }
func (*UseTitleCase) Description() string {
	return "Prompts you to use title case in relevant headings."
}
func (r *UseTitleCase) Lint(d *doc.Document) []lint.Lint {
	var lints []lint.Lint
	for _, heading := range d.IterHeadings() {
		span, ok := doc.Span(heading)
		if !ok {
			continue
		}
		if title, ok := doc.TryMakeTitleCase(heading, d.GetSource(), r.dictionary); ok {
			lints = append(lints, lint.Lint{Span: span, LintKind: lint.Capitalization, Suggestions: []lint.Suggestion{lint.ReplaceWith(title)}, Message: "Try to use title case in headings.", Priority: 127})
		}
	}
	return lints
}
