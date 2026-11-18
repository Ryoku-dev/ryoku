package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type OxfordComma struct{ expression expr.Expr }

func NewOxfordComma() *OxfordComma {
	item := expr.OrLongest(expr.NewSequenceExpr().ThenDeterminer().TWs().ThenNominal(), expr.NewSequenceExpr().ThenNominal())
	itemChunk := expr.NewSequenceExprWith(item).ThenComma().TWs()
	return &OxfordComma{expression: expr.NewSequenceExpr().ThenOneOrMore(itemChunk).Then(item).TWs().TSet([]string{"and", "or", "nor"}).TWs().Then(item)}
}
func (r *OxfordComma) Description() string {
	return "The Oxford comma is one of the more controversial rules in common use today. Enabling this lint checks that there is a comma before `and`, `or`, or `nor` when listing out more than two ideas."
}
func (r *OxfordComma) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, sentence := range d.IterSentences() {
		var words [2]*dict.DictWordMetadata
		wordCount := 0
		for _, tok := range sentence {
			if tok.Kind.IsWord() && tok.Kind.Metadata != nil {
				words[wordCount] = tok.Kind.Metadata
				wordCount++
				if wordCount == len(words) {
					break
				}
			}
		}
		skip := 0
		if wordCount == 2 && words[0].Preposition && words[1].IsLikelyHomograph() {
			skip = len(sentence)
			for i, tok := range sentence {
				if tok.Kind.IsComma() {
					skip = i
					break
				}
			}
		}
		sentence = sentence[skip:]
		for _, match := range expr.IterMatches(r.expression, sentence, d.GetSource()) {
			toks := sentence[match.Start:match.End]
			i, ok := doc.LastConjunctionIndex(toks)
			if !ok || i < 2 {
				continue
			}
			out = append(out, lint.Lint{Span: toks[i-2].Span, LintKind: lint.Style, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune{','})}, Message: "An Oxford comma is necessary here.", Priority: 31})
		}
	}
	return out
}
