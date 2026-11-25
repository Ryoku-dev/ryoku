package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type HowTo struct{ expression expr.Expr }

func NewHowTo() *HowTo {
	pattern := expr.NewAll()
	pattern.Add(expr.NewSequenceExprAnything().ThenAnything().TAco("how").ThenWhitespace().ThenVerbLemma())
	finiteClauseVerb := expr.NewSequenceExprWhitespace().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsAuxiliaryVerb, doc.TokenKind.IsVerbThirdPersonSingularPresentForm, doc.TokenKind.IsVerbSimplePastForm})
	nounLedClause := expr.NewSequenceExpr().ThenKindEither(doc.TokenKind.IsNominal, doc.TokenKind.IsProperNoun).ThenAnyOf([]expr.Expr{
		finiteClauseVerb,
		expr.NewSequenceExprWhitespace().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsNoun, doc.TokenKind.IsProperNoun, doc.TokenKind.IsVerbProgressiveForm}).ThenSeq(expr.NewSequenceExprWhitespace().Then(expr.Or(expr.NewInflectionOfBe(), expr.NewSequenceExpr().ThenAuxiliaryVerb()))),
		expr.NewSequenceExprWhitespace().TAco("to").ThenWhitespace().Then(expr.NewNominalPhrase()).ThenSeq(expr.NewSequenceExprWhitespace().ThenKindAny([]expr.KindPredicate{doc.TokenKind.IsAuxiliaryVerb, doc.TokenKind.IsVerbThirdPersonSingularPresentForm, doc.TokenKind.IsVerbSimplePastForm})),
	})
	exceptions := expr.NewSequenceExprUnless(expr.NewUPOSSet([]pos.UPOS{pos.PART})).ThenAnything().ThenUnless(func(t doc.Token, _ []rune) bool { return t.Kind.IsNpMember() }).ThenAnything().ThenUnless(expr.Or(expr.Or(expr.NewInflectionOfBe(), expr.NewSequenceExpr().ThenKindAnyOrWords([]expr.KindPredicate{doc.TokenKind.IsAuxiliaryVerb, doc.TokenKind.IsAdjective, doc.TokenKind.IsConjunction, doc.TokenKind.IsProperNoun}, []string{"did", "come", "does"})), nounLedClause))
	pattern.Add(exceptions)
	return &HowTo{expression: pattern}
}
func (r *HowTo) Expr() expr.Expr                  { return r.expression }
func (r *HowTo) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*HowTo) Description() string {
	return "Detects the omission of `to` in constructions like `how clone / how install` and suggests `how to …`."
}
func (*HowTo) MatchToLint(toks []doc.Token, _ []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	return &lint.Lint{Span: toks[2].Span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune(" to"))}, Message: "Insert `to` after `how` (e.g., `how to clone`).", Priority: 63}
}
