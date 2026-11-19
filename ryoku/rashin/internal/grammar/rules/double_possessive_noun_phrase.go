package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DoublePossessiveNounPhrase struct{ expression expr.Expr }

func NewDoublePossessiveNounPhrase() *DoublePossessiveNounPhrase {
	return &DoublePossessiveNounPhrase{expression: expr.NewSequenceExpr().ThenPossessiveDeterminer().TWs().ThenAdjective().TWs().TSet([]string{"column", "draft", "edition", "half", "line", "part", "point", "portion", "row", "section", "segment", "verse", "year"}).TWs().TAco("of").TWs().ThenPossessiveDeterminer()}
}
func (r *DoublePossessiveNounPhrase) Expr() expr.Expr { return r.expression }
func (r *DoublePossessiveNounPhrase) Lint(d *doc.Document) []lint.Lint {
	return lint.RunExprLinter(r, d)
}
func (r *DoublePossessiveNounPhrase) Description() string {
	return "Detects redundant possessive constructions like `my latter half of my book`."
}
func (r *DoublePossessiveNounPhrase) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 || !text.EqChLenient(toks[0].GetCh(src), toks[len(toks)-1].GetCh(src)) {
		return nil
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Redundancy, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("the", toks[0].GetCh(src))}, Message: "Avoid the redundant possessive by changing the first to `the`.", Priority: 127}
}
