package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type TillDate struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewTillDate(dialect dict.Dialect) *TillDate {
	return &TillDate{expression: expr.NewSequenceExprWordSet([]string{"till", "til", "'til", "til'"}).TWs().TAco("date"), dialect: dialect}
}
func (r *TillDate) Expr() expr.Expr                  { return r.expression }
func (r *TillDate) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *TillDate) Description() string {
	return "Corrects the Indian English `till date` to `to date` when Indian English is not the selected dialect."
}
func (r *TillDate) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if r.dialect == dict.Indian || len(toks) == 0 {
		return nil
	}
	if ctx != nil && len(ctx.Before) >= 2 {
		before := ctx.Before
		prev := before[len(before)-1]
		prevprev := before[len(before)-2]
		if prev.Kind.IsWhitespace() && text.EqAnyIgnoreASCIICaseStr(prevprev.GetCh(src), []string{"good", "valid"}) {
			return nil
		}
		if prev.Kind.IsApostrophe() && prevprev.Kind.IsWhitespace() && len(before) >= 3 && text.EqAnyIgnoreASCIICaseStr(before[len(before)-3].GetCh(src), []string{"good", "valid"}) {
			return nil
		}
	}
	return &lint.Lint{Span: toks[0].Span, LintKind: lint.Regionalism, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("to", toks[0].GetCh(src))}, Message: "Outside Indian English, prefer `to date`.", Priority: 127}
}
func (r *TillDate) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
