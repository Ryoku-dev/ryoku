package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type Cant struct{ expression expr.Expr }

func NewCant() *Cant {
	e := expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExpr().ThenKindExcept(doc.TokenKind.IsNominal, []string{"or"}).TWs().TAco("cant"), expr.NewSequenceExprAco("cant").TWs().ThenPersonalPronoun(), expr.NewSequenceExprAco("cant").TWs().ThenKindIsButIsNot(doc.TokenKind.IsVerbLemma, doc.TokenKind.IsNoun)})
	return &Cant{expression: e}
}
func (r *Cant) Expr() expr.Expr                  { return r.expression }
func (r *Cant) Description() string              { return "Suggests correcting `cant` to `can't`." }
func (r *Cant) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Cant) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), "cant") })
	if tok == nil {
		return nil
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Enhancement
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("can't", tok.GetCh(s))}
	l.Message = "`Cant` is secret language or jargon. If that's not what you mean you should use `can't` here."
	return &l
}
