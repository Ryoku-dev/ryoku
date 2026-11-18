package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
)

type PronounInflectionBe struct {
	expression expr.Expr
	mapping    *expr.ExprMap[string]
}

func NewPronounInflectionBe() *PronounInflectionBe {
	modTerm := expr.NewSequenceExprWhitespace().Then(expr.NewUPOSSet([]pos.UPOS{pos.ADJ, pos.ADV}))
	mapping := expr.NewExprMap[string]()
	mapping.Insert(expr.NewSequenceExpr().ThenThirdPersonSingularPronoun().ThenOptional(modTerm).TWs().TAco("are").TAny().ThenUnless(expr.NewNominalPhrase()), "is")
	mapping.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenThirdPersonSingularPronoun().ThenOptional(modTerm).TWs().TAco("are").TAny().TAny(), "is")
	mapping.Insert(expr.NewSequenceExpr().ThenThirdPersonSingularPronoun().ThenOptional(modTerm).TWs().TAco("aren't").TAny().TAny(), "isn't")
	mapping.Insert(expr.NewSequenceExpr().ThenKindWhere(func(k doc.TokenKind) bool {
		md, ok := k.AsWord()
		return ok && md != nil && md.NpMember != nil && *md.NpMember
	}).ThenWhitespace().ThenThirdPersonPluralPronoun().ThenOptional(modTerm).TWs().TAco("is").TAny().TAny(), "are")
	mapping.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenThirdPersonPluralPronoun().ThenOptional(modTerm).TWs().TAco("is").TAny().TAny(), "are")
	mapping.Insert(expr.NewSequenceExpr().ThenThirdPersonPluralPronoun().ThenOptional(modTerm).TWs().TAco("isn't").TAny().TAny(), "aren't")
	mapping.Insert(expr.NewSequenceExpr().ThenFirstPersonPluralPronoun().ThenOptional(modTerm).TWs().TAco("was").TAny().TAny(), "were")
	mapping.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenKindEither(doc.TokenKind.IsThirdPersonPluralPronoun, doc.TokenKind.IsSecondPersonPronoun).ThenOptional(modTerm).TWs().TAco("was").TAny().TAny(), "were")
	mapping.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenKindEither(doc.TokenKind.IsFirstPersonSingularPronoun, doc.TokenKind.IsThirdPersonSingularPronoun).ThenOptional(modTerm).TWs().TAco("were").TAny().TAny(), "was")
	return &PronounInflectionBe{expr.NewAll([]expr.Expr{mapping, expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsUpos(pos.PRON) })}), mapping}
}
func (r *PronounInflectionBe) Expr() expr.Expr                  { return r.expression }
func (r *PronounInflectionBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *PronounInflectionBe) Description() string {
	return "Checks subject–verb agreement for the verb `be`. Third-person singular pronouns (`he`, `she`, `it`) require the singular form `is`, while the plural pronoun `they` takes `are`. The linter flags mismatches such as `He are` or `They is` and offers the correct concord."
}
func (r *PronounInflectionBe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 3 {
		return nil
	}
	span := toks[len(toks)-3].Span
	correct, ok := r.mapping.Lookup(0, toks, src)
	if !ok {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Agreement
	l.Priority = 30
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(correct, span.GetContent(src))}
	l.Message = "Make the verb agree with its subject."
	return &l
}
