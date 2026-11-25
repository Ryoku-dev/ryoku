package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

// CapitalizePersonalPronouns makes sure you capitalize "I" and its contractions.
type CapitalizePersonalPronouns struct{}

func NewCapitalizePersonalPronouns() *CapitalizePersonalPronouns {
	return &CapitalizePersonalPronouns{}
}
func (*CapitalizePersonalPronouns) Description() string {
	return "Forgetting to capitalize personal pronouns, like \"I\" or \"I'm\" is one of the most common errors. This rule helps with that."
}
func (*CapitalizePersonalPronouns) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, t := range doc.IterWords(d.Tokens()) {
		chars := t.GetCh(d.GetSource())
		switch string(chars) {
		case "i", "i'd", "i'd've", "i'll", "i'm", "i've", "ive":
			replacement := append([]rune(nil), chars...)
			replacement[0] = 'I'
			result = append(result, lint.Lint{Span: t.Span, LintKind: lint.Capitalization, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Message: "The first-person singular subject pronoun must be capitalized.", Priority: 31})
		}
	}
	return result
}
