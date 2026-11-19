package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type JealousOf struct{ expression expr.Expr }

func NewJealousOf() *JealousOf {
	return &JealousOf{expression: expr.NewSequenceExprAco("jealous").TWs().TAco("from").TWs().ThenOptional(expr.NewSequenceExpr().ThenDeterminer().TWs()).Then(func(t doc.Token, _ []rune) bool {
		k := t.Kind
		return (k.IsNominal() || !k.IsVerb()) && (k.IsOov() || k.IsNominal()) && !k.IsPreposition()
	})}
}
func (r *JealousOf) Expr() expr.Expr { return r.expression }
func (r *JealousOf) Description() string {
	return "Encourages the standard preposition after `jealous`."
}
func (r *JealousOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *JealousOf) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span := toks[2].Span
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("of", src[span.Start:span.End])}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = "Use `of` after `jealous`."
	result.Suggestions = suggestions
	return &result
}
