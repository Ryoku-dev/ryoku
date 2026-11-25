package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
)

type UnclosedQuotes struct{}

func NewUnclosedQuotes() *UnclosedQuotes { return &UnclosedQuotes{} }
func (*UnclosedQuotes) Description() string {
	return "Quotation marks should always be closed. Unpaired quotation marks are a hallmark of sloppy work."
}
func (*UnclosedQuotes) Lint(d *doc.Document) []lint.Lint {
	var result []lint.Lint
	for _, tok := range d.Tokens() {
		if q, ok := tok.Kind.AsQuote(); ok && q.TwinLoc == nil {
			result = append(result, lint.Lint{Span: tok.Span, LintKind: lint.Formatting, Message: "This quote has no termination.", Priority: 255})
		}
	}
	return result
}
