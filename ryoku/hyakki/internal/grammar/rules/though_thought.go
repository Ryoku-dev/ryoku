package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ThoughThought struct{ expression expr.Expr }

func NewThoughThought() *ThoughThought {
	return &ThoughThought{expression: expr.NewSequenceExpr().ThenKindIsButIsNot(doc.TokenKind.IsSubjectPronoun, doc.TokenKind.IsObjectPronoun).TWs().TAco("though").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExpr().ThenSubjectPronoun(), expr.NewSequenceExprAco("that")})}
}
func (r *ThoughThought) Expr() expr.Expr                  { return r.expression }
func (r *ThoughThought) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ThoughThought) Description() string {
	return "Corrects `though` when it's a typo for `thought`."
}
func (r *ThoughThought) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(toks, src, func(tok *doc.Token, src []rune) bool { return text.EqStr(tok.GetCh(src), "though") })
	if tok == nil {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	span := tok.Span
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Typo
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("thought", span.GetContent(src))}
	result.Message = "Is this a typo for `thought`?"
	result.Priority = 127
	return &result
}
