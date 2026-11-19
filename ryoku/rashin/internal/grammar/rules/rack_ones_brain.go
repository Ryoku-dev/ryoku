package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"unicode"
)

type RackOnesBrain struct{ expression expr.Expr }

func NewRackOnesBrain() *RackOnesBrain {
	return &RackOnesBrain{expression: expr.NewSequenceExprWordSet([]string{"wrack", "wracked", "wracking", "wracks", "wreck", "wrecked", "wrecking", "wrecks"}).TWs().ThenPossessiveDeterminer().TWs().ThenOptional(expr.NewSequenceExpr().ThenAdjective().TWs()).TSet([]string{"brain", "brains"})}
}
func (r *RackOnesBrain) Expr() expr.Expr                  { return r.expression }
func (r *RackOnesBrain) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*RackOnesBrain) Description() string {
	return "Checks for improper uses of `wrack` or `wreck` instead of `rack` in the idiom `rack one's brain`."
}
func (r *RackOnesBrain) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := t[0]
	original := tok.GetCh(s)
	upperW, upperR := unicode.IsUpper(original[0]), unicode.IsUpper(original[1])
	correction := append([]rune(nil), original[1:]...)
	correction[0] = 'r'
	if upperW {
		correction[0] = 'R'
	}
	correction[1] = 'a'
	if upperR {
		correction[1] = 'A'
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(correction)}
	l.Message = "Did you mean to use the word `rack`? While historically debated, standard modern usage dictates `rack one's brain` when describing intense mental effort."
	return &l
}
