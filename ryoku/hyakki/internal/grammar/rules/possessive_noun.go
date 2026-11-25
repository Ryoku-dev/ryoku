package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type PossessiveNoun struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewPossessiveNoun(dictionary dict.Dictionary) *PossessiveNoun {
	return &PossessiveNoun{expression: expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(expr.NewUPOSSet([]pos.UPOS{pos.DET, pos.PROPN})).TWs().ThenKindIsButIsNot(doc.TokenKind.IsPluralNominal, doc.TokenKind.IsSingularNominal).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.NOUN, pos.PROPN})).ThenOptional(expr.NewSequenceExprAnything().TAny()), expr.NewSequenceExprAnything().TAny().TAny().TAny().ThenNoun(), expr.NewSequenceExprUnless(func(t doc.Token, _ []rune) bool { return t.Kind.IsDemonstrativeDeterminer() }).TAny().ThenUnless(expr.NewWordSet([]string{"flags", "checks", "catches", "you"})).TAny().ThenUnless(expr.NewWordSet([]string{"form", "go"}))}), dictionary: dictionary}
}
func (r *PossessiveNoun) Expr() expr.Expr { return r.expression }
func (r *PossessiveNoun) Description() string {
	return "Use an apostrophe and `s` to form a noun’s possessive."
}
func (r *PossessiveNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PossessiveNoun) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 || toks[len(toks)-1].Kind.IsUpos(pos.ADV) {
		return nil
	}
	span := toks[2].Span
	plural := src[span.Start:span.End]
	singular := plural
	if len(plural) > 0 && plural[len(plural)-1] == 's' {
		singular = plural[:len(plural)-1]
	}
	replacement := string(singular) + "'s"
	if !r.dictionary.ContainsWordStr(replacement) {
		return nil
	}
	suggestions := []lint.Suggestion{lint.ReplaceWith([]rune(replacement))}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Miscellaneous
	result.Priority = 10
	result.Message = "Use an apostrophe and `s` to form a noun’s possessive."
	result.Suggestions = suggestions
	return &result
}
