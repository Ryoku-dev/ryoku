package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type MissingPreposition struct{ expression expr.Expr }

func NewMissingPreposition() *MissingPreposition {
	return &MissingPreposition{expression: expr.NewSequenceExprWith(expr.OrLongest(expr.NewAnchorStart(), expr.NewSequenceExpr().ThenNonQuantifierDeterminer().TWs())).Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PRON, pos.PROPN})).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.AUX})).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ})).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PRON, pos.PROPN})).ThenOptional(expr.NewAnyPattern()).ThenOptional(expr.NewAnyPattern())}
}
func (r *MissingPreposition) Expr() expr.Expr                  { return r.expression }
func (r *MissingPreposition) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MissingPreposition) Description() string              { return "Locates potentially missing prepositions." }
func (r *MissingPreposition) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 || t[len(t)-1].Kind.IsUpos(pos.ADP) {
		return nil
	}
	span, ok := doc.Span(t[2:4])
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Miscellaneous, Message: "You may be missing a preposition here.", Priority: 31}
}
