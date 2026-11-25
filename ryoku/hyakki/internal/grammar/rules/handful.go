package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type Handful struct{ expression expr.Expr }

func NewHandful() *Handful {
	e := expr.NewSequenceExprAco("hand").ThenOneOrMore(expr.NewSpaceOrHyphen()).TAco("full").ThenOneOrMore(expr.NewSpaceOrHyphen()).TAco("of")
	return &Handful{expression: e}
}
func (r *Handful) Expr() expr.Expr { return r.expression }
func (r *Handful) Description() string {
	return "Keeps the palm-sized quantity expressed by `handful` as one word."
}
func (r *Handful) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Handful) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 2 {
		return nil
	}
	end := len(t) - 1
	for end > 0 && (t[end-1].Kind.IsWhitespace() || t[end-1].Kind.IsHyphen()) {
		end--
	}
	if end == 0 {
		return nil
	}
	span, _ := doc.Span(t[:end])
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.BoundaryError
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("handful", t[0].GetCh(s))}
	l.Message = "Write this quantity as the single word `handful`."
	l.Priority = 31
	return &l
}
