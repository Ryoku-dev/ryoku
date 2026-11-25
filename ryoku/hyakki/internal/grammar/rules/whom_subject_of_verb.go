package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type WhomSubjectOfVerb struct{ expression expr.Expr }

func NewWhomSubjectOfVerb() *WhomSubjectOfVerb {
	return &WhomSubjectOfVerb{expression: expr.NewSequenceExprWordSet([]string{"whom", "whomever", "whomsoever"}).TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool {
		return k.IsVerbThirdPersonSingularPresentForm() || k.IsVerbSimplePastForm()
	}), expr.AsExpr(expr.NewModalVerbWithCommonErrors())})}
}
func (r *WhomSubjectOfVerb) Expr() expr.Expr                  { return r.expression }
func (r *WhomSubjectOfVerb) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WhomSubjectOfVerb) Description() string {
	return "Detects whom and its variants used as the subject of a verb instead of who."
}
func (r *WhomSubjectOfVerb) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *WhomSubjectOfVerb) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if ctx != nil && len(ctx.Before) >= 4 {
		b := ctx.Before[len(ctx.Before)-4:]
		if b[3].Kind.IsWhitespace() && text.EqStr(b[2].GetCh(src), "of") && b[1].Kind.IsWhitespace() && text.EqAnyIgnoreASCIICaseStr(b[0].GetCh(src), []string{"each", "many"}) {
			return nil
		}
	}
	if len(toks) == 0 {
		return nil
	}
	tok := toks[0]
	ch := tok.GetCh(src)
	if len(ch) < 4 {
		return nil
	}
	replacement := append(append([]rune(nil), ch[:3]...), ch[4:]...)
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Grammar
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCase(replacement, ch)}
	l.Message = "“Whom” is used for the object of a verb and “who” is used for the subject of a verb."
	return &l
}
