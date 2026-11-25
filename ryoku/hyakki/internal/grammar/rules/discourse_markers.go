package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"strings"
)

type DiscourseMarkers struct{ expression expr.Expr }

func NewDiscourseMarkers() *DiscourseMarkers {
	phrases := []string{"accordingly", "actually", "additionally", "also", "alternatively", "besides", "by the way", "consequently", "conversely", "even so", "for example", "frankly", "furthermore", "hence", "honestly", "however", "in addition", "in other words", "incidentally", "indeed", "instead", "meanwhile", "moreover", "nevertheless", "notwithstanding", "nowadays", "now a days", "now-a-days", "on the other hand", "otherwise", "subsequently", "that said", "therefore", "thus"}
	patterns := expr.NewFirstMatchOf()
	for _, phrase := range phrases {
		patterns.Add(expr.NewFixedPhraseFromPhrase(phrase))
	}
	return &DiscourseMarkers{expression: expr.NewSequenceExprWith(patterns).TWs().ThenUnless(expr.NewUPOSSet([]pos.UPOS{pos.ADJ, pos.ADV, pos.ADP, pos.CCONJ, pos.VERB}))}
}
func (*DiscourseMarkers) Description() string {
	return "Flags sentences that begin with a discourse marker but omit the required following comma."
}
func (r *DiscourseMarkers) Lint(d *doc.Document) []lint.Lint {
	var out []lint.Lint
	for _, sent := range doc.IterSentences(d.Tokens()) {
		if l := r.lintSentence(sent, d.GetSource()); l != nil {
			out = append(out, *l)
		}
	}
	return out
}
func (r *DiscourseMarkers) lintSentence(sent []doc.Token, source []rune) *lint.Lint {
	indices := doc.IterWordIndices(sent)
	if len(indices) == 0 {
		return nil
	}
	matched, ok := r.expression.Run(indices[0], sent, source)
	if !ok {
		return nil
	}
	end := matched.End - 2
	marker := sent[matched.Start:end]
	if r.isPrepositional(marker, sent[end:], source) {
		return nil
	}
	span, ok := doc.Span(marker)
	if !ok {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.Punctuation, Suggestions: []lint.Suggestion{lint.InsertAfter([]rune{','})}, Message: "Discourse markers at the beginning of a sentence should be followed by a comma.", Priority: 31}
}
func (r *DiscourseMarkers) isPrepositional(marker, rest []doc.Token, source []rune) bool {
	span, ok := doc.Span(marker)
	next, found := doc.FirstNonWhitespace(rest)
	if !ok || !found {
		return false
	}
	word := strings.ToLower(span.GetContentString(source))
	if word != "besides" && word != "notwithstanding" {
		return false
	}
	if next.Kind.IsUpos(pos.DET) {
		return true
	}
	for _, tok := range rest {
		if tok.Kind.IsComma() {
			break
		}
		if tok.Kind.IsUpos(pos.VERB) {
			return false
		}
	}
	return true
}
