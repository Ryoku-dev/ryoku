package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type AccuseOf struct{ expression expr.Expr }

func NewAccuseOf() *AccuseOf {
	return &AccuseOf{expression: expr.ButNot(expr.NewSequenceExprOptional(expr.NewSequenceExprAnyWord().TWs()).TSet([]string{"accuse", "accused", "accuses", "accusing"}).ThenOptional(expr.NewSequenceExprWhitespace().ThenObjectPronoun()).TWs().TSet([]string{"for", "as"}), expr.NewSequenceExprWordSeq([]string{"the", "accused"}))}
}
func (r *AccuseOf) Expr() expr.Expr                  { return r.expression }
func (r *AccuseOf) Description() string              { return "Corrects `accuse for/as` to `accuse of`." }
func (r *AccuseOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AccuseOf) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *AccuseOf) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if c != nil && len(c.After) >= 6 {
		a := c.After
		if a[0].Kind.IsWhitespace() && a[1].Kind.IsWord() && text.EqStr(a[1].GetCh(s), "the") && a[2].Kind.IsWhitespace() && (a[3].Kind.IsOrdinalNumber() || a[3].Kind.IsWord() && text.EqAnyIgnoreASCIICaseStr(a[3].GetCh(s), []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth"})) && a[4].Kind.IsWhitespace() && a[5].Kind.IsWord() && text.EqStr(a[5].GetCh(s), "time") {
			return nil
		}
	}
	if len(t) == 0 {
		return nil
	}
	span := t[len(t)-1].Span
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of", span.GetContent(s))}, Message: "The correct preposition is `of`, not `for` or `as`.", Priority: 127}
}
