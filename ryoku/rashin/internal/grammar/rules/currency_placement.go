package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type CurrencyPlacement struct{}

func NewCurrencyPlacement() *CurrencyPlacement { return &CurrencyPlacement{} }
func (*CurrencyPlacement) Description() string {
	return "The location of currency symbols varies by country. The rule looks for and corrects improper positioning."
}
func (*CurrencyPlacement) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, chunk := range doc.IterChunks(d.Tokens()) {
		for i := 0; i+1 < len(chunk); i++ {
			if l := currencyPlacementGenerateLintForTokens(chunk[i], chunk[i+1], d); l != nil {
				out = append(out, *l)
			}
		}
		for i := 0; i+3 < len(chunk); i++ {
			p, a, b, c := chunk[i], chunk[i+1], chunk[i+2], chunk[i+3]
			if !b.Kind.IsWhitespace() || p.Kind.IsCurrency() {
				continue
			}
			if l := currencyPlacementGenerateLintForTokens(a, c, d); l != nil {
				out = append(out, *l)
			}
		}
	}
	return lint.RemoveOverlaps(out)
}
func currencyPlacementGenerateLintForTokens(a, b doc.Token, d *doc.Document) *lint.Lint {
	punct, ok := a.Kind.AsPunctuation()
	if !ok {
		punct, ok = b.Kind.AsPunctuation()
	}
	if !ok || !punct.IsCurrency() {
		return nil
	}
	number, ok := a.Kind.AsNumber()
	if !ok {
		number, ok = b.Kind.AsNumber()
	}
	if !ok {
		return nil
	}
	span := text.Span{Start: a.Span.Start, End: b.Span.End}
	correct := punct.Currency.FormatAmount(number)
	if correct == d.GetSpanContentStr(span) {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Formatting, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(correct))}, Message: "The position of the currency symbol matters.", Priority: 63}
}
