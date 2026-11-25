package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type MeansALotTo struct{ expression expr.Expr }

func NewMeansALotTo() *MeansALotTo {
	return &MeansALotTo{expression: expr.NewSequenceExprWordSet([]string{"mean", "means", "meant"}).TWs().ThenAnyOf([]expr.Expr{expr.NewFixedPhraseFromPhrase("a lot"), expr.AsExpr(expr.NewWordSet([]string{"alot", "lot"}))}).TWs().TAco("for")}
}
func (r *MeansALotTo) Expr() expr.Expr { return r.expression }
func (r *MeansALotTo) Description() string {
	return "Corrects wrong variants of `means a lot for [someone]` to `means a lot to [someone]`."
}
func (r *MeansALotTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MeansALotTo) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) != 5 && len(t) != 7 {
		return nil
	}
	span := t[len(t)-1].Span
	sug := "to"
	msg := "The correct preposition in this phrase is `to`."
	if len(t) == 5 {
		span.Start = t[2].Span.Start
		sug = "a lot to"
		msg = "In this phrase the correct spelling is `a lot` and the correct preposition is `to`."
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(sug, span.GetContent(s))}
	result.Message = msg
	return &result
}
