package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// FootInchMinuteSecondSymbols corrects typewriter-style symbols used in
// measurements to their proper Unicode equivalents, such as 6'7" → 6′7″
// and 48°51' → 48°51′.
//
// It corrects both ASCII and smart quotes/apostrophes when there's strong
// context they represent measurements, since these symbols have many other
// legitimate uses in English text (quotes, contractions, etc.).
type FootInchMinuteSecondSymbols struct{ expression expr.Expr }

func NewFootInchMinuteSecondSymbols() *FootInchMinuteSecondSymbols {
	return &FootInchMinuteSecondSymbols{expression: expr.NewSequenceExpr().ThenCardinalNumber().TOws().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsApostrophe, doc.TokenKind.IsOpenSingle, doc.TokenKind.IsSinglePrime}).TOws().ThenCardinalNumber().TOws().ThenKindEither(doc.TokenKind.IsQuote, doc.TokenKind.IsDoublePrime), expr.NewSequenceExpr().ThenDegree().TOws().ThenCardinalNumber().TOws().ThenKindEither(doc.TokenKind.IsApostrophe, doc.TokenKind.IsOpenSingle)})}
}
func (r *FootInchMinuteSecondSymbols) Expr() expr.Expr { return r.expression }
func (r *FootInchMinuteSecondSymbols) Description() string {
	return "Corrects the use of typewriter-style apostrophes and quotes for measurements to Unicode prime and double prime symbols."
}
func (r *FootInchMinuteSecondSymbols) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *FootInchMinuteSecondSymbols) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 4 {
		return nil
	}
	minor := len(t) - 1
	double := t[minor].Kind.IsQuote() || t[minor].Kind.IsDoublePrime()
	if !double && !footInchMinuteSecondSymbolsSingle(t[minor]) {
		return nil
	}
	matches := func(t doc.Token) bool {
		if double {
			return footInchMinuteSecondSymbolsSingle(t)
		}
		return t.Kind.IsDegree()
	}
	major := 2
	if matches(t[1]) {
		major = 1
	}
	if footInchMinuteSecondSymbolsCorrect(t[major]) && footInchMinuteSecondSymbolsCorrect(t[minor]) {
		return nil
	}
	span, ok := doc.Span(t[major:])
	if !ok {
		return nil
	}
	between, ok := doc.Span(t[major+1 : minor])
	if !ok {
		return nil
	}
	a, b := '°', '′'
	message := "For minutes, use the correct prime symbol (′) rather than an apostrophe or single quote."
	if double {
		a, b = '′', '″'
		message = "For feet and inches or minutes and seconds, use the correct prime and double prime symbols (′ and ″) rather than apostrophes or quotes."
	}
	replacement := make([]rune, 0, between.Len()+2)
	replacement = append(replacement, a)
	replacement = append(replacement, between.GetContent(s)...)
	replacement = append(replacement, b)
	return &lint.Lint{Span: span, LintKind: lint.Formatting, Suggestions: []lint.Suggestion{lint.ReplaceWith(replacement)}, Message: message, Priority: 127}
}

// footInchMinuteSecondSymbolsSingle checks whether a token is a single prime
// symbol or a character often used as one.
func footInchMinuteSecondSymbolsSingle(t doc.Token) bool {
	return t.Kind.IsApostrophe() || t.Kind.IsOpenSingle() || t.Kind.IsSinglePrime()
}

// footInchMinuteSecondSymbolsCorrect checks whether a token is already one of
// the correct Unicode degree or prime symbols.
func footInchMinuteSecondSymbolsCorrect(t doc.Token) bool {
	return t.Kind.IsDegree() || t.Kind.IsSinglePrime() || t.Kind.IsDoublePrime()
}
