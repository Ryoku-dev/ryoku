package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type AsToInterrogative struct{ expression expr.Expr }

func NewAsToInterrogative() *AsToInterrogative {
	return &AsToInterrogative{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"curious", "unsure"}).TWs().TAco("to").TWs().TSet([]string{"how", "what", "what's", "which", "when", "when's", "where", "whether", "who", "who's", "whose", "whom", "why"}), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().TWs().TSet([]string{"degree", "extent", "extend"}))}
}
func (r *AsToInterrogative) Expr() expr.Expr { return r.expression }
func (r *AsToInterrogative) Description() string {
	return "Corrects `to` to `as to` between certain adjectives and `wh-words`."
}
func (r *AsToInterrogative) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AsToInterrogative) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) < 5 {
		return nil
	}
	return &lint.Lint{Span: t[2].Span, LintKind: lint.Usage, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("as to", t[2].GetCh(s))}, Message: "This construction requires `as to` instead of just `to`.", Priority: 127}
}
