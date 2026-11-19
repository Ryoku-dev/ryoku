package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type CompoundSubjectI struct{ expression expr.Expr }

func NewCompoundSubjectI() *CompoundSubjectI {
	return &CompoundSubjectI{expression: expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenOptional(expr.NewSequenceExpr().ThenQuote().ThenOptional(expr.NewSequenceExpr().TWs())).ThenOptional(expr.NewSequenceExpr().ThenPunctuation().ThenOptional(expr.NewSequenceExpr().TWs())).ThenWordSet([]string{"my", "your", "her", "his", "their", "our"}).TWs().ThenNominal().TWs().TAco("and").TWs().TAco("me").TWs().ThenKindEither(doc.TokenKind.IsVerb, doc.TokenKind.IsAuxiliaryVerb)}
}
func (r *CompoundSubjectI) Expr() expr.Expr { return r.expression }
func (r *CompoundSubjectI) Description() string {
	return "Promotes `I` in compound subjects headed by a possessive determiner."
}
func (r *CompoundSubjectI) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *CompoundSubjectI) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	for _, tok := range t {
		if tok.Kind.IsWord() && text.EqStr(tok.GetCh(s), "me") {
			result := lint.NewLint()
			result.Span = tok.Span
			result.LintKind = lint.Grammar
			result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune("I"))}
			result.Message = "Use `I` when this pronoun is part of a compound subject."
			result.Priority = 31
			return &result
		}
	}
	return nil
}
