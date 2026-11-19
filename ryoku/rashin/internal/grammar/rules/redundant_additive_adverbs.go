package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type RedundantAdditiveAdverbs struct{ expression expr.Expr }

func NewRedundantAdditiveAdverbs() *RedundantAdditiveAdverbs {
	add := expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"also", "too"}), expr.NewFixedPhraseFromPhrase("as well")})
	return &RedundantAdditiveAdverbs{expression: expr.NewSequenceExprWith(add).ThenOneOrMore(expr.NewSequenceExprWhitespace().Then(add)).ThenOptional(expr.NewSequenceExprWhitespace().TAco("as"))}
}
func (r *RedundantAdditiveAdverbs) Expr() expr.Expr                  { return r.expression }
func (r *RedundantAdditiveAdverbs) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RedundantAdditiveAdverbs) Description() string              { return "Detects redundant additive adverbs." }
func (r *RedundantAdditiveAdverbs) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	phrase := strings.ToLower(span.GetContentString(src))
	if phrase == "also too" {
		return nil
	}
	if strings.HasSuffix(phrase, " as well as") && len(toks) >= 6 {
		toks = toks[:len(toks)-6]
	}
	var adverbs []string
	for _, tok := range toks {
		if !tok.Kind.IsWord() {
			continue
		}
		term := tok.GetStr(src)
		if term == "as" || term == "well" {
			term = "as well"
		}
		found := false
		for _, a := range adverbs {
			if a == term {
				found = true
				break
			}
		}
		if !found {
			adverbs = append(adverbs, term)
		}
	}
	if len(adverbs) < 2 {
		return nil
	}
	span, ok = doc.Span(toks)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: lint.ToReplaceSuggestions(adverbs, span.GetContent(src)), Message: "Use just one of `" + strings.Join(adverbs, "` or `") + "`.", Priority: 31}
}
