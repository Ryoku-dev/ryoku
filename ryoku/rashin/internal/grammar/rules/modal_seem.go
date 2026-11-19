package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ModalSeem struct{ expression expr.Expr }

func NewModalSeem() *ModalSeem {
	return &ModalSeem{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWith(expr.NewModalVerb()).TWs().TAco("seen").TWs().ThenAdjective(), expr.NewSequenceExprWith(expr.NewModalVerb()).TWs().TAco("seen").TWs().ThenAdverb().TWs().ThenAdjective()})}
}
func (r *ModalSeem) Expr() expr.Expr                  { return r.expression }
func (r *ModalSeem) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*ModalSeem) Description() string {
	return "Detects modal verbs followed by `seen` before adjectives and suggests `seem` or `be`."
}
func (r *ModalSeem) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if _, ok := r.expression.Run(0, t, s); !ok {
		return nil
	}
	for _, tok := range t {
		if !text.EqStr(tok.GetCh(s), "seen") {
			continue
		}
		return &lint.Lint{Span: tok.Span, LintKind: lint.Grammar, Suggestions: lint.ToReplaceSuggestions([]string{"seem", "be"}, tok.GetCh(s)), Message: "Swap `seen` for a linking verb when it follows a modal before an adjective.", Priority: 32}
	}
	return nil
}
