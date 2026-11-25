package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

// GeneralNounInsteadOfVerb corrects common noun/verb confusions.
type GeneralNounInsteadOfVerb struct{ expression expr.Expr }

func NewGeneralNounInsteadOfVerb() *GeneralNounInsteadOfVerb {
	frequency := func(tok doc.Token, src []rune) bool {
		return tok.Kind.IsFrequencyAdverb() && !text.EqStr(tok.GetCh(src), "sometimes")
	}
	pre := expr.NewFirstMatchOf([]expr.Expr{expr.AsExpr(expr.NewWordSet([]string{"he", "I", "it", "she", "they", "we", "who", "you"})), expr.AsExpr(expr.NewModalVerbWithCommonErrors()), expr.AsExpr(expr.NewWordSet([]string{"do", "don't", "dont"})), expr.AsExpr(frequency), expr.AsExpr(expr.NewWord("to"))})
	nouns := make([]string, len(nounVerbConfusionNounVerbPairs))
	for i, pair := range nounVerbConfusionNounVerbPairs {
		nouns[i] = pair[0]
	}
	basic := expr.NewSequenceExprWith(pre).TWs().Then(expr.NewWordSet(nouns))
	return &GeneralNounInsteadOfVerb{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWith(basic).ThenPunctuation(), expr.NewSequenceExprWith(basic).TWs().ThenAnyWord(), basic})}
}
func (r *GeneralNounInsteadOfVerb) Expr() expr.Expr { return r.expression }
func (r *GeneralNounInsteadOfVerb) Description() string {
	return "Corrects nouns used instead of verbs when the two are related."
}
func (r *GeneralNounInsteadOfVerb) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GeneralNounInsteadOfVerb) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) < 3 {
		return nil
	}
	prev := tokens[0]
	if len(tokens) > 4 {
		following := tokens[4]
		if following.Kind.IsNoun() && !following.Kind.IsProperNoun() && !following.Kind.IsPreposition() && !text.EqAnyIgnoreASCIICaseStr(following.GetCh(source), []string{"it", "me", "on", "that"}) {
			return nil
		}
		if text.EqStr(prev.GetCh(source), "to") && !following.Kind.IsDeterminer() {
			return nil
		}
	} else if text.EqStr(prev.GetCh(source), "to") {
		return nil
	}
	target := tokens[2]
	word := target.GetStr(source)
	verb := ""
	for _, pair := range nounVerbConfusionNounVerbPairs {
		if pair[0] == strings.ToLower(word) {
			verb = pair[1]
			break
		}
	}
	if verb == "" {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 63
	result.Message = fmt.Sprintf("`%s` is a noun, the verb should be `%s`.", word, verb)
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(verb, target.GetCh(source))}
	return &result
}
