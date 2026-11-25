package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type LookDownOnesNose struct{ expression expr.Expr }

func NewLookDownOnesNose() *LookDownOnesNose {
	return &LookDownOnesNose{expression: expr.NewSequenceExprWordSet([]string{"look", "looked", "looking", "looks"}).TWs().ThenPossessiveDeterminer().TWs().ThenWordSet([]string{"nose", "noses"}).TWs().TAco("down")}
}
func (r *LookDownOnesNose) Expr() expr.Expr                  { return r.expression }
func (r *LookDownOnesNose) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*LookDownOnesNose) Description() string {
	return "Corrects `look one's nose down` to `look down one's nose`"
}
func (r *LookDownOnesNose) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(t[0].GetStr(s)+" down "+t[2].GetStr(s)+" "+t[4].GetStr(s), s[span.Start:span.End])}
	l.Message = "The correct idiom is `look down one's nose`."
	return &l
}
