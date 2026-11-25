package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type RedundantProgressiveComparative struct{ expression expr.Expr }

func NewRedundantProgressiveComparative() *RedundantProgressiveComparative {
	return &RedundantProgressiveComparative{expression: expr.NewSequenceExprAco("increasingly").TWs().ThenWordSet([]string{"more", "less"}).TWs().ThenKindEither(doc.TokenKind.IsAdjective, doc.TokenKind.IsAdverb)}
}
func (r *RedundantProgressiveComparative) Expr() expr.Expr { return r.expression }
func (r *RedundantProgressiveComparative) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (*RedundantProgressiveComparative) Description() string {
	return "Detects redundant comparatives like `increasingly more` and `increasingly less`."
}
func (r *RedundantProgressiveComparative) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	word := ""
	if text.EqStr(toks[2].GetCh(src), "more") {
		word = "more"
	} else if text.EqStr(toks[2].GetCh(src), "less") {
		word = "less"
	} else {
		return nil
	}
	span := text.Span{Start: toks[0].Span.Start, End: toks[2].Span.End}
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(word+" and "+word, span.GetContent(src))}, Message: "This phrasing is redundant; use a direct comparative like `" + word + " and " + word + "`.", Priority: 31}
}
