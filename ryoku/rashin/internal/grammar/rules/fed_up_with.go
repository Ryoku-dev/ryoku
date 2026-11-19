package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type FedUpWith struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewFedUpWith(dialect dict.Dialect) *FedUpWith {
	return &FedUpWith{expr.NewSequenceExprWordSeq([]string{"fed", "up", "of"}), dialect}
}
func (r *FedUpWith) Expr() expr.Expr                  { return r.expression }
func (r *FedUpWith) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FedUpWith) Description() string {
	return "Corrects `fed up of` to `fed up with` in dialects other than British English."
}
func (r *FedUpWith) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if r.dialect == dict.British || len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Usage
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("with", tok.GetCh(src))}
	result.Message = "`Fed up of` is not accepted outside of British English."
	return &result
}
