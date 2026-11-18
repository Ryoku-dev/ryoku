package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FewUnitsOfTimeAgo struct{ expression expr.Expr }

func NewFewUnitsOfTimeAgo() *FewUnitsOfTimeAgo {
	start := expr.NewSequenceExpr().ThenWordExcept([]string{"a"}).TWs()
	return &FewUnitsOfTimeAgo{expression: expr.NewSequenceExprWith(start).TAco("few").ThenWhitespace().Then(expr.NewTimeUnitExpr()).ThenWhitespace().TAco("ago")}
}
func (r *FewUnitsOfTimeAgo) Expr() expr.Expr                  { return r.expression }
func (r *FewUnitsOfTimeAgo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FewUnitsOfTimeAgo) Description() string {
	return "Corrects some expressions using `few` where `a few` is correct."
}
func (r *FewUnitsOfTimeAgo) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	for i, tok := range toks {
		if i >= 3 {
			break
		}
		if text.EqStr(tok.GetCh(src), "few") {
			l := lint.NewLint()
			l.Span = tok.Span
			l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("a few", tok.GetCh(src))}
			l.Message = "In this construction you need to use `a few` instead of just `few`."
			return &l
		}
	}
	return nil
}
