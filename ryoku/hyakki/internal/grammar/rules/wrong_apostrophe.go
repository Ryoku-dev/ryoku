package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type WrongApostrophe struct{ expression expr.Expr }

func NewWrongApostrophe() *WrongApostrophe {
	return &WrongApostrophe{expression: expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprAnyWord().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSemicolon(), expr.NewSequenceExpr().ThenAcute(), expr.NewSequenceExpr().ThenBacktick()}).ThenWordSet([]string{"d", "ll", "m", "re", "s", "t", "ve"})})}
}
func (r *WrongApostrophe) Expr() expr.Expr                  { return r.expression }
func (r *WrongApostrophe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WrongApostrophe) Description() string {
	return "Corrects semicolons, acute accents, and backticks typed instead of apostrophes."
}
func (r *WrongApostrophe) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	base, ending := t[0].GetStr(s), t[len(t)-1].GetStr(s)
	replacement := strings.ToLower(base) + "'" + strings.ToLower(ending)
	return &lint.Lint{Span: span, LintKind: lint.Typo, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, []rune(base+ending))}, Message: "Did you mean `" + replacement + "`?", Priority: 57}
}
func (r *WrongApostrophe) Unit() lint.DocumentIterator { return lint.Sentence{} }
