package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type VerbToAdjective struct{ expression expr.Expr }

func NewVerbToAdjective() *VerbToAdjective {
	return &VerbToAdjective{expression: expr.NewSequenceExprWordSet([]string{"the", "a", "an"}).TWs().ThenKindWhere(doc.TokenKind.IsDegreeAdverb).TWs().ThenKindWhere(func(k doc.TokenKind) bool { return k.IsNoun() && k.IsVerbProgressiveForm() }).TWs().Then(expr.NewWordSet([]string{"of"}))}
}
func (r *VerbToAdjective) Expr() expr.Expr                  { return r.expression }
func (r *VerbToAdjective) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *VerbToAdjective) Description() string {
	return "Looks for article-led gerund noun phrases like `a fully accounting of`, where an adjective is more likely than an adverb."
}
func (r *VerbToAdjective) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	words := doc.IterWords(toks)
	if len(words) != 4 {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Typo
	l.Priority = 31
	l.Suggestions = []lint.Suggestion{}
	l.Message = fmt.Sprintf("`%s` is an adverb. Before the noun `%s`, this phrase more likely needs an adjective.", words[1].GetStr(src), words[2].GetStr(src))
	return &l
}
