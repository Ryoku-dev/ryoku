package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type OneOfTheSingular struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewOneOfTheSingular(dictionary dict.Dictionary) *OneOfTheSingular {
	advs := expr.NewSequenceExpr().ThenZeroOrMoreSpaced(expr.NewSequenceExpr().ThenAdverb())
	nouns := expr.NewSequenceExpr().ThenZeroOrMoreSpaced(expr.NewSequenceExpr().Then(func(t doc.Token, s []rune) bool {
		return (t.Kind.IsNonPossessiveNoun() || t.Kind.IsAdjective()) && !t.Kind.IsPreposition() && !t.Kind.IsPronoun() && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"ah", "few", "first", "former", "latter", "said", "uh"})
	}))
	return &OneOfTheSingular{expression: expr.NewSequenceExprWordSeq([]string{"one", "of", "the"}).TWs().Then(expr.NewSequenceExprOptional(advs.TWs()).Then(nouns)), dictionary: dictionary}
}

func (r *OneOfTheSingular) Expr() expr.Expr                  { return r.expression }
func (r *OneOfTheSingular) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OneOfTheSingular) Description() string {
	return "Corrects 'one of the [singular]' to 'one of the [plural]'"
}
func (r *OneOfTheSingular) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	noun := toks[len(toks)-1]
	if !noun.Kind.IsSingularNoun() || noun.Kind.IsVerbProgressiveForm() {
		return nil
	}
	for _, tok := range toks {
		if tok.Kind.IsPluralNoun() && !tok.Kind.IsSingularNoun() {
			return nil
		}
	}
	if ctx == nil {
		return nil
	}
	if len(ctx.After) > 0 && (ctx.After[0].Kind.IsHyphen() || ctx.After[0].Kind.IsComma()) {
		return nil
	}
	singular := noun.GetCh(src)
	candidates := []string{string(singular) + "s", string(singular) + "es"}
	if text.EndsWithIgnoreASCIICaseStr(singular, "y") {
		candidates = append(candidates, string(singular[:len(singular)-1])+"ies")
	}
	if text.EndsWithIgnoreASCIICaseStr(singular, "fe") {
		candidates = append(candidates, string(singular[:len(singular)-2])+"ves")
	}
	var suggestions []lint.Suggestion
	for _, candidate := range candidates {
		if m := r.dictionary.GetWordMetadataStr(candidate); m != nil && m.IsPluralNoun() {
			suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(candidate, singular))
		}
	}
	return &lint.Lint{Span: noun.Span, LintKind: lint.Usage, Suggestions: suggestions, Message: "The construction `one of the ...` should use a plural noun.", Priority: 127}
}
func (r *OneOfTheSingular) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
