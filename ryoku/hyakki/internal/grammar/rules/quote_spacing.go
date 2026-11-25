package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type QuoteSpacing struct{ expression expr.Expr }

func NewQuoteSpacing() *QuoteSpacing {
	return &QuoteSpacing{expression: expr.NewSequenceExprAnyWord().ThenQuote().ThenAnyWord()}
}
func (*QuoteSpacing) Description() string {
	return "Checks that quotation marks are preceded or succeeded by whitespace."
}
func (r *QuoteSpacing) Lint(d *doc.Document) []lint.Lint {
	var lints []lint.Lint
	for _, m := range expr.IterMatchesInDoc(r.expression, d) {
		t := d.Tokens()[m.Start:m.End]
		if _, ok := doc.Span(t); !ok {
			continue
		}
		q, ok := t[1].Kind.AsQuote()
		isClosing := ok && q.TwinLoc != nil && *q.TwinLoc < m.Start+1
		suggestions := []lint.Suggestion{lint.ReplaceWith([]rune{' ', '"'}), lint.ReplaceWith([]rune{'"', ' '})}
		if isClosing {
			suggestions[0], suggestions[1] = suggestions[1], suggestions[0]
		}
		lints = append(lints, lint.Lint{Span: t[1].Span, LintKind: lint.Formatting, Suggestions: suggestions, Message: "A quote must be preceded or succeeded by a space.", Priority: 31})
	}
	return lints
}
