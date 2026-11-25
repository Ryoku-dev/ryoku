package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
)

type VerbInsteadOfNoun struct{ expression expr.Expr }

func NewVerbInsteadOfNoun() *VerbInsteadOfNoun {
	verbs := make([]string, len(nounVerbConfusionNounVerbPairs))
	for i, pair := range nounVerbConfusionNounVerbPairs {
		verbs[i] = pair[1]
	}
	return &VerbInsteadOfNoun{expression: expr.NewSequenceExprWith(expr.NewUPOSSet([]pos.UPOS{pos.ADJ, pos.ADP})).TWs().Then(expr.NewWordSet(verbs))}
}
func (r *VerbInsteadOfNoun) Expr() expr.Expr { return r.expression }
func (r *VerbInsteadOfNoun) Description() string {
	return "Corrects verbs used instead of nouns when the two are related."
}
func (r *VerbInsteadOfNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *VerbInsteadOfNoun) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) == 0 {
		return nil
	}
	adj := tokens[0]
	target := tokens[len(tokens)-1]
	adjText := adj.GetStr(source)
	word := target.GetStr(source)
	if adj.Kind.IsAuxiliaryVerb() || adj.Kind.IsUpos(pos.AUX) {
		return nil
	}
	noun := ""
	for _, pair := range nounVerbConfusionNounVerbPairs {
		if pair[1] == strings.ToLower(word) {
			noun = pair[0]
			break
		}
	}
	if noun == "" || adjText == "better" || adjText == "best" || (adjText == "sound" && word == "affect") {
		return nil
	}
	result := lint.NewLint()
	result.Span = target.Span
	result.LintKind = lint.WordChoice
	result.Priority = 63
	result.Message = fmt.Sprintf("`%s` is a verb, the noun should be `%s`.", word, noun)
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(noun, target.GetCh(source))}
	return &result
}
