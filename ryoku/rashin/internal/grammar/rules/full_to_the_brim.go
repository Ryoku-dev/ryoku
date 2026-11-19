package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type FullToTheBrim struct{ expression expr.Expr }

func NewFullToTheBrim() *FullToTheBrim {
	return &FullToTheBrim{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"full", "filled"}).TWs().ThenWordSeq([]string{"to", "the", "brim", "of"}), expr.NewSequenceExpr().TAny().TAny().TAny().TAny().TAny().TAny().TAny().TAny().TAny().TWs().TAco("the"))}
}
func (r *FullToTheBrim) Expr() expr.Expr { return r.expression }
func (r *FullToTheBrim) Description() string {
	return "Corrects the wrong preposition in the idiom `full` or `filled to the brim`."
}
func (r *FullToTheBrim) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FullToTheBrim) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, _ []rune) bool { return text.EqStr(t.GetCh(s), "of") })
	if tok == nil {
		return nil
	}
	return &lint.Lint{Span: tok.Span, LintKind: lint.Grammar, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("with", tok.GetCh(s))}, Message: "The correct preposition with this idiom is 'with', not 'of'", Priority: 127}
}
