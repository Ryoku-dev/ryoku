package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OutOfTheWindow struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewOutOfTheWindow(dialect dict.Dialect) *OutOfTheWindow {
	return &OutOfTheWindow{expr.NewSequenceExprAco("out").ThenOptional(expr.NewSequenceExprWhitespace().TAco("of")).TWs().TAco("the").TWs().TAco("window"), dialect}
}
func (r *OutOfTheWindow) Expr() expr.Expr                  { return r.expression }
func (r *OutOfTheWindow) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OutOfTheWindow) Description() string              { return "A linter for the idiom `out (of) the window`." }
func (r *OutOfTheWindow) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	result := lint.NewLint()
	result.LintKind = lint.Regionalism
	if len(toks) == 7 && (r.dialect == dict.American || r.dialect == dict.Australian || r.dialect == dict.Canadian) {
		span, ok := doc.Span(toks[1:3])
		if !ok {
			return nil
		}
		result.Span = span
		result.Suggestions = []lint.Suggestion{lint.Remove()}
		result.Message = "If this is the idiom about abandoning a plan, it's more usual to leave out `of` in " + r.dialect.String() + " English"
	} else if len(toks) == 5 && r.dialect == dict.British {
		result.Span = toks[0].Span
		result.Suggestions = []lint.Suggestion{lint.InsertAfter([]rune(" of"))}
		result.Message = "If this is the idiom about abandoning a plan, consider using `of` in British English"
	} else {
		return nil
	}
	return &result
}
